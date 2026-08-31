package studyengine

import (
	"fmt"
	"testing"
	"time"

	studyDB "github.com/case-framework/case-backend/pkg/db/study"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Reference/Lookup methods
func TestEvalCheckEventType(t *testing.T) {
	exp := studyTypes.Expression{Name: "checkEventType", Data: []studyTypes.ExpressionArg{
		{DType: "str", Str: "ENTER"},
	}}

	t.Run("for matching", func(t *testing.T) {
		EvalContext := EvalContext{
			Event: StudyEvent{Type: "ENTER"},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected type or value: %s", ret)
		}
	})

	t.Run("for not matching", func(t *testing.T) {
		EvalContext := EvalContext{
			Event: StudyEvent{Type: "enter"},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected type or value: %s", ret)
		}
	})
}

func TestEvalCheckSurveyResponseKey(t *testing.T) {
	exp := studyTypes.Expression{Name: "checkSurveyResponseKey", Data: []studyTypes.ExpressionArg{
		{DType: "str", Str: "weekly"},
	}}

	t.Run("for no survey responses at all", func(t *testing.T) {
		EvalContext := EvalContext{
			Event: StudyEvent{Type: "SUBMIT"},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected type or value: %s", ret)
		}
	})

	t.Run("not matching key", func(t *testing.T) {
		EvalContext := EvalContext{
			Event: StudyEvent{
				Type: "SUBMIT",
				Response: studyTypes.SurveyResponse{
					Key:       "intake",
					Responses: []studyTypes.SurveyItemResponse{},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected type or value: %s", ret)
		}
	})

	t.Run("for matching key", func(t *testing.T) {
		EvalContext := EvalContext{
			Event: StudyEvent{
				Type: "SUBMIT",
				Response: studyTypes.SurveyResponse{
					Key:       "weekly",
					Responses: []studyTypes.SurveyItemResponse{},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected type or value: %s", ret)
		}
	})
}

func TestEvalHasStudyStatus(t *testing.T) {
	t.Run("with not matching state", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasStudyStatus", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: studyTypes.PARTICIPANT_STUDY_STATUS_EXITED},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("with matching state", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasStudyStatus", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

func TestEvalHasEventPayload(t *testing.T) {
	evalHasEventPayload := func(payload map[string]interface{}) (interface{}, error) {
		exp := studyTypes.Expression{Name: "hasEventPayload"}
		evalContext := EvalContext{
			Event: StudyEvent{
				Payload: payload,
			},
		}
		return ExpressionEval(exp, evalContext)
	}

	t.Run("Should return true if event payload is present", func(t *testing.T) {

		payload := map[string]interface{}{
			"test": "test",
		}

		ret, err := evalHasEventPayload(payload)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
	t.Run("Should return false if event payload is not present", func(t *testing.T) {
		payload := map[string]interface{}{}
		ret, err := evalHasEventPayload(payload)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("Should return false if event payload is not present", func(t *testing.T) {
		ret, err := evalHasEventPayload(nil)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

type MockStudyDBService struct {
	Responses []studyTypes.SurveyResponse
	Variables map[string]studyTypes.StudyVariables
	Updated   []struct {
		Key   string
		Value any
	}
}

func (db MockStudyDBService) GetResponses(instanceID string, studyKey string, filter bson.M, sort bson.M, page int64, limit int64) (responses []studyTypes.SurveyResponse, paginationInfo *studyDB.PaginationInfos, err error) {
	for _, r := range db.Responses {
		if filter["participantID"] != r.ParticipantID {
			continue
		}
		keyFilter, ok := filter["key"]
		if ok && len(keyFilter.(string)) > 0 && keyFilter != r.Key {
			continue
		}
		responses = append(responses, r)
	}

	return responses, nil, nil
}

func (db MockStudyDBService) DeleteConfidentialResponses(instanceID string, studyKey string, participantID string, key string) (count int64, err error) {
	return
}

func (db MockStudyDBService) SaveResearcherMessage(instanceID string, studyKey string, message studyTypes.StudyMessage) error {
	return nil
}

func (db MockStudyDBService) StudyCodeListEntryExists(instanceID string, studyKey string, listKey string, code string) (bool, error) {
	return false, nil
}

func (db MockStudyDBService) DeleteStudyCodeListEntry(instanceID string, studyKey string, listKey string, code string) error {
	return nil
}

func (db MockStudyDBService) DrawStudyCode(instanceID string, studyKey string, listKey string) (string, error) {
	return "", nil
}

func (db MockStudyDBService) GetCurrentStudyCounterValue(instanceID string, studyKey string, scope string) (int64, error) {
	return 0, nil
}

func (db MockStudyDBService) IncrementAndGetStudyCounterValue(instanceID string, studyKey string, scope string) (int64, error) {
	return 0, nil
}

func (db MockStudyDBService) RemoveStudyCounterValue(instanceID string, studyKey string, scope string) error {
	return nil
}

func (db MockStudyDBService) GetStudyVariableByStudyKeyAndKey(instanceID string, studyKey string, key string, onlyValue bool) (studyTypes.StudyVariables, error) {
	if db.Variables == nil {
		return studyTypes.StudyVariables{}, nil
	}
	v, ok := db.Variables[key]
	if !ok {
		return studyTypes.StudyVariables{}, nil
	}
	return v, nil
}

func (db *MockStudyDBService) UpdateStudyVariableValue(instanceID string, studyKey string, key string, value any) (studyTypes.StudyVariables, error) {
	// Note: Using value receiver; copy then append to Updated for assertions
	entry := struct {
		Key   string
		Value any
	}{Key: key, Value: value}
	db.Updated = append(db.Updated, entry)
	return studyTypes.StudyVariables{}, nil
}

func TestEvalCheckConditionForOldResponses(t *testing.T) {

	testResponses := []studyTypes.SurveyResponse{
		{
			Key: "S1", ParticipantID: "P1", SubmittedAt: 10, Responses: []studyTypes.SurveyItemResponse{
				{Key: "S1.Q1", Response: &studyTypes.ResponseItem{
					Key: "rg", Items: []*studyTypes.ResponseItem{
						{Key: "scg", Items: []*studyTypes.ResponseItem{{Key: "1"}}},
					},
				}}},
		},
		{
			Key: "S1", ParticipantID: "P1", SubmittedAt: 13, Responses: []studyTypes.SurveyItemResponse{
				{Key: "S1.Q1", Response: &studyTypes.ResponseItem{
					Key: "rg", Items: []*studyTypes.ResponseItem{
						{Key: "scg", Items: []*studyTypes.ResponseItem{{Key: "1"}}},
					},
				}}},
		},
		{
			Key: "S1", ParticipantID: "P2", SubmittedAt: 13, Responses: []studyTypes.SurveyItemResponse{
				{Key: "S1.Q1", Response: &studyTypes.ResponseItem{
					Key: "rg", Items: []*studyTypes.ResponseItem{
						{Key: "scg", Items: []*studyTypes.ResponseItem{{Key: "1"}}},
					},
				}}},
		},
		{
			Key: "S2", ParticipantID: "P1", SubmittedAt: 15, Responses: []studyTypes.SurveyItemResponse{
				{Key: "S2.Q1", Response: &studyTypes.ResponseItem{
					Key: "rg", Items: []*studyTypes.ResponseItem{
						{Key: "scg", Items: []*studyTypes.ResponseItem{{Key: "1"}}},
					},
				}}},
		},
		{
			Key: "S1", ParticipantID: "P1", SubmittedAt: 17, Responses: []studyTypes.SurveyItemResponse{
				{Key: "S1.Q1", Response: &studyTypes.ResponseItem{
					Key: "rg", Items: []*studyTypes.ResponseItem{
						{Key: "scg", Items: []*studyTypes.ResponseItem{{Key: "1"}}},
					},
				}}},
		},
		{
			Key: "S1", ParticipantID: "P1", SubmittedAt: 22, Responses: []studyTypes.SurveyItemResponse{
				{Key: "S1.Q1", Response: &studyTypes.ResponseItem{
					Key: "rg", Items: []*studyTypes.ResponseItem{
						{Key: "scg", Items: []*studyTypes.ResponseItem{{Key: "2"}}},
					},
				}}},
		},
	}

	CurrentStudyEngine = &StudyEngine{
		studyDBService: &MockStudyDBService{
			Responses: testResponses,
		},
	}

	t.Run("missing DB config", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "checkConditionForOldResponses"}

		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return error")
			return
		}
	})

	t.Run("missing instanceID", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "checkConditionForOldResponses"}

		EvalContext := EvalContext{
			Event: StudyEvent{
				StudyKey: "testStudy",
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return error")
			return
		}
	})

	t.Run("missing studyKey", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "checkConditionForOldResponses"}

		EvalContext := EvalContext{
			Event: StudyEvent{
				StudyKey:   "testStudy",
				InstanceID: "testInstance",
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return error")
			return
		}
	})

	t.Run("missing condition", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "checkConditionForOldResponses"}

		EvalContext := EvalContext{
			Event: StudyEvent{
				StudyKey:   "testStudy",
				InstanceID: "testInstance",
			},
			ParticipantState: studyTypes.Participant{
				ParticipantID: "P1",
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return error")
			return
		}
	})

	t.Run("checkType all", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "checkConditionForOldResponses", Data: []studyTypes.ExpressionArg{
			{Exp: &studyTypes.Expression{
				Name: "responseHasKeysAny",
				Data: []studyTypes.ExpressionArg{
					{Str: "S1.Q1", DType: "str"},
					{Str: "rg.scg", DType: "str"},
					{Str: "1", DType: "str"},
				},
			}, DType: "exp"},
		}}

		EvalContext := EvalContext{
			Event: StudyEvent{
				StudyKey:   "testStudy",
				InstanceID: "testInstance",
			},
			ParticipantState: studyTypes.Participant{
				ParticipantID: "P1",
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("checkType any", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "checkConditionForOldResponses", Data: []studyTypes.ExpressionArg{
			{Exp: &studyTypes.Expression{
				Name: "responseHasKeysAny",
				Data: []studyTypes.ExpressionArg{
					{Str: "S1.Q1", DType: "str"},
					{Str: "rg.scg", DType: "str"},
					{Str: "1", DType: "str"},
				},
			}, DType: "exp"},
			{Str: "any", DType: "str"},
		}}

		EvalContext := EvalContext{
			Event: StudyEvent{
				StudyKey:   "testStudy",
				InstanceID: "testInstance",
			},
			ParticipantState: studyTypes.Participant{
				ParticipantID: "P1",
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("checkType count - with enough", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "checkConditionForOldResponses", Data: []studyTypes.ExpressionArg{
			{Exp: &studyTypes.Expression{
				Name: "responseHasKeysAny",
				Data: []studyTypes.ExpressionArg{
					{Str: "S1.Q1", DType: "str"},
					{Str: "rg.scg", DType: "str"},
					{Str: "1", DType: "str"},
				},
			}, DType: "exp"},
			{Num: 3, DType: "num"},
		}}

		EvalContext := EvalContext{
			Event: StudyEvent{
				StudyKey:   "testStudy",
				InstanceID: "testInstance",
			},
			ParticipantState: studyTypes.Participant{
				ParticipantID: "P1",
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("checkType count - with not enough", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "checkConditionForOldResponses", Data: []studyTypes.ExpressionArg{
			{Exp: &studyTypes.Expression{
				Name: "responseHasKeysAny",
				Data: []studyTypes.ExpressionArg{
					{Str: "S1.Q1", DType: "str"},
					{Str: "rg.scg", DType: "str"},
					{Str: "1", DType: "str"},
				},
			}, DType: "exp"},
			{Num: 4, DType: "num"},
		}}

		EvalContext := EvalContext{
			Event: StudyEvent{
				StudyKey:   "testStudy",
				InstanceID: "testInstance",
			},
			ParticipantState: studyTypes.Participant{
				ParticipantID: "P1",
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("filter for survey type", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "checkConditionForOldResponses", Data: []studyTypes.ExpressionArg{
			{Exp: &studyTypes.Expression{
				Name: "responseHasKeysAny",
				Data: []studyTypes.ExpressionArg{
					{Str: "S1.Q1", DType: "str"},
					{Str: "rg.scg", DType: "str"},
					{Str: "1", DType: "str"},
				},
			}, DType: "exp"},
			{Num: 4, DType: "num"},
			{Str: "S2", DType: "str"},
		}}

		EvalContext := EvalContext{
			Event: StudyEvent{
				StudyKey:   "testStudy",
				InstanceID: "testInstance",
			},
			ParticipantState: studyTypes.Participant{
				ParticipantID: "P1",
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})
}

func TestEvalGetStudyEntryTime(t *testing.T) {
	t.Run("try retrieve entered at time", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getStudyEntryTime"}
		tStart := time.Now().Unix()
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				EnteredAt:   tStart,
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(float64) != float64(tStart) {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})
}

func TestEvalHasSurveyKeyAssigned(t *testing.T) {
	t.Run("has survey assigned", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasSurveyKeyAssigned", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test1"},
					{SurveyKey: "test2"},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("doesn't have the survey assigned", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasSurveyKeyAssigned", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test2"},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("missing argument", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasSurveyKeyAssigned", Data: []studyTypes.ExpressionArg{}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test2"},
				},
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should throw an error about missing arg")
			return
		}
	})

	t.Run("wrong argument", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasSurveyKeyAssigned", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{}},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test2"},
				},
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should throw an error about arg type")
			return
		}
	})
}

func TestEvalGetSurveyKeyAssignedFrom(t *testing.T) {
	t.Run("has survey assigned", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getSurveyKeyAssignedFrom", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test1", ValidFrom: 10, ValidUntil: 100},
					{SurveyKey: "test2", ValidFrom: 10, ValidUntil: 100},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(float64) != 10 {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("doesn't have the survey assigned", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getSurveyKeyAssignedFrom", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test2", ValidFrom: 10, ValidUntil: 100},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(float64) != -1 {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("missing argument", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getSurveyKeyAssignedFrom", Data: []studyTypes.ExpressionArg{}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test1", ValidFrom: 10, ValidUntil: 100},
					{SurveyKey: "test2", ValidFrom: 10, ValidUntil: 100},
				},
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should throw an error about missing arg")
			return
		}
	})

	t.Run("wrong argument", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getSurveyKeyAssignedFrom", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{}},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test1", ValidFrom: 10, ValidUntil: 100},
					{SurveyKey: "test2", ValidFrom: 10, ValidUntil: 100},
				},
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should throw an error about arg type")
			return
		}
	})
}

func TestEvalGetSurveyKeyAssignedUntil(t *testing.T) {
	t.Run("has survey assigned", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getSurveyKeyAssignedUntil", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test1", ValidFrom: 10, ValidUntil: 100},
					{SurveyKey: "test2", ValidFrom: 10, ValidUntil: 100},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(float64) != 100 {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("doesn't have the survey assigned", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getSurveyKeyAssignedUntil", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test2", ValidFrom: 10, ValidUntil: 100},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(float64) != -1 {
			t.Errorf("unexpected value retrieved: %d", ret)
		}
	})

	t.Run("missing argument", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getSurveyKeyAssignedUntil", Data: []studyTypes.ExpressionArg{}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test1", ValidFrom: 10, ValidUntil: 100},
					{SurveyKey: "test2", ValidFrom: 10, ValidUntil: 100},
				},
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should throw an error about missing arg")
			return
		}
	})

	t.Run("wrong argument", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getSurveyKeyAssignedUntil", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{}},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				AssignedSurveys: []studyTypes.AssignedSurvey{
					{SurveyKey: "test1", ValidFrom: 10, ValidUntil: 100},
					{SurveyKey: "test2", ValidFrom: 10, ValidUntil: 100},
				},
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should throw an error about arg type")
			return
		}
	})
}

