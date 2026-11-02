package study

import (
	"errors"
	"log/slog"
	"time"

	"github.com/case-framework/case-backend/pkg/study/studyengine"
	"github.com/case-framework/case-backend/pkg/study/types"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
)

/* func checkIfParticipantExists(instanceID string, studyKey string, participantID string, withStatus string) bool {
pState, err := studyDBService.GetParticipantByID(instanceID, studyKey, participantID)
if err != nil || (withStatus != "" && pState.StudyStatus != withStatus) {
	return false
}
return err == nil
}
*/

func getStudyIfActive(instanceID string, studyKey string) (study studyTypes.Study, err error) {
	study, err = studyDBService.GetStudy(instanceID, studyKey)
	if err != nil {
		return study, err
	}

	if study.Status != studyTypes.STUDY_STATUS_ACTIVE {
		return study, errors.New("study is not active")
	}

	return study, nil
}

func getAndPerformStudyRules(instanceID, studyKey string, pState studyTypes.Participant, currentEvent studyengine.StudyEvent) (newState studyengine.ActionData, err error) {
	newState = studyengine.ActionData{
		PState:          pState,
		ReportsToCreate: []types.Report{},
	}

	rulesObj, err := studyDBService.GetCurrentStudyRules(instanceID, studyKey)
	if err != nil {
		return
	}
	for _, rule := range rulesObj.Rules {
		newState, err = studyengine.ActionEval(rule, newState, currentEvent)
		if err != nil {
			return
		}
	}

	return newState, nil
}

func saveResponses(instanceID string, studyKey string, response studyTypes.SurveyResponse, pState studyTypes.Participant, confidentialID string) (string, error) {
	nonConfidentialResponses := []studyTypes.SurveyItemResponse{}
	confidentialResponses := []studyTypes.SurveyItemResponse{}

	for _, item := range response.Responses {
		if len(item.ConfidentialMode) > 0 {
			item.Meta = types.ResponseMeta{}
			confidentialResponses = append(confidentialResponses, item)
		} else {
			nonConfidentialResponses = append(nonConfidentialResponses, item)
		}
	}
	response.Responses = nonConfidentialResponses
	response.ParticipantID = pState.ParticipantID

	if response.Context == nil {
		response.Context = map[string]string{}
	}
	response.Context["session"] = pState.CurrentStudySession

	var rID string
	var err error
	if len(nonConfidentialResponses) > 0 || len(confidentialResponses) < 1 {
		// Save responses only if non empty or there were no confidential responses
		rID, err = studyDBService.AddSurveyResponse(instanceID, studyKey, response)
		if err != nil {
			return "", err
		}
	}

	// Save confidential data:
	if len(confidentialResponses) > 0 {
		for _, confItem := range confidentialResponses {
			itemKey := confItem.Key
			if len(confItem.MapToKey) > 0 {
				itemKey = confItem.MapToKey
				confItem.Key = itemKey
			}
			rItem := studyTypes.SurveyResponse{
				Key:           itemKey,
				ParticipantID: confidentialID,
				Responses:     []studyTypes.SurveyItemResponse{confItem},
			}
			if confItem.ConfidentialMode == "add" {
				_, err := studyDBService.AddConfidentialResponse(instanceID, studyKey, rItem)
				if err != nil {
					slog.Error("Unexpected error", slog.String("error", err.Error()))
				}
			} else {
				// Replace
				err := studyDBService.ReplaceConfidentialResponse(instanceID, studyKey, rItem)
				if err != nil {
					slog.Error("Unexpected error", slog.String("error", err.Error()))
				}
			}
		}
	}

	return rID, nil
}

func saveReports(instanceID string, studyKey string, reports []studyTypes.Report, withResponseID string) {
	// save reports
	for _, report := range reports {
		report.ResponseID = withResponseID
		err := studyDBService.SaveReport(instanceID, studyKey, report)
		if err != nil {
			slog.Error("Error saving report", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("participantID", report.ParticipantID), slog.String("error", err.Error()), slog.String("reportKey", report.Key))
		} else {
			slog.Debug("Report saved.", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("participantID", report.ParticipantID), slog.String("reportKey", report.Key))
		}
	}
}

func isSurveyAssignedAndActive(pState studyTypes.Participant, surveyKey string) bool {
	now := time.Now().Unix()

	for _, as := range pState.AssignedSurveys {
		if as.SurveyKey != surveyKey {
			continue
		}

		if as.ValidFrom > 0 && now < as.ValidFrom {
			continue
		}

		if as.ValidUntil > 0 && now > as.ValidUntil {
			continue
		}

		// --> survey is currently active
		return true
	}

	return false
}

func hasRuleForEventType(rules []types.Expression, event studyengine.StudyEvent) bool {
	for _, rule := range rules {
		if len(rule.Data) < 1 {
			continue
		}
		exp := rule.Data[0].Exp
		if exp == nil || len(exp.Data) < 1 || exp.Data[0].Str != event.Type {
			continue
		}
		return true
	}
	return false
}
