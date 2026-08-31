package surveyresponses

import (
	"log/slog"
	"slices"
	"strings"

	studydefinition "github.com/case-framework/case-backend/pkg/study/exporter/survey-definition"
	studytypes "github.com/case-framework/case-backend/pkg/study/types"
)

var defaultCtxColNames = []string{
	"language",
	"engineVersion",
	"session",
}

const (
	accountIDColumn   = "accountID"
	mainProfileColumn = "mainProfile"
)

type ResponseParser struct {
	surveyVersions    []studydefinition.SurveyVersionPreview
	surveyKey         string
	removeRootKey     bool
	columns           ColumnNames
	includeMeta       *IncludeMeta
	questionOptionSep string
	accountTracking   bool
}

func NewResponseParser(
	surveyKey string,
	surveyVersions []studydefinition.SurveyVersionPreview,
	removeRootKey bool,
	includeMeta *IncludeMeta,
	questionOptionSep string,
	extraContextColumns *[]string,
) (*ResponseParser, error) {
	rp := &ResponseParser{
		surveyKey:         surveyKey,
		surveyVersions:    surveyVersions,
		removeRootKey:     removeRootKey,
		includeMeta:       includeMeta,
		questionOptionSep: questionOptionSep,
	}

	if err := rp.initColumnNames(extraContextColumns); err != nil {
		return nil, err
	}

	return rp, nil
}

func (rp *ResponseParser) initColumnNames(extraContextColumns *[]string) error {
	fixedCols := []string{
		"ID",
		"participantID",
		"version",
		"opened",
		"submitted",
		"arrived",
	}

	ctxCols := append([]string{}, defaultCtxColNames...)
	if extraContextColumns != nil {
		ctxCols = append(ctxCols, *extraContextColumns...)
	}
	ctxCols = slices.DeleteFunc(ctxCols, func(column string) bool {
		return column == accountIDColumn || column == mainProfileColumn
	})

	if rp.removeRootKey {
		for versionInd, sv := range rp.surveyVersions {
			for qInd, question := range sv.Questions {
				rp.surveyVersions[versionInd].Questions[qInd].ID = strings.TrimPrefix(question.ID, rp.surveyKey+".")
			}
		}
	}

	respCols := getResponseColNamesForAllVersions(rp.surveyVersions, rp.questionOptionSep)
	slices.Sort(respCols)

	metaCols := getMetaColNamesForAllVersions(rp.surveyVersions, rp.includeMeta, rp.questionOptionSep)
	slices.Sort(metaCols)

	rp.columns = ColumnNames{
		FixedColumns:    fixedCols,
		ContextColumns:  ctxCols,
		ResponseColumns: respCols,
		MetaColumns:     metaCols,
	}
	return nil
}

// EnableAccountTracking adds the account tracking fields to all output
// formats. The account ID value is the pseudonymized value from the
// participant state.
func (rp *ResponseParser) EnableAccountTracking() {
	if rp.accountTracking {
		return
	}

	rp.accountTracking = true
	rp.columns.FixedColumns = append(rp.columns.FixedColumns, accountIDColumn, mainProfileColumn)
}