func TestEvalHasParticipantFlag(t *testing.T) {
	t.Run("participant hasn't got any participant flags (empty / nil)", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlag", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "key1"},
			{DType: "str", Str: "value1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(bool) {
			t.Error("should be false")
		}
	})

	t.Run("participant has other participant flags, but this key is missing", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlag", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "key1"},
			{DType: "str", Str: "value1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				Flags: map[string]string{
					"key2": "value1",
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(bool) {
			t.Error("should be false")
		}
	})

	t.Run("participant has correct participant flag's key, but value is different", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlag", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "key1"},
			{DType: "str", Str: "value1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				Flags: map[string]string{
					"key1": "value2",
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(bool) {
			t.Error("should be false")
		}
	})

	t.Run("participant has correct participant flag's key and value is same", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlag", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "key1"},
			{DType: "str", Str: "value1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				Flags: map[string]string{
					"key1": "value1",
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if !ret.(bool) {
			t.Error("should be true")
		}
	})

	t.Run("missing arguments", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlag", Data: []studyTypes.ExpressionArg{}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				Flags: map[string]string{
					"key1": "value1",
				},
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should throw error")
			return
		}
	})

	t.Run("using num at 1st argument (expressions allowed, should return string)", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlag", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 22},
			{DType: "str", Str: "value1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				Flags: map[string]string{
					"key1": "value1",
				},
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should throw error")
			return
		}
	})

	t.Run("missing arguments", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlag", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "key1"},
			{DType: "num", Num: 22},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				Flags: map[string]string{
					"key1": "value1",
				},
			},
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should throw error")
			return
		}
	})
}