func (rp *ResponseParser) ParseResponse(
	rawResp *studytypes.SurveyResponse,
	trackingInfo ...AccountTrackingInfo,
) (ParsedResponse, error) {
	parsedResponse := ParsedResponse{
		ID:            rawResp.ID.Hex(),
		ParticipantID: rawResp.ParticipantID,
		Version:       rawResp.VersionID,
		OpenedAt:      rawResp.OpenedAt,
		SubmittedAt:   rawResp.SubmittedAt,
		ArrivedAt:     rawResp.ArrivedAt,
		Context:       rawResp.Context,
		Responses:     map[string]interface{}{},
		Meta: ResponseMeta{
			Initialised: map[string][]int64{},
			Displayed:   map[string][]int64{},
			Responded:   map[string][]int64{},
			Position:    map[string]int32{},
		},
	}
	if len(trackingInfo) > 0 {
		parsedResponse.AccountID = trackingInfo[0].AccountID
		parsedResponse.MainProfile = trackingInfo[0].MainProfile
	}

	currentVersion, err := findSurveyVersion(rawResp.VersionID, rawResp.ArrivedAt, rp.surveyVersions)
	if err != nil {
		return parsedResponse, err
	}
	if currentVersion.VersionID != rawResp.VersionID && currentVersion.VersionID != "" {
		parsedResponse.Version = rawResp.VersionID + " (" + currentVersion.VersionID + ")"
		if rawResp.VersionID == "" {
			parsedResponse.Version = currentVersion.VersionID
			slog.Debug("VersionID of used survey is empty, only mapped versionID is displayed.")
		}
	}

	if rp.removeRootKey {
		for i, r := range rawResp.Responses {
			rawResp.Responses[i].Key = strings.TrimPrefix(r.Key, rp.surveyKey+".")
		}
	}

	for _, question := range currentVersion.Questions {
		resp := findResponse(rawResp.Responses, question.ID)

		responseColumns := getResponseColumns(question, resp, rp.questionOptionSep)
		for k, v := range responseColumns {
			_, hasKey := parsedResponse.Responses[k]
			if hasKey {
				slog.Error("response with unknown key", slog.String("key", k))
				continue
			}
			parsedResponse.Responses[k] = v
		}

		// Set meta infos
		initColName := question.ID + rp.questionOptionSep + "metaInit"
		parsedResponse.Meta.Initialised[initColName] = []int64{}

		dispColName := question.ID + rp.questionOptionSep + "metaDisplayed"
		parsedResponse.Meta.Displayed[dispColName] = []int64{}

		respColName := question.ID + rp.questionOptionSep + "metaResponse"
		parsedResponse.Meta.Responded[respColName] = []int64{}

		positionColName := question.ID + rp.questionOptionSep + "metaPosition"
		parsedResponse.Meta.Position[positionColName] = 0

		if resp != nil {
			if resp.Meta.Rendered != nil {
				parsedResponse.Meta.Initialised[initColName] = resp.Meta.Rendered
			}
			if resp.Meta.Displayed != nil {
				parsedResponse.Meta.Displayed[dispColName] = resp.Meta.Displayed
			}
			if resp.Meta.Responded != nil {
				parsedResponse.Meta.Responded[respColName] = resp.Meta.Responded
			}
			parsedResponse.Meta.Position[positionColName] = resp.Meta.Position
		}
	}

	return parsedResponse, nil
}

func (rp *ResponseParser) ResponseToStrList(
	parsedResponse ParsedResponse,
) ([]string, error) {
	result, err := rp.ResponseToFlatObj(parsedResponse)
	if err != nil {
		return nil, err
	}

	out := []string{}

	// add fixed columns
	for _, colName := range rp.columns.FixedColumns {
		out = append(out, valueToStr(result[colName]))
	}

	// add context columns
	for _, colName := range rp.columns.ContextColumns {
		out = append(out, valueToStr(result[colName]))
	}

	// add response item columns
	for _, colName := range rp.columns.ResponseColumns {
		str := valueToStr(result[colName])
		str = replaceNewlines(str)
		out = append(out, str)
	}

	// add meta columns
	for _, colName := range rp.columns.MetaColumns {
		out = append(out, valueToStr(result[colName]))
	}

	return out, nil
}