func TestEvalHasParticipantFlagKey(t *testing.T) {
	t.Run("participant hasn't got any participant flags (empty / nil)", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlagKey", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "key1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(bool) {
			t.Error("should be false")
		}
	})

	t.Run("participant has other key", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlagKey", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "key1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				Flags: map[string]string{
					"key2": "1",
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if ret.(bool) {
			t.Error("should be false")
		}
	})

	t.Run("participant has correct key", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasParticipantFlagKey", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "key1"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				Flags: map[string]string{
					"key2": "1",
					"key1": "1",
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if !ret.(bool) {
			t.Error("should be true")
		}
	})
}

func TestEvalHasResponseKey(t *testing.T) {
	testEvalContext := EvalContext{
		Event: StudyEvent{
			Type: "SUBMIT",
			Response: studyTypes.SurveyResponse{
				Key: "weekly",
				Responses: []studyTypes.SurveyItemResponse{
					{
						Key: "weekly.Q1", Response: &studyTypes.ResponseItem{
							Key: "rg", Items: []*studyTypes.ResponseItem{
								{Key: "1", Value: "something"},
								{Key: "2"},
							}},
					},
					{
						Key: "weekly.Q2", Response: &studyTypes.ResponseItem{
							Key: "rg", Items: []*studyTypes.ResponseItem{
								{Key: "1", Value: "123.23", Dtype: "date"},
							}},
					},
				},
			},
		},
	}

	//
	t.Run("no survey item response found", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasResponseKey", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q3"},
			{DType: "str", Str: "rg.1"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if v.(bool) {
			t.Errorf("unexpected value: %b", v)
		}
	})

	t.Run("repsonse item in question missing", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasResponseKey", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.wrong"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if v.(bool) {
			t.Errorf("unexpected value: %b", v)
		}
	})

	t.Run("repsonse item in partly there missing", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasResponseKey", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.1.1"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if v.(bool) {
			t.Errorf("unexpected value: %b", v)
		}
	})

	t.Run("has key", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasResponseKey", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.2"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !v.(bool) {
			t.Errorf("unexpected value: %b", v)
		}
	})
}
func TestEvalHasResponseKeyWithValue(t *testing.T) {
	testEvalContext := EvalContext{
		Event: StudyEvent{
			Type: "SUBMIT",
			Response: studyTypes.SurveyResponse{
				Key: "weekly",
				Responses: []studyTypes.SurveyItemResponse{
					{
						Key: "weekly.Q1", Response: &studyTypes.ResponseItem{
							Key: "rg", Items: []*studyTypes.ResponseItem{
								{Key: "1", Value: "something"},
								{Key: "2"},
							}},
					},
					{
						Key: "weekly.Q2", Response: &studyTypes.ResponseItem{
							Key: "rg", Items: []*studyTypes.ResponseItem{
								{Key: "1", Value: "123.23", Dtype: "date"},
							}},
					},
				},
			},
		},
	}

	//
	t.Run("no survey item response found", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasResponseKeyWithValue", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q3"},
			{DType: "str", Str: "rg.1"},
			{DType: "str", Str: "something"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if v.(bool) {
			t.Errorf("unexpected value: %b", v)
		}
	})

	t.Run("repsonse item in question missing", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasResponseKeyWithValue", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.wrong"},
			{DType: "str", Str: "something"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if v.(bool) {
			t.Errorf("unexpected value: %b", v)
		}
	})

	t.Run("has empty value", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasResponseKeyWithValue", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.2"},
			{DType: "str", Str: "something"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if v.(bool) {
			t.Errorf("unexpected value: %b", v)
		}
	})

	t.Run("normal", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasResponseKeyWithValue", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.1"},
			{DType: "str", Str: "something"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !v.(bool) {
			t.Errorf("unexpected value: %b", v)
		}
	})
}
func TestEvalGetResponseValueAsNum(t *testing.T) {
	testEvalContext := EvalContext{
		Event: StudyEvent{
			Type: "SUBMIT",
			Response: studyTypes.SurveyResponse{
				Key: "weekly",
				Responses: []studyTypes.SurveyItemResponse{
					{
						Key: "weekly.Q1", Response: &studyTypes.ResponseItem{
							Key: "rg", Items: []*studyTypes.ResponseItem{
								{Key: "1", Value: "something"},
								{Key: "2"},
							}},
					},
					{
						Key: "weekly.Q2", Response: &studyTypes.ResponseItem{
							Key: "rg", Items: []*studyTypes.ResponseItem{
								{Key: "1", Value: "123.23", Dtype: "date"},
							}},
					},
				},
			},
		},
	}

	//
	t.Run("no survey item response found", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getResponseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q3"},
			{DType: "str", Str: "rg.1"},
		}}
		_, err := ExpressionEval(exp, testEvalContext)
		if err == nil {
			t.Error("should return an error")
			return
		}
	})

	t.Run("repsonse item in question missing", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getResponseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.wrong"},
		}}
		_, err := ExpressionEval(exp, testEvalContext)
		if err == nil {
			t.Error("should return an error")
			return
		}
	})

	t.Run("has empty value", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getResponseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.2"},
		}}
		_, err := ExpressionEval(exp, testEvalContext)
		if err == nil {
			t.Error("should return an error")
			return
		}
	})

	t.Run("repsonse item's value is not a number", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getResponseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.1"},
		}}
		_, err := ExpressionEval(exp, testEvalContext)
		if err == nil {
			t.Error("should return an error")
			return
		}
	})

	t.Run("is number", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getResponseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q2"},
			{DType: "str", Str: "rg.1"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if v.(float64) != 123.23 {
			t.Errorf("unexpected value: %b", v)
		}
	})
}

func TestEvalCountResponseItems(t *testing.T) {
	testEvalContext := EvalContext{
		Event: StudyEvent{
			Type: "SUBMIT",
			Response: studyTypes.SurveyResponse{
				Key: "weekly",
				Responses: []studyTypes.SurveyItemResponse{
					{
						Key: "weekly.Q1", Response: &studyTypes.ResponseItem{
							Key: "rg", Items: []*studyTypes.ResponseItem{
								{Key: "mcg", Items: []*studyTypes.ResponseItem{
									{Key: "1"},
									{Key: "2"},
									{Key: "3"},
								}},
							}},
					},
					{
						Key: "weekly.Q2", Response: &studyTypes.ResponseItem{
							Key: "rg", Items: []*studyTypes.ResponseItem{
								{Key: "mcg", Items: []*studyTypes.ResponseItem{}},
							}},
					},
				},
			},
		},
	}

	//
	t.Run("no survey item response found", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "countResponseItems", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q3"},
			{DType: "str", Str: "rg.mcg"},
		}}
		_, err := ExpressionEval(exp, testEvalContext)
		if err == nil {
			t.Error("should return an error")
			return
		}
	})

	t.Run("repsonse item in question missing", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "countResponseItems", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.wrong"},
		}}
		_, err := ExpressionEval(exp, testEvalContext)
		if err == nil {
			t.Error("should return an error")
			return
		}
	})

	t.Run("has empty value", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "countResponseItems", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q2"},
			{DType: "str", Str: "rg.mcg"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if v.(float64) != 0.0 {
			t.Errorf("unexpected value: %b", v)
		}
	})

	t.Run("has 3 values", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "countResponseItems", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.mcg"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if v.(float64) != 3.0 {
			t.Errorf("unexpected value: %b", v)
		}
	})
}

func TestEvalGetResponseValueAsStr(t *testing.T) {
	testEvalContext := EvalContext{
		Event: StudyEvent{
			Type: "SUBMIT",
			Response: studyTypes.SurveyResponse{
				Key: "weekly",
				Responses: []studyTypes.SurveyItemResponse{
					{
						Key: "weekly.Q1", Response: &studyTypes.ResponseItem{
							Key: "rg", Items: []*studyTypes.ResponseItem{
								{Key: "1", Value: "something"},
								{Key: "2"},
							}},
					},
				},
			},
		},
	}

	//
	t.Run("no survey item response found", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getResponseValueAsStr", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q3"},
			{DType: "str", Str: "rg.1"},
		}}
		_, err := ExpressionEval(exp, testEvalContext)
		if err == nil {
			t.Error("should return an error")
			return
		}
	})

	t.Run("repsonse item in question missing", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getResponseValueAsStr", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.wrong"},
		}}
		_, err := ExpressionEval(exp, testEvalContext)
		if err == nil {
			t.Error("should return an error")
			return
		}
	})

	t.Run("has empty value", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getResponseValueAsStr", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.2"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if v != "" {
			t.Errorf("unexpected value: %s instead of %s", v, "blank")
		}
	})

	t.Run("has value", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getResponseValueAsStr", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.Q1"},
			{DType: "str", Str: "rg.1"},
		}}
		v, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if v != "something" {
			t.Errorf("unexpected value: %s instead of %s", v, "something")
		}
	})
}

func TestMustGetStrValue(t *testing.T) {
	testEvalContext := EvalContext{}

	t.Run("not string value", func(t *testing.T) {
		_, err := testEvalContext.mustGetStrValue(studyTypes.ExpressionArg{
			Num:   0,
			DType: "num",
		})
		if err == nil {
			t.Error("should produce error")
		}
	})

	t.Run("string value", func(t *testing.T) {
		v, err := testEvalContext.mustGetStrValue(studyTypes.ExpressionArg{
			Str:   "hello",
			DType: "str",
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if v != "hello" {
			t.Errorf("unexpected value: %s", v)
		}
	})
}

func TestEvalResponseHasOnlyKeysOtherThan(t *testing.T) {
	testEvalContext := EvalContext{
		Event: StudyEvent{
			Type: "SUBMIT",
			Response: studyTypes.SurveyResponse{
				Key:       "wwekly",
				Responses: []studyTypes.SurveyItemResponse{},
			},
		},
	}

	t.Run("no survey item response found", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasOnlyKeysOtherThan", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q2", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "mcg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})

	t.Run("with response item found, but no response parent group", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasOnlyKeysOtherThan", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q1", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "scg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})

	t.Run("response group does include at least one", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasOnlyKeysOtherThan", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q1", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "mcg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
				{Key: "1"},
				{Key: "3"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})

	t.Run("response group is empty", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasOnlyKeysOtherThan", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q1", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "mcg", Items: []*studyTypes.ResponseItem{}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})

	t.Run("response group includes all and other responses", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasOnlyKeysOtherThan", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q1", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "mcg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
				{Key: "1"},
				{Key: "2"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})

	t.Run("response group includes none of the options", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasOnlyKeysOtherThan", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q1", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "mcg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
				{Key: "3"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

func TestEvalResponseHasKeysAny(t *testing.T) {
	testEvalContext := EvalContext{
		Event: StudyEvent{
			Type: "SUBMIT",
			Response: studyTypes.SurveyResponse{
				Key:       "wwekly",
				Responses: []studyTypes.SurveyItemResponse{},
			},
		},
	}
	t.Run("no survey item response found", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasKeysAny", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q2", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "mcg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})
	t.Run("with response item found, but no response parent group", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasKeysAny", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q1", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "scg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})

	t.Run("response group does not include any", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasKeysAny", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q1", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "mcg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
				{Key: "3"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})

	t.Run("response group includes all and other responses", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasKeysAny", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q1", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "mcg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
				{Key: "1"},
				{Key: "2"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})
	t.Run("response group includes only of the multiple options", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "responseHasKeysAny", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "weekly.G1.Q1"},
			{DType: "str", Str: "rg.mcg"},
			{DType: "str", Str: "1"},
			{DType: "str", Str: "2"},
		}}
		testEvalContext.Event.Response.Responses = []studyTypes.SurveyItemResponse{
			{Key: "weekly.G1.Q1", Response: &studyTypes.ResponseItem{Key: "rg", Items: []*studyTypes.ResponseItem{{Key: "mcg", Items: []*studyTypes.ResponseItem{
				{Key: "0"},
				{Key: "1"},
			}}}}},
		}
		ret, err := ExpressionEval(exp, testEvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}

	})

}

func TestEvalGetLastSubmissionDate(t *testing.T) {
	t.Run("with no submissions", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getLastSubmissionDate", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(float64) != 0 {
			t.Errorf("unexpected value: %f", ret)
		}
	})

	t.Run("with submissions", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getLastSubmissionDate", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test"},
		}}

		last_submission := time.Now().Unix() - 10
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				LastSubmissions: map[string]int64{
					"test": last_submission,
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(float64) != float64(last_submission) {
			t.Errorf("unexpected value: %f", ret)
		}
	})

	t.Run("with no arguments", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getLastSubmissionDate"}
		lastTs := time.Now().Unix() - 10
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				LastSubmissions: map[string]int64{
					"test":  lastTs,
					"test2": lastTs - 10,
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(float64) != float64(lastTs) {
			t.Errorf("unexpected value: %f", ret)
		}
	})

	t.Run("with wrong survey key", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getLastSubmissionDate", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "wrong"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				LastSubmissions: map[string]int64{
					"test": time.Now().Unix() - 10,
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(float64) != 0 {
			t.Errorf("unexpected value: %f", ret)
		}
	})
}