func (rp *ResponseParser) ResponseToLongFormat(
	parsedResponse ParsedResponse,
) ([][]string, error) {
	result, err := rp.ResponseToFlatObj(parsedResponse)
	if err != nil {
		return nil, err
	}

	out := [][]string{}

	fixedValues := []string{}
	for _, colName := range rp.columns.FixedColumns {
		fixedValues = append(fixedValues, valueToStr(result[colName]))
	}

	for _, colName := range rp.columns.ContextColumns {
		fixedValues = append(fixedValues, valueToStr(result[colName]))
	}

	for _, colName := range rp.columns.ResponseColumns {
		currentRespLine := []string{}
		currentRespLine = append(currentRespLine, fixedValues...)
		currentRespLine = append(currentRespLine, colName)
		str := valueToStr(result[colName])
		str = replaceNewlines(str)
		currentRespLine = append(currentRespLine, str)
		out = append(out, currentRespLine)
	}

	for _, colName := range rp.columns.MetaColumns {
		currentRespLine := []string{}
		currentRespLine = append(currentRespLine, fixedValues...)
		currentRespLine = append(currentRespLine, colName)
		currentRespLine = append(currentRespLine, valueToStr(result[colName]))
		out = append(out, currentRespLine)
	}

	return out, nil
}

func (rp *ResponseParser) ResponseToFlatObj(
	parsedResponse ParsedResponse,
) (map[string]interface{}, error) {

	result := rp.initWithFixedColumnsWithValues(&parsedResponse)
	result = rp.addContextColumnsWithValues(&parsedResponse, result)
	result = rp.addResponseItemColumnsWithValues(&parsedResponse, result)
	result = rp.addMetaColumnsWithValues(&parsedResponse, result)

	return result, nil
}

func (rp ResponseParser) initWithFixedColumnsWithValues(
	parsedResponse *ParsedResponse,
) map[string]interface{} {
	result := map[string]interface{}{
		rp.columns.FixedColumns[0]: parsedResponse.ID,
		rp.columns.FixedColumns[1]: parsedResponse.ParticipantID,
		rp.columns.FixedColumns[2]: parsedResponse.Version,
		rp.columns.FixedColumns[3]: parsedResponse.OpenedAt,
		rp.columns.FixedColumns[4]: parsedResponse.SubmittedAt,
		rp.columns.FixedColumns[5]: parsedResponse.ArrivedAt,
	}
	if rp.accountTracking {
		result[accountIDColumn] = parsedResponse.AccountID
		if parsedResponse.MainProfile != nil {
			result[mainProfileColumn] = *parsedResponse.MainProfile
		} else {
			result[mainProfileColumn] = ""
		}
	}
	return result
}

func (rp ResponseParser) addContextColumnsWithValues(
	parsedResponse *ParsedResponse,
	res map[string]interface{},
) map[string]interface{} {
	for _, colName := range rp.columns.ContextColumns {
		v, ok := parsedResponse.Context[colName]
		if !ok {
			res[colName] = ""
		} else {
			res[colName] = v
		}
	}
	return res
}

func (rp ResponseParser) addResponseItemColumnsWithValues(
	parsedResponse *ParsedResponse,
	res map[string]interface{},
) map[string]interface{} {
	for _, colName := range rp.columns.ResponseColumns {
		r, ok := parsedResponse.Responses[colName]
		if !ok {
			res[colName] = ""
		} else {
			res[colName] = r
		}
	}
	return res
}

func (rp ResponseParser) addMetaColumnsWithValues(
	parsedResponse *ParsedResponse,
	res map[string]interface{},
) map[string]interface{} {
	if rp.includeMeta == nil {
		return res
	}

	for _, colName := range rp.columns.MetaColumns {
		if strings.Contains(colName, "metaInit") {
			v, ok := parsedResponse.Meta.Initialised[colName]
			if !ok {
				res[colName] = ""
			} else {
				res[colName] = v
			}
		} else if strings.Contains(colName, "metaDisplayed") {
			v, ok := parsedResponse.Meta.Displayed[colName]
			if !ok {
				res[colName] = ""
			} else {
				res[colName] = v
			}
		} else if strings.Contains(colName, "metaResponse") {
			v, ok := parsedResponse.Meta.Responded[colName]
			if !ok {
				res[colName] = ""
			} else {
				res[colName] = v
			}
		} else if strings.Contains(colName, "metaPosition") {
			v, ok := parsedResponse.Meta.Position[colName]
			if !ok {
				res[colName] = ""
			} else {
				res[colName] = v
			}
		}
	}
	return res
}