func TestEvalLastSubmissionDateOlderThan(t *testing.T) {
	t.Run("with not older", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lastSubmissionDateOlderThan", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
				{DType: "num", Num: -10},
			}}},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				LastSubmissions: map[string]int64{
					"s1": time.Now().Unix() - 2,
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("with specific survey is older", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lastSubmissionDateOlderThan", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
				{DType: "num", Num: -10},
			}}},
			{DType: "str", Str: "s2"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				LastSubmissions: map[string]int64{
					"s1": time.Now().Unix() - 2,
					"s2": time.Now().Unix() - 20,
				}},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("with only one type of survey is older", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lastSubmissionDateOlderThan", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 10},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				LastSubmissions: map[string]int64{
					"s1": time.Now().Unix() - 2,
					"s2": time.Now().Unix() - 20,
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("with all types are older", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lastSubmissionDateOlderThan", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
				{DType: "num", Num: -10},
			}}},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				LastSubmissions: map[string]int64{
					"s1": time.Now().Unix() - 25,
					"s2": time.Now().Unix() - 20,
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

// Comparisons
func TestEvalEq(t *testing.T) {
	t.Run("for eq numbers", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "eq", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 23},
			{DType: "num", Num: 23},
		}}
		EvalContext := EvalContext{
			Event: StudyEvent{Type: "TIMER"},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("for not equal numbers", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "eq", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 13},
			{DType: "num", Num: 23},
		}}
		EvalContext := EvalContext{
			Event: StudyEvent{Type: "enter"},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("for equal strings", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "eq", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "enter"},
			{DType: "str", Str: "enter"},
		}}
		EvalContext := EvalContext{
			Event: StudyEvent{Type: "enter"},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("for not equal strings", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "eq", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "enter"},
			{DType: "str", Str: "time..."},
		}}
		EvalContext := EvalContext{
			Event: StudyEvent{Type: "enter"},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

func TestEvalLT(t *testing.T) {
	t.Run("2 < 2", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lt", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 2},
			{DType: "num", Num: 2},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("2 < 1", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lt", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 2},
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("1 < 2", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lt", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: 2},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("a < b", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lt", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "a"},
			{DType: "str", Str: "b"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("b < b", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lt", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "b"},
			{DType: "str", Str: "b"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("b < a", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lt", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "b"},
			{DType: "str", Str: "a"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

func TestEvalLTE(t *testing.T) {
	t.Run("2 <= 2", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lte", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 2},
			{DType: "num", Num: 2},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("2 <= 1", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lte", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 2},
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("1 <= 2", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lte", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: 2},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("a <= b", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lte", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "a"},
			{DType: "str", Str: "b"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("b <= b", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lte", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "b"},
			{DType: "str", Str: "b"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("b <= a", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "lte", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "b"},
			{DType: "str", Str: "a"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

func TestEvalGT(t *testing.T) {
	t.Run("2 > 2", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gt", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 2},
			{DType: "num", Num: 2},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("2 > 1", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gt", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 2},
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("1 > 2", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gt", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: 2},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("a > b", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gt", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "a"},
			{DType: "str", Str: "b"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("b > b", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gt", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "b"},
			{DType: "str", Str: "b"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("b > a", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gt", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "b"},
			{DType: "str", Str: "a"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

func TestEvalGTE(t *testing.T) {
	t.Run("2 >= 2", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gte", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 2},
			{DType: "num", Num: 2},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("2 >= 1", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gte", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 2},
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("1 >= 2", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gte", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: 2},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("a >= b", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gte", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "a"},
			{DType: "str", Str: "b"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("b >= b", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gte", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "b"},
			{DType: "str", Str: "b"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("b >= a", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "gte", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "b"},
			{DType: "str", Str: "a"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

// Logic operators
func TestEvalAND(t *testing.T) {
	t.Run("0 && 0 ", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "and", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 0},
			{DType: "num", Num: 0},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("1 && 0 ", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "and", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: 0},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("0 && 1 ", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "and", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 0},
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("1 && 1 ", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "and", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

func TestEvalOR(t *testing.T) {
	t.Run("0 || 0 ", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "or", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 0},
			{DType: "num", Num: 0},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("1 || 0 ", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "or", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: 0},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("0 || 1 ", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "or", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 0},
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})

	t.Run("1 || 1 ", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "or", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

func TestEvalNOT(t *testing.T) {
	t.Run("0", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "not", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 0},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if !ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
	t.Run("1", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "not", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		if ret.(bool) {
			t.Errorf("unexpected value: %b", ret)
		}
	})
}

func TestEvalSum(t *testing.T) {

	testAdd := func(expected float64, label string, values ...studyTypes.ExpressionArg) {

		t.Run(fmt.Sprintf("Sum %s", label), func(t *testing.T) {
			exp := studyTypes.Expression{Name: "sum", Data: values}
			EvalContext := EvalContext{}
			ret, err := ExpressionEval(exp, EvalContext)
			if err != nil {
				t.Errorf("unexpected error: %s", err.Error())
				return
			}
			resTS := ret.(float64)
			if resTS != expected {
				t.Errorf("unexpected value: %f - expected ca. %f", ret, expected)
			}
		})
	}

	argNum := func(v float64) studyTypes.ExpressionArg {
		return studyTypes.ExpressionArg{DType: "num", Num: v}
	}

	argBool := func(v bool) studyTypes.ExpressionArg {
		var vN float64
		if v {
			vN = 1
		} else {
			vN = 0
		}
		return studyTypes.ExpressionArg{
			DType: "exp",
			Exp:   &studyTypes.Expression{Name: "or", Data: []studyTypes.ExpressionArg{argNum(vN), argNum(vN)}},
		}
	}

	testAdd(1, "0 + 1", argNum(0), argNum(1))
	testAdd(2, "1 + 1", argNum(1), argNum(1))
	testAdd(1, "-1 + 2", argNum(-1), argNum(2))
	testAdd(3, "1+1+1", argNum(1), argNum(1), argNum(1))
	testAdd(2, "true + true", argBool(true), argBool(true))
	testAdd(0, "false + false", argBool(false), argBool(false))
	testAdd(1, "true + false", argBool(true), argBool(false))
	testAdd(1, "false + true", argBool(false), argBool(true))

}

func TestEvalNeg(t *testing.T) {

	testNeg := func(v1 float64, expected float64) {
		t.Run(fmt.Sprintf("Negate %f", v1), func(t *testing.T) {
			exp := studyTypes.Expression{Name: "neg", Data: []studyTypes.ExpressionArg{
				{DType: "num", Num: v1},
			}}
			EvalContext := EvalContext{}
			ret, err := ExpressionEval(exp, EvalContext)
			if err != nil {
				t.Errorf("unexpected error: %s", err.Error())
				return
			}
			resTS := ret.(float64)
			if resTS != expected {
				t.Errorf("unexpected value: %f - expected ca. %f", ret, expected)
			}
		})
	}

	testNeg(0, 0)
	testNeg(1, -1)
	testNeg(-1, 1)
}

func TestEvalTimestampWithOffset(t *testing.T) {
	t.Run("T + 0", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 0},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := int64(ret.(float64))
		if resTS > time.Now().Unix()+1 || resTS < time.Now().Unix()-1 {
			t.Errorf("unexpected value: %d - expected ca. %d", ret, time.Now().Unix()+0)
		}
	})

	t.Run("T + 10", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 10},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := int64(ret.(float64))
		if resTS > time.Now().Unix()+11 || resTS < time.Now().Unix()+9 {
			t.Errorf("unexpected value: %d - expected ca. %d", ret, time.Now().Unix()+10)
		}
	})

	t.Run("T - 10", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: -10},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := int64(ret.(float64))
		if resTS < time.Now().Unix()-11 || resTS > time.Now().Unix()-9 {
			t.Errorf("unexpected value: %d - expected ca. %d", ret, time.Now().Unix()-10)
		}
	})

	t.Run("T + No num", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "0"},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Errorf("unexpected lack of error: parameter 1 was not num")
			return
		}
	})

	t.Run("R + 0", func(t *testing.T) {
		r := time.Now().Unix() - 31536000
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 0},
			{DType: "num", Num: float64(r)},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := int64(ret.(float64))
		if resTS > r+1 || resTS < r-1 {
			t.Errorf("unexpected value: %d - expected ca. %d", ret, r+0)
		}
	})

	t.Run("R + 10", func(t *testing.T) {
		r := time.Now().Unix() - 31536000
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 10},
			{DType: "num", Num: float64(r)},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := int64(ret.(float64))
		if resTS > r+11 || resTS < r+9 {
			t.Errorf("unexpected value: %d - expected ca. %d", ret, r+10)
		}
	})

	t.Run("R - 10", func(t *testing.T) {
		r := time.Now().Unix() - 31536000
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: -10},
			{DType: "num", Num: float64(r)},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := int64(ret.(float64))
		if resTS > r-9 || resTS < r-11 {
			t.Errorf("unexpected value: %d - expected ca. %d", ret, r-10)
		}
	})

	t.Run("R + No num", func(t *testing.T) {
		r := time.Now().Unix() - 31536000
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "0"},
			{DType: "num", Num: float64(r)},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Errorf("unexpected lack of error: parameter 1 was not num")
			return
		}
	})

	t.Run("No num + 10", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 10},
			{DType: "str", Str: "1"},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Errorf("unexpected lack of error: parameter 2 was not num")
			return
		}
	})

	t.Run("No num + No num", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "0"},
			{DType: "str", Str: "1"},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Errorf("unexpected lack of error: parameters 1 & 2 were not num")
			return
		}
	})

	t.Run("Valid Exp", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{
				DType: "exp", Exp: &studyTypes.Expression{
					Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
						{DType: "num", Num: -float64(time.Now().Unix())},
					}},
			}}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := int64(ret.(float64))
		if resTS-time.Now().Unix() > 1 {
			t.Errorf("unexpected value: %d, expected %d", resTS, time.Now().Unix())
		}
	})

	t.Run("Valid Exp + Valid Exp", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{
				Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
					{DType: "num", Num: -float64(time.Now().Unix())},
				}},
			},
			{DType: "exp", Exp: &studyTypes.Expression{
				Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
					{DType: "num", Num: -float64(time.Now().Unix())},
				}},
			},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := int64(ret.(float64))
		if resTS > 1 {
			t.Errorf("unexpected value: %d, expected %d", resTS, 0)
		}
	})

	t.Run("Not Valid Exp + Valid Exp", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{
				Name: "or", Data: []studyTypes.ExpressionArg{
					{DType: "num", Num: 1},
					{DType: "num", Num: 1},
				}},
			},
			{DType: "exp", Exp: &studyTypes.Expression{
				Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
					{DType: "num", Num: -float64(time.Now().Unix())},
				}},
			},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Errorf("unexpected lack of error")
			return
		}
	})

	t.Run("Valid Exp + Not Valid Exp", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{
				Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
					{DType: "num", Num: -float64(time.Now().Unix())},
				}},
			},
			{DType: "exp", Exp: &studyTypes.Expression{
				Name: "or", Data: []studyTypes.ExpressionArg{
					{DType: "num", Num: 1},
					{DType: "num", Num: 1},
				}},
			},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Errorf("unexpected lack of error")
			return
		}
	})
}

func TestEvalGetISOWeekForTs(t *testing.T) {
	t.Run("wrong argument type", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getISOWeekForTs", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test"},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return type error")
			return
		}
	})
	t.Run("with number", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getISOWeekForTs", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: float64(time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local).Unix())},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		iw := ret.(float64)
		if iw != 1 {
			t.Errorf("unexpected value: %f", iw)
			return
		}
	})

	t.Run("with expression", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getISOWeekForTs", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{
				Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
					{DType: "num", Num: 0},
				},
			},
			},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		iw := ret.(float64)
		_, ciw := time.Now().ISOWeek()
		if iw != float64(ciw) {
			t.Errorf("unexpected value: %f (should be %d)", iw, ciw)
			return
		}
	})
}

func TestEvalGetTsForNextStartOfMonth(t *testing.T) {

	t.Run("wrong month name", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextStartOfMonth", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "wrong"},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return type error")
			return
		}
	})

	t.Run("wrong month index", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextStartOfMonth", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 0},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return type error")
			return
		}
	})

	t.Run("wrong reference type", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextStartOfMonth", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 3},
			{DType: "str", Str: "test"},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return type error")
			return
		}
	})

	t.Run("without reference", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextStartOfMonth", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "January"},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		ts := ret.(float64)
		tsD := time.Unix(int64(ts), 0)
		refTs := time.Now().AddDate(1, 0, 0)
		// beginning of the year
		refTs = time.Date(refTs.Year(), 1, 1, 0, 0, 0, 0, time.Local)

		y_i := refTs.Unix()
		y := tsD.Unix()
		if y != y_i {
			t.Errorf("unexpected value: %d, expected %d", y, y_i)
		}
	})

	t.Run("with absolute reference", func(t *testing.T) {
		refTs := time.Date(2023, 9, 10, 0, 0, 0, 0, time.Local)
		exp := studyTypes.Expression{Name: "getTsForNextStartOfMonth", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: float64(refTs.Unix())},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		ts := ret.(float64)
		tsD := time.Unix(int64(ts), 0)
		refTs = refTs.AddDate(1, 0, 0)
		// beginning of the year
		refTs = time.Date(refTs.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		y_i := refTs.Unix()
		y := tsD.Unix()
		if y != y_i {
			t.Errorf("unexpected value: %d, expected %d", y, y_i)
		}

	})

	t.Run("with relative reference", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextStartOfMonth", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "exp", Exp: &studyTypes.Expression{
				Name: "timestampWithOffset",
				Data: []studyTypes.ExpressionArg{
					{DType: "num", Num: 0},
				},
			}},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		ts := ret.(float64)
		tsD := time.Unix(int64(ts), 0)
		refTs := time.Now().AddDate(1, 0, 0)
		// beginning of the year
		refTs = time.Date(refTs.Year(), 1, 1, 0, 0, 0, 0, time.Local)

		y_i := refTs.Unix()
		y := tsD.Unix()
		if y != y_i {
			t.Errorf("unexpected value: %d, expected %d", y, y_i)
		}
	})
}

func TestEvalGetTsForNextISOWeek(t *testing.T) {
	t.Run("wrong iso week type", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextISOWeek", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "test"},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return type error")
			return
		}
	})

	t.Run("with iso week not in range", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextISOWeek", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 0},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return range error")
			return
		}
	})

	t.Run("wrong reference type", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextISOWeek", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 3},
			{DType: "str", Str: "test"},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return type error")
			return
		}
	})

	t.Run("without reference", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextISOWeek", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		ts := ret.(float64)
		tsD := time.Unix(int64(ts), 0)
		refTs := Now().AddDate(1, 0, 0)
		// Find the first date in ISO week 1. January 1 can belong to
		// the previous ISO year (for example, January 1, 2027 is 2026-W53).
		refTs = time.Date(refTs.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		for {
			_, week := refTs.ISOWeek()
			if week == 1 {
				break
			}
			refTs = refTs.AddDate(0, 0, 1)
		}

		y_i, w_i := refTs.ISOWeek()
		y, w := tsD.ISOWeek()
		if y != y_i || w != w_i {
			t.Errorf("unexpected value: %d-%d, expected %d-%d", y, w, y_i, w_i)
		}
	})

	t.Run("with absolute reference", func(t *testing.T) {
		refTs := time.Date(2023, 9, 10, 0, 0, 0, 0, time.Local)
		exp := studyTypes.Expression{Name: "getTsForNextISOWeek", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "num", Num: float64(refTs.Unix())},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		ts := ret.(float64)
		tsD := time.Unix(int64(ts), 0)
		refTs = refTs.AddDate(1, 0, 0)
		// beginning of the year
		refTs = time.Date(refTs.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		y_i, w_i := refTs.ISOWeek()
		y, w := tsD.ISOWeek()
		if y != y_i || w != w_i {
			t.Errorf("unexpected value: %d-%d, expected %d-%d", y, w, y_i, w_i)
		}

	})

	t.Run("with relative reference", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getTsForNextISOWeek", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 1},
			{DType: "exp", Exp: &studyTypes.Expression{
				Name: "timestampWithOffset",
				Data: []studyTypes.ExpressionArg{
					{DType: "num", Num: 0},
				},
			}},
		}}
		EvalContext := EvalContext{}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		ts := ret.(float64)
		tsD := time.Unix(int64(ts), 0)
		refTs := Now().AddDate(1, 0, 0)
		// Find the first date in ISO week 1. January 1 can belong to
		// the previous ISO year (for example, January 1, 2027 is 2026-W53).
		refTs = time.Date(refTs.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		for {
			_, week := refTs.ISOWeek()
			if week == 1 {
				break
			}
			refTs = refTs.AddDate(0, 0, 1)
		}

		y_i, w_i := refTs.ISOWeek()
		y, w := tsD.ISOWeek()
		if y != y_i || w != w_i {
			t.Errorf("unexpected value: %d-%d, expected %d-%d", y, w, y_i, w_i)
		}
	})
}

func TestEvalHasMessageTypeAssigned(t *testing.T) {
	t.Run("participant has no messages", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasMessageTypeAssigned", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "testMessage"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				Messages: []studyTypes.ParticipantMessage{},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := ret.(bool)
		if resTS {
			t.Errorf("unexpected value: %v", ret)
		}
	})

	t.Run("participant has messages but none that are looked for", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasMessageTypeAssigned", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "testMessage"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				Messages: []studyTypes.ParticipantMessage{
					{Type: "testMessage2", ScheduledFor: 100},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := ret.(bool)
		if resTS {
			t.Errorf("unexpected value: %v", ret)
		}
	})

	t.Run("participant has messages and one is the one looked for", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "hasMessageTypeAssigned", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "testMessage"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				Messages: []studyTypes.ParticipantMessage{
					{Type: "testMessage2", ScheduledFor: 100},
					{Type: "testMessage", ScheduledFor: 200},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := ret.(bool)
		if !resTS {
			t.Errorf("unexpected value: %v", ret)
		}
	})
}

func TestEvalGenerateRandomNumber(t *testing.T) {
	t.Run("with invalid args", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "generateRandomNumber", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "wrong"},
			{DType: "str", Str: "wrong"},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return error")
			return
		}
	})

	t.Run("with not enough args", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "generateRandomNumber", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 10},
		}}
		EvalContext := EvalContext{}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return error")
			return
		}
	})

	t.Run("with valid args", func(t *testing.T) {
		// repeat 100 times
		for i := 0; i < 100; i++ {
			exp := studyTypes.Expression{Name: "generateRandomNumber", Data: []studyTypes.ExpressionArg{
				{DType: "num", Num: 10},
				{DType: "num", Num: 20},
			}}
			EvalContext := EvalContext{}
			val, err := ExpressionEval(exp, EvalContext)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			// logger.Debug.Println(val.(float64))
			if val.(float64) < 10 || val.(float64) > 20 {
				t.Errorf("unexpected value: %v", val)
				return
			}
		}
	})
}

func TestEvalParseValueAsNum(t *testing.T) {
	testPState := studyTypes.Participant{
		Flags: map[string]string{
			"testKey": "3",
		},
	}

	t.Run("attempt to parse incorrect string", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "parseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "wrong"},
		}}
		EvalContext := EvalContext{
			ParticipantState: testPState,
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return error")
			return
		}
	})

	t.Run("attempt to parse correct string", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "parseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "15"},
		}}
		EvalContext := EvalContext{
			ParticipantState: testPState,
		}
		res, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if res != 15.0 {
			t.Errorf("unexpected value: %v", res)
			return
		}
	})

	t.Run("already a number", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "parseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: 65},
		}}
		EvalContext := EvalContext{
			ParticipantState: testPState,
		}
		res, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if res != 65.0 {
			t.Errorf("unexpected value: %v", res)
			return
		}
	})

	t.Run("expression that returns error", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "parseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{Name: "wrong", Data: []studyTypes.ExpressionArg{}}},
		}}
		EvalContext := EvalContext{
			ParticipantState: testPState,
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Errorf("should return an error: %v", err)
			return
		}
	})

	t.Run("expression that returns number", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "parseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
				{DType: "num", Num: -10},
			}}},
		}}
		EvalContext := EvalContext{
			ParticipantState: testPState,
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
	})
	t.Run("expression that returns boolean", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "parseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{Name: "gt", Data: []studyTypes.ExpressionArg{
				{DType: "num", Num: -10},
				{DType: "num", Num: 10},
			}}},
		}}
		EvalContext := EvalContext{
			ParticipantState: testPState,
		}
		_, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return an error")
			return
		}
	})

	t.Run("expression that returns string", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "parseValueAsNum", Data: []studyTypes.ExpressionArg{
			{DType: "exp", Exp: &studyTypes.Expression{Name: "getParticipantFlagValue", Data: []studyTypes.ExpressionArg{
				{DType: "str", Str: "testKey"},
			}}},
		}}
		EvalContext := EvalContext{
			ParticipantState: testPState,
		}
		res, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if res != 3.0 {
			t.Errorf("unexpected value: %v", res)
			return
		}
	})
}

func TestEvalGetMessageNextTime(t *testing.T) {
	t.Run("participant has no messages", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getMessageNextTime", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "testMessage"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return error")
			return
		}
		resTS := ret.(int64)
		if resTS != 0 {
			t.Errorf("unexpected value: %d", ret)
		}
	})

	t.Run("participant has messages but none that are looked for", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getMessageNextTime", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "testMessage"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				Messages: []studyTypes.ParticipantMessage{
					{Type: "testMessage2", ScheduledFor: 100},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err == nil {
			t.Error("should return error")
			return
		}
		resTS := ret.(int64)
		if resTS != 0 {
			t.Errorf("unexpected value: %d", ret)
		}
	})

	t.Run("participant has messages and one is the one looked for", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getMessageNextTime", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "testMessage"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				Messages: []studyTypes.ParticipantMessage{
					{Type: "testMessage2", ScheduledFor: 50},
					{Type: "testMessage", ScheduledFor: 100},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := ret.(int64)
		if resTS != 100 {
			t.Errorf("unexpected value: %d", ret)
		}
	})

	t.Run("participant has messages and two from the specified type", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getMessageNextTime", Data: []studyTypes.ExpressionArg{
			{DType: "str", Str: "testMessage"},
		}}
		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				Messages: []studyTypes.ParticipantMessage{
					{Type: "testMessage1", ScheduledFor: 100},
					{Type: "testMessage", ScheduledFor: 200},
					{Type: "testMessage", ScheduledFor: 400},
				},
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Errorf("unexpected error: %s", err.Error())
			return
		}
		resTS := ret.(int64)
		if resTS != 200 {
			t.Errorf("unexpected value: %d", ret)
		}
	})
}

func TestStudyVariableExpressions(t *testing.T) {
	// Prepare mock study DB with predefined variables
	dateVal := time.Unix(1700000000, 0)
	CurrentStudyEngine = &StudyEngine{
		studyDBService: &MockStudyDBService{
			Variables: map[string]studyTypes.StudyVariables{
				"boolVar":   {Key: "boolVar", Type: studyTypes.STUDY_VARIABLES_TYPE_BOOLEAN, Value: true},
				"intVar":    {Key: "intVar", Type: studyTypes.STUDY_VARIABLES_TYPE_INT, Value: int(42)},
				"floatVar":  {Key: "floatVar", Type: studyTypes.STUDY_VARIABLES_TYPE_FLOAT, Value: 3.5},
				"stringVar": {Key: "stringVar", Type: studyTypes.STUDY_VARIABLES_TYPE_STRING, Value: "hello"},
				"dateVar":   {Key: "dateVar", Type: studyTypes.STUDY_VARIABLES_TYPE_DATE, Value: dateVal},
			},
		},
	}

	evalCtx := EvalContext{Event: StudyEvent{InstanceID: "i1", StudyKey: "s1"}}

	t.Run("getStudyVariableBoolean", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getStudyVariableBoolean", Data: []studyTypes.ExpressionArg{{DType: "str", Str: "boolVar"}}}
		v, err := ExpressionEval(exp, evalCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vb, ok := v.(bool); !ok || vb != true {
			t.Fatalf("unexpected value: %#v", v)
		}
	})

	t.Run("getStudyVariableInt", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getStudyVariableInt", Data: []studyTypes.ExpressionArg{{DType: "str", Str: "intVar"}}}
		v, err := ExpressionEval(exp, evalCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vf, ok := v.(float64); !ok || vf != 42 {
			t.Fatalf("unexpected value: %#v", v)
		}
	})

	t.Run("getStudyVariableFloat", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getStudyVariableFloat", Data: []studyTypes.ExpressionArg{{DType: "str", Str: "floatVar"}}}
		v, err := ExpressionEval(exp, evalCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vf, ok := v.(float64); !ok || vf != 3.5 {
			t.Fatalf("unexpected value: %#v", v)
		}
	})

	t.Run("getStudyVariableString", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getStudyVariableString", Data: []studyTypes.ExpressionArg{{DType: "str", Str: "stringVar"}}}
		v, err := ExpressionEval(exp, evalCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vs, ok := v.(string); !ok || vs != "hello" {
			t.Fatalf("unexpected value: %#v", v)
		}
	})

	t.Run("getStudyVariableDate", func(t *testing.T) {
		exp := studyTypes.Expression{Name: "getStudyVariableDate", Data: []studyTypes.ExpressionArg{{DType: "str", Str: "dateVar"}}}
		v, err := ExpressionEval(exp, evalCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vf, ok := v.(float64); !ok || int64(vf) != dateVal.Unix() {
			t.Fatalf("unexpected value: %#v", v)
		}
	})
}

func TestNow(t *testing.T) {
	t.Run("testing now", func(t *testing.T) {
		cur := time.Now()
		now := Now()
		if cur.Sub(now).Abs() > time.Microsecond {
			t.Errorf("Current time is more than 1 microsecond")
		}
	})

	t.Run("testing change time", func(t *testing.T) {
		cur := time.Unix(1730419200, 0)
		Now = func() time.Time {
			return cur
		}
		now := Now()
		if cur.Sub(now).Abs() > 0 {
			t.Errorf("Current time is not the time set %s got %s", cur, now)
		}
		Now = time.Now // resetting to current time
	})

	t.Run("testing change time on timestampWithOffset", func(t *testing.T) {
		curTS := int64(1730419200)
		cur := time.Unix(curTS, 0)
		Now = func() time.Time {
			return cur
		}
		exp := studyTypes.Expression{Name: "timestampWithOffset", Data: []studyTypes.ExpressionArg{
			{DType: "num", Num: -10},
		},
		}

		EvalContext := EvalContext{
			ParticipantState: studyTypes.Participant{
				StudyStatus: studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
			},
		}
		ret, err := ExpressionEval(exp, EvalContext)
		if err != nil {
			t.Error(err)
		}
		resTS := int64(ret.(float64))
		expTS := curTS - 10
		if resTS != expTS {
			t.Errorf("Unexpected timestamp got %d, expecting %d", resTS, expTS)
		}
		Now = time.Now // resetting to current time
	})
}
