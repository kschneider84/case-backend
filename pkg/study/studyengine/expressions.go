package studyengine

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/case-framework/case-backend/pkg/apihelpers"
	httpclient "github.com/case-framework/case-backend/pkg/http-client"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	"go.mongodb.org/mongo-driver/bson"
)

func ExpressionEval(expression studyTypes.Expression, evalCtx EvalContext) (val interface{}, err error) {
	switch expression.Name {
	case "checkEventType":
		val, err = evalCtx.checkEventType(expression)
	case "checkEventKey":
		val, err = evalCtx.checkEventKey(expression)
	// Response checkers:
	case "checkSurveyResponseKey":
		val, err = evalCtx.checkSurveyResponseKey(expression)
	case "responseHasKeysAny":
		val, err = evalCtx.responseHasKeysAny(expression)
	case "responseHasOnlyKeysOtherThan":
		val, err = evalCtx.responseHasOnlyKeysOtherThan(expression)
	case "getResponseValueAsNum":
		val, err = evalCtx.getResponseValueAsNum(expression)
	case "getResponseValueAsStr":
		val, err = evalCtx.getResponseValueAsStr(expression)
	case "getSelectedKeys":
		val, err = evalCtx.getSelectedKeys(expression)
	case "countResponseItems":
		val, err = evalCtx.countResponseItems(expression)
	case "hasResponseKey":
		val, err = evalCtx.hasResponseKey(expression)
	case "hasResponseKeyWithValue":
		val, err = evalCtx.hasResponseKeyWithValue(expression)
	// Old responses:
	case "checkConditionForOldResponses":
		val, err = evalCtx.checkConditionForOldResponses(expression)
	// Study code lists:
	case "isStudyCodePresent":
		val, err = evalCtx.isStudyCodePresent(expression)
	// Study counters:
	case "getCurrentStudyCounterValue":
		val, err = evalCtx.getCurrentStudyCounterValue(expression)
	case "getNextStudyCounterValue":
		val, err = evalCtx.getNextStudyCounterValue(expression)
	// Study variables:
	case "getStudyVariableBoolean":
		val, err = evalCtx.getStudyVariableBoolean(expression)
	case "getStudyVariableInt":
		val, err = evalCtx.getStudyVariableInt(expression)
	case "getStudyVariableFloat":
		val, err = evalCtx.getStudyVariableFloat(expression)
	case "getStudyVariableString":
		val, err = evalCtx.getStudyVariableString(expression)
	case "getStudyVariableDate":
		val, err = evalCtx.getStudyVariableDate(expression)
	// Access event payload:
	case "hasEventPayload":
		val, err = evalCtx.hasEventPayload()
	case "getEventPayloadValueAsStr":
		val, err = evalCtx.getEventPayloadValueAsStr(expression)
	case "getEventPayloadValueAsNum":
		val, err = evalCtx.getEventPayloadValueAsNum(expression)
	case "hasEventPayloadKey":
		val, err = evalCtx.hasEventPayloadKey(expression)
	case "hasEventPayloadKeyWithValue":
		val, err = evalCtx.hasEventPayloadKeyWithValue(expression)
	// Participant state:
	case "getStudyEntryTime":
		val, err = evalCtx.getStudyEntryTime(false)
	case "hasSurveyKeyAssigned":
		val, err = evalCtx.hasSurveyKeyAssigned(expression, false)
	case "getSurveyKeyAssignedFrom":
		val, err = evalCtx.getSurveyKeyAssignedFrom(expression, false)
	case "getSurveyKeyAssignedUntil":
		val, err = evalCtx.getSurveyKeyAssignedUntil(expression, false)
	case "hasStudyStatus":
		val, err = evalCtx.hasStudyStatus(expression, false)
	case "hasParticipantFlag":
		val, err = evalCtx.hasParticipantFlag(expression, false)
	case "hasParticipantFlagKey":
		val, err = evalCtx.hasParticipantFlagKey(expression, false)
	case "getParticipantFlagValue":
		val, err = evalCtx.getParticipantFlagValue(expression, false)
	case "hasLinkingCode":
		val, err = evalCtx.hasLinkingCode(expression, false)
	case "getLinkingCodeValue":
		val, err = evalCtx.getLinkingCode(expression, false)
	case "getLastSubmissionDate":
		val, err = evalCtx.getLastSubmissionDate(expression, false)
	case "lastSubmissionDateOlderThan":
		val, err = evalCtx.lastSubmissionDateOlderThan(expression, false)
	case "hasMessageTypeAssigned":
		val, err = evalCtx.hasMessageTypeAssigned(expression, false)
	case "getMessageNextTime":
		val, err = evalCtx.getMessageNextTime(expression, false)
	case "getCurrentStudySession":
		val, err = evalCtx.getCurrentStudySession(false)
	// exprssions for merge participant states:
	case "incomingState:getStudyEntryTime":
		val, err = evalCtx.getStudyEntryTime(true)
	case "incomingState:hasSurveyKeyAssigned":
		val, err = evalCtx.hasSurveyKeyAssigned(expression, true)
	case "incomingState:getSurveyKeyAssignedFrom":
		val, err = evalCtx.getSurveyKeyAssignedFrom(expression, true)
	case "incomingState:getSurveyKeyAssignedUntil":
		val, err = evalCtx.getSurveyKeyAssignedUntil(expression, true)
	case "incomingState:hasStudyStatus":
		val, err = evalCtx.hasStudyStatus(expression, true)
	case "incomingState:hasParticipantFlag":
		val, err = evalCtx.hasParticipantFlag(expression, true)
	case "incomingState:hasParticipantFlagKey":
		val, err = evalCtx.hasParticipantFlagKey(expression, true)
	case "incomingState:getParticipantFlagValue":
		val, err = evalCtx.getParticipantFlagValue(expression, true)
	case "incomingState:hasLinkingCode":
		val, err = evalCtx.hasLinkingCode(expression, true)
	case "incomingState:getLinkingCodeValue":
		val, err = evalCtx.getLinkingCode(expression, true)
	case "incomingState:getLastSubmissionDate":
		val, err = evalCtx.getLastSubmissionDate(expression, true)
	case "incomingState:lastSubmissionDateOlderThan":
		val, err = evalCtx.lastSubmissionDateOlderThan(expression, true)
	case "incomingState:hasMessageTypeAssigned":
		val, err = evalCtx.hasMessageTypeAssigned(expression, true)
	case "incomingState:getMessageNextTime":
		val, err = evalCtx.getMessageNextTime(expression, true)
	case "incomingState:getCurrentStudySession":
		val, err = evalCtx.getCurrentStudySession(true)
	// Logical and comparisions:
	case "eq":
		val, err = evalCtx.eq(expression)
	case "lt":
		val, err = evalCtx.lt(expression)
	case "lte":
		val, err = evalCtx.lte(expression)
	case "gt":
		val, err = evalCtx.gt(expression)
	case "gte":
		val, err = evalCtx.gte(expression)
	case "and":
		val, err = evalCtx.and(expression)
	case "or":
		val, err = evalCtx.or(expression)
	case "not":
		val, err = evalCtx.not(expression)
	// Math functions
	case "sum":
		val, err = evalCtx.sum(expression)
	case "neg":
		val, err = evalCtx.neg(expression)
	// Other
	case "timestampWithOffset":
		val, err = evalCtx.timestampWithOffset(expression)
	case "timestampDiff":
		val, err = evalCtx.timestampDiff(expression)
	case "getTsForNextStartOfMonth":
		val, err = evalCtx.getTsForNextStartOfMonth(expression)
	case "getISOWeekForTs":
		val, err = evalCtx.getISOWeekForTs(expression)
	case "getTsForNextISOWeek":
		val, err = evalCtx.getTsForNextISOWeek(expression)
	case "getTsForStartOfISOWeek":
		val, err = evalCtx.getTsForStartOfISOWeek(expression)
	case "dateToStr":
		val, err = evalCtx.dateToStr(expression)
	case "parseValueAsNum":
		val, err = evalCtx.parseValueAsNum(expression)
	case "generateRandomNumber":
		val, err = evalCtx.generateRandomNumber(expression)
	case "externalEventEval":
		val, err = evalCtx.externalEventEval(expression)
	default:
		err = fmt.Errorf("expression name not known: %s", expression.Name)
		slog.Debug("unexpected error during expression eval", slog.String("error", err.Error()))
		return
	}
	return
}

func (ctx EvalContext) ExpressionArgResolver(arg studyTypes.ExpressionArg) (interface{}, error) {
	switch arg.DType {
	case "num":
		return arg.Num, nil
	case "exp":
		if arg.Exp == nil {
			return nil, errors.New("missing argument - expected expression, but was empty")
		}
		return ExpressionEval(*arg.Exp, ctx)
	case "str":
		return arg.Str, nil
	default:
		return arg.Str, nil
	}
}

// checkEventType compares the eventType with a string
func (ctx EvalContext) checkEventType(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	return ctx.Event.Type == arg1Val, nil
}

// checkEventKey compares the event key with a string
func (ctx EvalContext) checkEventKey(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	return ctx.Event.EventKey == arg1Val, nil
}

// checkSurveyResponseKey compares the key of the submitted survey response (if any)
func (ctx EvalContext) checkSurveyResponseKey(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	return ctx.Event.Response.Key == arg1Val, nil
}

func (ctx EvalContext) hasStudyStatus(exp studyTypes.Expression, withIncomingParticipantState bool) (val bool, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1Val, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	return pState.StudyStatus == arg1Val, nil
}

func (ctx EvalContext) isStudyCodePresent(exp studyTypes.Expression) (val bool, err error) {
	if CurrentStudyEngine == nil || CurrentStudyEngine.studyDBService == nil {
		return val, errors.New("studyCodeExists: DB connection not available in the context")
	}

	if len(exp.Data) != 2 {
		return val, errors.New("studyCodeExists: invalid number of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	listKey, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}
	code, ok := arg2.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	exists, err := CurrentStudyEngine.studyDBService.StudyCodeListEntryExists(ctx.Event.InstanceID, ctx.Event.StudyKey, listKey, code)
	if err != nil {
		exists = false
	}
	return exists, nil
}

func (ctx EvalContext) getCurrentStudyCounterValue(exp studyTypes.Expression) (val float64, err error) {
	if CurrentStudyEngine == nil || CurrentStudyEngine.studyDBService == nil {
		return val, errors.New("getCurrentStudyCounterValue: DB connection not available in the context")
	}

	if len(exp.Data) != 1 {
		return val, errors.New("getCurrentStudyCounterValue: invalid number of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	scope, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}
	value, err := CurrentStudyEngine.studyDBService.GetCurrentStudyCounterValue(ctx.Event.InstanceID, ctx.Event.StudyKey, scope)
	if err != nil {
		return val, err
	}
	return float64(value), nil
}

func (ctx EvalContext) getNextStudyCounterValue(exp studyTypes.Expression) (val float64, err error) {
	if CurrentStudyEngine == nil || CurrentStudyEngine.studyDBService == nil {
		return val, errors.New("getNextStudyCounterValue: DB connection not available in the context")
	}

	if len(exp.Data) != 1 {
		return val, errors.New("getNextStudyCounterValue: invalid number of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	scope, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}
	value, err := CurrentStudyEngine.studyDBService.IncrementAndGetStudyCounterValue(ctx.Event.InstanceID, ctx.Event.StudyKey, scope)
	if err != nil {
		return val, err
	}
	return float64(value), nil
}

func (ctx EvalContext) getStudyVariable(exp studyTypes.Expression, asType studyTypes.StudyVariablesType) (val studyTypes.StudyVariables, err error) {
	if CurrentStudyEngine == nil || CurrentStudyEngine.studyDBService == nil {
		return val, errors.New("getStudyVariable: DB connection not available in the context")
	}

	if len(exp.Data) != 1 {
		return val, errors.New("getStudyVariable: invalid number of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	key, ok := arg1.(string)
	if !ok {
		return val, errors.New("getStudyVariable: could not cast arguments")
	}

	val, err = CurrentStudyEngine.studyDBService.GetStudyVariableByStudyKeyAndKey(ctx.Event.InstanceID, ctx.Event.StudyKey, key, true)
	if err != nil {
		return val, err
	}
	if val.Type != asType {
		return val, fmt.Errorf("getStudyVariable: wrong type, expected %s, got %s", asType, val.Type)
	}
	return
}

func (ctx EvalContext) getStudyVariableBoolean(exp studyTypes.Expression) (val bool, err error) {
	variable, err := ctx.getStudyVariable(exp, studyTypes.STUDY_VARIABLES_TYPE_BOOLEAN)
	if err != nil {
		return val, err
	}
	bVal, ok := variable.Value.(bool)
	if !ok {
		return val, errors.New("getStudyVariableBoolean: could not cast arguments")
	}
	return bVal, nil
}

func (ctx EvalContext) getStudyVariableInt(exp studyTypes.Expression) (val float64, err error) {
	variable, err := ctx.getStudyVariable(exp, studyTypes.STUDY_VARIABLES_TYPE_INT)
	if err != nil {
		return val, err
	}
	switch v := variable.Value.(type) {
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		// Be tolerant if stored as float
		return v, nil
	default:
		return val, errors.New("getStudyVariableInt: could not cast arguments")
	}
}

func (ctx EvalContext) getStudyVariableFloat(exp studyTypes.Expression) (val float64, err error) {
	variable, err := ctx.getStudyVariable(exp, studyTypes.STUDY_VARIABLES_TYPE_FLOAT)
	if err != nil {
		return val, err
	}
	floatVal, ok := variable.Value.(float64)
	if !ok {
		return val, errors.New("getStudyVariableFloat: could not cast arguments")
	}
	return floatVal, nil
}

func (ctx EvalContext) getStudyVariableString(exp studyTypes.Expression) (val string, err error) {
	variable, err := ctx.getStudyVariable(exp, studyTypes.STUDY_VARIABLES_TYPE_STRING)
	if err != nil {
		return val, err
	}
	sVal, ok := variable.Value.(string)
	if !ok {
		return val, errors.New("getStudyVariableString: could not cast arguments")
	}
	return sVal, nil
}

func (ctx EvalContext) getStudyVariableDate(exp studyTypes.Expression) (val float64, err error) {
	variable, err := ctx.getStudyVariable(exp, studyTypes.STUDY_VARIABLES_TYPE_DATE)
	if err != nil {
		return val, err
	}
	timeVal, ok := variable.Value.(time.Time)
	if !ok {
		return val, errors.New("getStudyVariableDate: could not cast arguments")
	}
	return float64(timeVal.Unix()), nil
}

func (ctx EvalContext) checkConditionForOldResponses(exp studyTypes.Expression) (val bool, err error) {
	if CurrentStudyEngine == nil || CurrentStudyEngine.studyDBService == nil {
		return val, errors.New("checkConditionForOldResponses: DB connection not available in the context")
	}
	if ctx.Event.InstanceID == "" || ctx.Event.StudyKey == "" {
		return val, errors.New("checkConditionForOldResponses: instanceID or study key missing from context")
	}

	argNum := len(exp.Data)
	if argNum < 1 || argNum > 5 {
		return val, fmt.Errorf("checkConditionForOldResponses: unexpected numbers of arguments: %d", len(exp.Data))
	}

	arg1 := exp.Data[0]
	if !arg1.IsExpression() {
		return val, errors.New("checkConditionForOldResponses: first argument must be an expression")
	}
	condition := arg1.Exp
	if condition == nil {
		return val, errors.New("checkConditionForOldResponses: first argument must be an expression")
	}

	checkFor := "all"
	checkForCount := 1
	surveyKey := ""
	since := int64(0)
	until := int64(0)
	if argNum > 1 {
		arg1, err := ctx.ExpressionArgResolver(exp.Data[1])
		if err != nil {
			return val, err
		}
		switch arg1Val := arg1.(type) {
		case string:
			checkFor = arg1Val
		case float64:
			checkFor = "count"
			checkForCount = int(arg1Val)
		default:
			return val, fmt.Errorf("type unknown %T", arg1Val)
		}
	}
	if argNum > 2 {
		surveyKey, err = ctx.mustGetStrValue(exp.Data[2])
		if err != nil {
			return val, err
		}
	}
	if argNum > 3 {
		arg4, err := ctx.ExpressionArgResolver(exp.Data[3])
		if err != nil {
			return val, err
		}
		arg4Val, ok := arg4.(float64)
		if ok {
			since = int64(arg4Val)
		}

	}
	if argNum > 4 {
		arg5, err := ctx.ExpressionArgResolver(exp.Data[4])
		if err != nil {
			return val, err
		}
		arg5Val, ok := arg5.(float64)
		if ok {
			until = int64(arg5Val)
		}
	}

	filter := bson.M{
		"participantID": ctx.ParticipantState.ParticipantID,
	}
	if surveyKey != "" {
		filter["key"] = surveyKey
	}
	if since > 0 && until > 0 {
		filter["$and"] = bson.A{
			bson.M{"arrivedAt": bson.M{"$gt": since}},
			bson.M{"arrivedAt": bson.M{"$lt": until}},
		}
	} else if since > 0 {
		filter["arrivedAt"] = bson.M{"$gt": since}
	} else if until > 0 {
		filter["arrivedAt"] = bson.M{"$lt": until}
	}

	responses, _, err := CurrentStudyEngine.studyDBService.GetResponses(
		ctx.Event.InstanceID,
		ctx.Event.StudyKey,
		filter,
		bson.M{
			"arrivedAt": -1,
		},
		1,
		100,
	)

	if err != nil {
		return val, err
	}

	counter := 0
	result := false

	for _, resp := range responses {
		oldEvalContext := EvalContext{
			ParticipantState: ctx.ParticipantState,
			Event: StudyEvent{
				Response: resp,
			},
		}

		expResult, err := ExpressionEval(*condition, oldEvalContext)
		if err != nil {
			return false, err
		}
		val := expResult.(bool)

		switch checkFor {
		case "all":
			if val {
				result = true
			} else {
				result = false
				return result, nil
			}
		case "any":
			if val {
				result = true
				return result, nil
			}
		case "count":
			if val {
				counter += 1
				if counter >= checkForCount {
					result = true
					return result, nil
				}
			}
		}
	}

	return result, nil
}

func (ctx EvalContext) hasEventPayload() (val bool, err error) {
	return len(ctx.Event.Payload) > 0, nil
}

func (ctx EvalContext) getEventPayloadValueAsStr(exp studyTypes.Expression) (val string, err error) {
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1Val, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	mV, ok := ctx.Event.Payload[arg1Val]
	if !ok {
		return "", nil
	}
	val, ok = mV.(string)
	if !ok {
		slog.Debug("could not cast value to string", slog.String("value", fmt.Sprintf("%v", mV)))
		return "", nil
	}
	return val, nil
}

func (ctx EvalContext) getEventPayloadValueAsNum(exp studyTypes.Expression) (val float64, err error) {
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1Val, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	mV, ok := ctx.Event.Payload[arg1Val]
	if !ok {
		return 0, nil
	}
	val, ok = mV.(float64)
	if !ok {
		slog.Debug("could not cast value to number", slog.String("value", fmt.Sprintf("%v", mV)))
		return 0, nil
	}

	return val, nil
}

func (ctx EvalContext) hasEventPayloadKey(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1Val, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	_, ok := ctx.Event.Payload[arg1Val]
	return ok, nil
}

func (ctx EvalContext) hasEventPayloadKeyWithValue(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1Val, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	arg2Val, err := ctx.mustGetStrValue(exp.Data[1])
	if err != nil {
		return val, err
	}

	value, ok := ctx.Event.Payload[arg1Val]
	if !ok {
		return false, nil
	}

	if value == arg2Val {
		return true, nil
	}

	return false, nil
}

func (ctx EvalContext) getStudyEntryTime(withIncomingParticipantState bool) (t float64, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	return float64(pState.EnteredAt), nil
}

func (ctx EvalContext) hasSurveyKeyAssigned(exp studyTypes.Expression, withIncomingParticipantState bool) (val bool, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}

	if len(exp.Data) != 1 || !exp.Data[0].IsString() {
		return val, errors.New("unexpected number or wrong type of argument")
	}
	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast argument")
	}

	for _, survey := range pState.AssignedSurveys {
		if survey.SurveyKey == arg1Val {
			val = true
			return
		}
	}
	return
}

func (ctx EvalContext) getSurveyKeyAssignedFrom(exp studyTypes.Expression, withIncomingParticipantState bool) (val float64, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}

	if len(exp.Data) != 1 || !exp.Data[0].IsString() {
		return val, errors.New("unexpected number or wrong type of argument")
	}

	arg1Val, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	for _, survey := range pState.AssignedSurveys {
		if survey.SurveyKey == arg1Val {
			val = float64(survey.ValidFrom)
			return
		}
	}

	return -1, nil
}

func (ctx EvalContext) getSurveyKeyAssignedUntil(exp studyTypes.Expression, withIncomingParticipantState bool) (val float64, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}

	if len(exp.Data) != 1 || !exp.Data[0].IsString() {
		return val, errors.New("unexpected number or wrong type of argument")
	}
	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast argument")
	}

	for _, survey := range pState.AssignedSurveys {
		if survey.SurveyKey == arg1Val {
			val = float64(survey.ValidUntil)
			return
		}
	}

	return -1, nil
}

func (ctx EvalContext) hasParticipantFlagKey(exp studyTypes.Expression, withIncomingParticipantState bool) (val bool, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	if exp.Data[0].IsNumber() {
		return val, errors.New("unexpected argument types")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast argument 1")
	}

	_, ok = pState.Flags[arg1Val]
	if !ok {
		return false, nil
	}
	return true, nil
}

func (ctx EvalContext) getParticipantFlagValue(exp studyTypes.Expression, withIncomingParticipantState bool) (val string, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	if exp.Data[0].IsNumber() {
		return val, errors.New("unexpected argument types")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast argument 1")
	}

	res, ok := pState.Flags[arg1Val]
	if !ok {
		return "", nil
	}
	return res, nil
}

func (ctx EvalContext) hasParticipantFlag(exp studyTypes.Expression, withIncomingParticipantState bool) (val bool, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) != 2 {
		return val, errors.New("unexpected numbers of arguments")
	}

	if exp.Data[0].IsNumber() || exp.Data[1].IsNumber() {
		return val, errors.New("unexpected argument types")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast argument 1")
	}

	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}
	arg2Val, ok := arg2.(string)
	if !ok {
		return val, errors.New("could not cast argument 2")
	}

	value, ok := pState.Flags[arg1Val]
	if !ok || value != arg2Val {
		return false, nil
	}
	return true, nil
}

func (ctx EvalContext) hasLinkingCode(exp studyTypes.Expression, withIncomingParticipantState bool) (val bool, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	if exp.Data[0].IsNumber() {
		return val, errors.New("unexpected argument types")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast argument 1")
	}

	_, ok = pState.LinkingCodes[arg1Val]
	if !ok {
		return false, nil
	}
	return true, nil
}

func (ctx EvalContext) getLinkingCode(exp studyTypes.Expression, withIncomingParticipantState bool) (val string, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) != 1 {
		return val, errors.New("unexpected numbers of arguments")
	}

	if exp.Data[0].IsNumber() {
		return val, errors.New("unexpected argument types")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast argument 1")
	}

	res, ok := pState.LinkingCodes[arg1Val]
	if !ok {
		return "", nil
	}
	return res, nil
}

func (ctx EvalContext) getLastSubmissionDate(exp studyTypes.Expression, withIncomingParticipantState bool) (val float64, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) < 1 {
		// if no arguments are provided, return the last submission date of the participant
		maxTs := int64(0)
		for _, lastTs := range pState.LastSubmissions {
			if lastTs > maxTs {
				maxTs = lastTs
			}
		}

		return float64(maxTs), nil
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	surveyKey := arg1.(string)

	lastSubmissionDate, ok := pState.LastSubmissions[surveyKey]
	if !ok {
		return 0, nil
	}

	return float64(lastSubmissionDate), nil
}

func (ctx EvalContext) lastSubmissionDateOlderThan(exp studyTypes.Expression, withIncomingParticipantState bool) (val bool, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) != 1 && len(exp.Data) != 2 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(float64)
	if !ok {
		return val, errors.New("could not cast argument 1")
	}
	refTime := int64(arg1Val)

	if len(exp.Data) == 2 {
		arg2Val, err := ctx.mustGetStrValue(exp.Data[1])
		if err != nil {
			return val, err
		}
		lastTs, ok := pState.LastSubmissions[arg2Val]
		if !ok {
			return false, nil
		}
		return lastTs < refTime, nil

	} else {
		for _, lastTs := range pState.LastSubmissions {
			if lastTs > refTime {
				return false, nil
			}
		}
	}
	return true, nil
}

func (ctx EvalContext) responseHasKeysAny(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) < 3 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}
	arg2Val, ok := arg2.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	targetKeys := []string{}
	for _, d := range exp.Data[2:] {
		arg, err := ctx.ExpressionArgResolver(d)
		if err != nil {
			return val, err
		}
		argVal, ok := arg.(string)
		if !ok {
			return val, errors.New("could not cast arguments")
		}
		targetKeys = append(targetKeys, argVal)
	}

	// find survey item:
	responseOfInterest, err := findSurveyItemResponse(ctx.Event.Response.Responses, arg1Val)
	if err != nil {
		// Item not found
		return false, nil
	}

	responseParentGroup, err := findResponseObject(responseOfInterest, arg2Val)
	if err != nil {
		// Item not found
		return false, nil
	}

	// Check if any of the target in response
	anyFound := false
	for _, target := range targetKeys {
		for _, item := range responseParentGroup.Items {
			if item.Key == target {
				anyFound = true
				break
			}
		}
		if anyFound {
			break
		}
	}
	return anyFound, nil
}

func (ctx EvalContext) responseHasOnlyKeysOtherThan(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) < 3 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}
	arg2Val, ok := arg2.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	targetKeys := []string{}
	for _, d := range exp.Data[2:] {
		arg, err := ctx.ExpressionArgResolver(d)
		if err != nil {
			return val, err
		}
		argVal, ok := arg.(string)
		if !ok {
			return val, errors.New("could not cast arguments")
		}
		targetKeys = append(targetKeys, argVal)
	}

	// find survey item:
	responseOfInterest, err := findSurveyItemResponse(ctx.Event.Response.Responses, arg1Val)
	if err != nil {
		// Item not found
		return false, nil
	}

	responseParentGroup, err := findResponseObject(responseOfInterest, arg2Val)
	if err != nil {
		// Item not found
		return false, nil
	}

	if len(responseParentGroup.Items) < 1 {
		return false, nil
	}

	// Check if any of the target in response
	anyFound := true
	for _, target := range targetKeys {
		for _, item := range responseParentGroup.Items {
			if item.Key == target {
				anyFound = false
				break
			}
		}
		if anyFound {
			break
		}
	}
	return anyFound, nil
}

func (ctx EvalContext) getResponseValueAsNum(exp studyTypes.Expression) (val float64, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}
	arg2Val, ok := arg2.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	// find survey item:
	surveyItem, err := findSurveyItemResponse(ctx.Event.Response.Responses, arg1Val)
	if err != nil {
		// Item not found
		return 0, errors.New("item not found")
	}

	responseObject, err := findResponseObject(surveyItem, arg2Val)
	if err != nil {
		// Item not found
		return 0, errors.New("item not found")
	}

	val, err = strconv.ParseFloat(responseObject.Value, 64)
	return
}

func (ctx EvalContext) getResponseValueAsStr(exp studyTypes.Expression) (val string, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}
	arg2Val, ok := arg2.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	// find survey item:
	surveyItem, err := findSurveyItemResponse(ctx.Event.Response.Responses, arg1Val)
	if err != nil {
		// Item not found
		return "", errors.New("item not found")
	}

	responseObject, err := findResponseObject(surveyItem, arg2Val)
	if err != nil {
		// Item not found
		return "", errors.New("item not found")
	}
	val = responseObject.Value
	return
}

func (ctx EvalContext) getSelectedKeys(exp studyTypes.Expression) (val string, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("unexpected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Val, ok := arg1.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}
	arg2Val, ok := arg2.(string)
	if !ok {
		return val, errors.New("could not cast arguments")
	}

	// find survey item:
	surveyItem, err := findSurveyItemResponse(ctx.Event.Response.Responses, arg1Val)
	if err != nil {
		// Item not found
		return "", errors.New("item not found")
	}

	responseObject, err := findResponseObject(surveyItem, arg2Val)
	if err != nil {
		// Item not found
		return "", errors.New("item not found")
	}

	keys := []string{}
	for _, item := range responseObject.Items {
		keys = append(keys, item.Key)
	}
	val = strings.Join(keys, ";")
	return
}

func (ctx EvalContext) countResponseItems(exp studyTypes.Expression) (val float64, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("unexpected numbers of arguments")
	}

	itemKey, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	responseGroupKey, err := ctx.mustGetStrValue(exp.Data[1])
	if err != nil {
		return val, err
	}

	// find survey item:
	surveyItem, err := findSurveyItemResponse(ctx.Event.Response.Responses, itemKey)
	if err != nil {
		// Item not found
		return -1.0, errors.New("item not found")
	}

	responseObject, err := findResponseObject(surveyItem, responseGroupKey)
	if err != nil {
		// Item not found
		return -1.0, errors.New("item not found")
	}

	val = float64(len(responseObject.Items))
	return
}

func (ctx EvalContext) hasResponseKey(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("unexpected numbers of arguments")
	}

	itemKey, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	responseGroupKey, err := ctx.mustGetStrValue(exp.Data[1])
	if err != nil {
		return val, err
	}

	// find survey item:
	surveyItem, err := findSurveyItemResponse(ctx.Event.Response.Responses, itemKey)
	if err != nil {
		// Item not found
		return false, nil
	}

	_, err = findResponseObject(surveyItem, responseGroupKey)
	if err != nil {
		// Item not found
		return false, nil
	}
	return true, nil
}

func (ctx EvalContext) hasResponseKeyWithValue(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 3 {
		return val, errors.New("unexpected numbers of arguments")
	}

	itemKey, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	responseKey, err := ctx.mustGetStrValue(exp.Data[1])
	if err != nil {
		return val, err
	}

	value, err := ctx.mustGetStrValue(exp.Data[2])
	if err != nil {
		return val, err
	}

	// find survey item:
	surveyItem, err := findSurveyItemResponse(ctx.Event.Response.Responses, itemKey)
	if err != nil {
		// Item not found
		return false, nil
	}

	responseObject, err := findResponseObject(surveyItem, responseKey)
	if err != nil {
		// Item not found
		return false, nil
	}

	val = responseObject.Value == value
	return
}

func (ctx EvalContext) eq(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("not expected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}

	switch arg1Val := arg1.(type) {
	case string:
		arg2Val, ok2 := arg2.(string)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return strings.Compare(arg1Val, arg2Val) == 0, nil
	case float64:
		arg2Val, ok2 := arg2.(float64)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return arg1Val == arg2Val, nil
	default:
		return val, fmt.Errorf("I don't know about type %T", arg1Val)
	}
}

func (ctx EvalContext) lt(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("not expected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}

	switch arg1Val := arg1.(type) {
	case string:
		arg2Val, ok2 := arg2.(string)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return strings.Compare(arg1Val, arg2Val) == -1, nil
	case float64:
		arg2Val, ok2 := arg2.(float64)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return arg1Val < arg2Val, nil
	default:
		return val, fmt.Errorf("I don't know about type %T", arg1Val)
	}
}

func (ctx EvalContext) lte(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("not expected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}

	switch arg1Val := arg1.(type) {
	case string:
		arg2Val, ok2 := arg2.(string)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return strings.Compare(arg1Val, arg2Val) <= 0, nil
	case float64:
		arg2Val, ok2 := arg2.(float64)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return arg1Val <= arg2Val, nil
	default:
		return val, fmt.Errorf("I don't know about type %T", arg1Val)
	}
}

func (ctx EvalContext) gt(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("not expected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}

	switch arg1Val := arg1.(type) {
	case string:
		arg2Val, ok2 := arg2.(string)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return strings.Compare(arg1Val, arg2Val) == 1, nil
	case float64:
		arg2Val, ok2 := arg2.(float64)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return arg1Val > arg2Val, nil
	default:
		return val, fmt.Errorf("unexpected type %T", arg1Val)
	}
}

func (ctx EvalContext) gte(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("not expected numbers of arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}

	switch arg1Val := arg1.(type) {
	case string:
		arg2Val, ok2 := arg2.(string)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return strings.Compare(arg1Val, arg2Val) >= 0, nil
	case float64:
		arg2Val, ok2 := arg2.(float64)
		if !ok2 {
			return val, errors.New("could not cast arguments")
		}
		return arg1Val >= arg2Val, nil
	default:
		return val, fmt.Errorf("I don't know about type %T", arg1Val)
	}
}

func (ctx EvalContext) hasMessageTypeAssigned(exp studyTypes.Expression, withIncomingParticipantState bool) (val bool, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) != 1 {
		return val, errors.New("should have at exactly one argument")
	}
	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	for _, m := range pState.Messages {
		if m.Type == arg1 {
			return true, nil
		}
	}
	return false, nil
}

func (ctx EvalContext) getMessageNextTime(exp studyTypes.Expression, withIncomingParticipantState bool) (val int64, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	if len(exp.Data) != 1 {
		return val, errors.New("should have at exactly one argument")
	}
	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	msgType := arg1.(string)
	nextTime := int64(0)
	for _, m := range pState.Messages {
		if m.Type == msgType {
			if nextTime == 0 || nextTime > m.ScheduledFor {
				nextTime = m.ScheduledFor
			}
		}
	}
	if nextTime == 0 {
		return 0, errors.New("no message for this type found")
	}
	return nextTime, nil
}

func (ctx EvalContext) getCurrentStudySession(withIncomingParticipantState bool) (val string, err error) {
	pState := ctx.ParticipantState
	if withIncomingParticipantState {
		pState = ctx.Event.MergeWithParticipant
	}
	return pState.CurrentStudySession, nil
}

func (ctx EvalContext) and(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) < 2 {
		return val, errors.New("should have at least two arguments")
	}

	for _, d := range exp.Data {
		arg1, err := ctx.ExpressionArgResolver(d)
		if err != nil {
			return val, err
		}
		switch arg1Val := arg1.(type) {
		case bool:
			if !arg1Val {
				return false, nil
			}
		case float64:
			if arg1Val == 0 {
				return false, nil
			}
		}
	}
	return true, nil
}

func (ctx EvalContext) or(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) < 2 {
		return val, errors.New("should have at least two arguments")
	}

	for _, d := range exp.Data {
		arg1, err := ctx.ExpressionArgResolver(d)
		if err != nil {
			slog.Debug("unexpected error during expression eval", slog.String("expression", exp.Name), slog.String("error", err.Error()))
			continue
		}
		switch arg1Val := arg1.(type) {
		case bool:
			if arg1Val {
				return true, nil
			}
		case float64:
			if arg1Val > 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (ctx EvalContext) not(exp studyTypes.Expression) (val bool, err error) {
	if len(exp.Data) != 1 {
		return val, errors.New("should have one argument")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	switch arg1Val := arg1.(type) {
	case bool:
		return !arg1Val, nil
	case float64:
		if arg1Val == 0 {
			return true, nil
		}
		return false, nil
	}
	return
}

func (ctx EvalContext) sum(exp studyTypes.Expression) (t float64, err error) {
	for idx, dataExp := range exp.Data {
		arg, err := ctx.ExpressionArgResolver(dataExp)
		if err != nil {
			slog.Error("unexpected error during expression eval", slog.Int("index", idx), slog.String("expression", exp.Name), slog.String("error", err.Error()))
			continue
		}
		switch v := arg.(type) {
		case bool:
			if v {
				t = t + 1
			}
		case float64:
			t = t + v
		default:
			slog.Error("unexpected type during expression eval", slog.Int("index", idx), slog.String("expression", exp.Name), slog.String("type", fmt.Sprintf("%T", arg)))
		}

	}
	return
}

func (ctx EvalContext) neg(exp studyTypes.Expression) (val float64, err error) {
	if len(exp.Data) != 1 {
		return val, errors.New("should have one argument")
	}

	arg, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	v, ok := arg.(float64)
	if !ok {
		return val, errors.New("argument 1 should be resolved as type number (float64)")
	}
	val = -1 * v
	return
}

func (ctx EvalContext) timestampWithOffset(exp studyTypes.Expression) (t float64, err error) {
	if len(exp.Data) != 1 && len(exp.Data) != 2 {
		return t, errors.New("should have one or two arguments")
	}

	arg1, err1 := ctx.ExpressionArgResolver(exp.Data[0])
	if err1 != nil {
		return t, err1
	}
	arg1Float, ok := arg1.(float64)
	if !ok {
		return t, errors.New("argument 1 should be resolved as type number (float64)")
	}
	delta := int64(arg1Float)

	referenceTime := Now().Unix()
	if len(exp.Data) == 2 {
		arg2, err2 := ctx.ExpressionArgResolver(exp.Data[1])
		if err2 != nil {
			return t, err2
		}
		arg2Float, ok := arg2.(float64)
		if !ok {
			return t, errors.New("argument 2 should be resolved as type number (float64)")
		}

		referenceTime = int64(arg2Float)
	}

	t = float64(referenceTime + delta)
	return
}

func (ctx EvalContext) timestampDiff(exp studyTypes.Expression) (t float64, err error) {
	if len(exp.Data) != 2 {
		return t, errors.New("should have exactly two arguments")
	}

	arg1, err1 := ctx.ExpressionArgResolver(exp.Data[0])
	if err1 != nil {
		return t, err1
	}
	laterTimestamp, ok := arg1.(float64)
	if !ok {
		return t, errors.New("argument 1 should be resolved as type number (float64)")
	}

	arg2, err2 := ctx.ExpressionArgResolver(exp.Data[1])
	if err2 != nil {
		return t, err2
	}
	earlierTimestamp, ok := arg2.(float64)
	if !ok {
		return t, errors.New("argument 2 should be resolved as type number (float64)")
	}

	t = laterTimestamp - earlierTimestamp
	return
}

func (ctx EvalContext) getTsForNextStartOfMonth(exp studyTypes.Expression) (t float64, err error) {
	if len(exp.Data) != 1 && len(exp.Data) != 2 {
		return t, errors.New("should have one or two arguments")
	}

	arg1, err1 := ctx.ExpressionArgResolver(exp.Data[0])
	if err1 != nil {
		return t, err1
	}

	var month int
	// Check if arg1 is a string or a number
	switch v := arg1.(type) {
	case float64:
		month = int(v)
		if month < 1 || month > 12 {
			return t, errors.New("month number should be between 1 and 12")
		}
	case string:
		// Convert month name to month number
		monthStr := strings.ToLower(v)
		switch monthStr {
		case "january", "jan":
			month = 1
		case "february", "feb":
			month = 2
		case "march", "mar":
			month = 3
		case "april", "apr":
			month = 4
		case "may":
			month = 5
		case "june", "jun":
			month = 6
		case "july", "jul":
			month = 7
		case "august", "aug":
			month = 8
		case "september", "sep":
			month = 9
		case "october", "oct":
			month = 10
		case "november", "nov":
			month = 11
		case "december", "dec":
			month = 12
		default:
			return t, errors.New("invalid month name: " + v)
		}
	default:
		return t, errors.New("argument 1 should be a month name (string) or month number (float64)")
	}

	referenceTime := Now()
	if len(exp.Data) == 2 {
		arg2, err2 := ctx.ExpressionArgResolver(exp.Data[1])
		if err2 != nil {
			return t, err2
		}
		arg2Float, ok := arg2.(float64)
		if !ok {
			return t, errors.New("argument 2 should be resolved as type number (float64)")
		}

		referenceTime = time.Unix(int64(arg2Float), 0)
	}

	// Get the first day of the next occurrence of the specified month
	currentYear := referenceTime.Year()
	currentMonth := int(referenceTime.Month())

	targetYear := currentYear
	if currentMonth > month {
		// If the current month is after the target month, we need to go to next year
		targetYear++
	}

	// Create the first day of the target month
	firstOfMonth := time.Date(targetYear, time.Month(month), 1, 0, 0, 0, 0, referenceTime.Location())

	// If the target month is the current month, but we've already passed the 1st day
	// we need to go to the next year
	if currentMonth == month && referenceTime.After(firstOfMonth) {
		firstOfMonth = time.Date(targetYear+1, time.Month(month), 1, 0, 0, 0, 0, referenceTime.Location())
	}

	t = float64(firstOfMonth.Unix())
	return
}

func (ctx EvalContext) getTsForNextISOWeek(exp studyTypes.Expression) (t float64, err error) {
	if len(exp.Data) != 1 && len(exp.Data) != 2 {
		return t, errors.New("should have one or two arguments")
	}

	arg1, err1 := ctx.ExpressionArgResolver(exp.Data[0])
	if err1 != nil {
		return t, err1
	}
	arg1Float, ok := arg1.(float64)
	if !ok {
		return t, errors.New("argument 1 should be resolved as type number (float64)")
	}

	ISOWeek := int64(arg1Float)

	if ISOWeek < 1 || ISOWeek > 53 {
		return t, errors.New("argument 1 should be between 1 and 53")
	}

	referenceTime := Now()
	if len(exp.Data) == 2 {
		arg2, err2 := ctx.ExpressionArgResolver(exp.Data[1])
		if err2 != nil {
			return t, err2
		}
		arg2Float, ok := arg2.(float64)
		if !ok {
			return t, errors.New("argument 2 should be resolved as type number (float64)")
		}

		referenceTime = time.Unix(int64(arg2Float), 0)
	}

	for {
		_, week := referenceTime.ISOWeek()
		if week == int(ISOWeek) {
			break
		}
		referenceTime = referenceTime.AddDate(0, 0, 1)
	}

	weekday := int(referenceTime.Weekday())
	if weekday == 0 {
		// time.Sunday is 0, but for ISO weeks Sunday is the 7th day
		weekday = 7
	}

	startOfWeek := referenceTime.AddDate(0, 0, -weekday+1)
	startOfWeek = time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, startOfWeek.Location())

	t = float64(startOfWeek.Unix())
	return
}

func (ctx EvalContext) getTsForStartOfISOWeek(exp studyTypes.Expression) (t float64, err error) {
	if len(exp.Data) != 0 && len(exp.Data) != 1 {
		return t, errors.New("should have zero or one argument")
	}

	referenceTime := Now()
	if len(exp.Data) == 1 {
		arg1, err1 := ctx.ExpressionArgResolver(exp.Data[0])
		if err1 != nil {
			return t, err1
		}
		arg1Float, ok := arg1.(float64)
		if !ok {
			return t, errors.New("argument 1 should be resolved as type number (float64)")
		}

		referenceTime = time.Unix(int64(arg1Float), 0)
	}

	weekday := int(referenceTime.Weekday())
	if weekday == 0 {
		// time.Sunday is 0, but for ISO weeks Sunday is the 7th day
		weekday = 7
	}

	startOfWeek := referenceTime.AddDate(0, 0, -weekday+1)
	startOfWeek = time.Date(startOfWeek.Year(), startOfWeek.Month(), startOfWeek.Day(), 0, 0, 0, 0, startOfWeek.Location())

	t = float64(startOfWeek.Unix())
	return
}

func (ctx EvalContext) getISOWeekForTs(exp studyTypes.Expression) (t float64, err error) {
	if len(exp.Data) != 1 {
		return t, errors.New("should have one argument")
	}

	arg1, err1 := ctx.ExpressionArgResolver(exp.Data[0])
	if err1 != nil {
		return t, err1
	}
	arg1Float, ok := arg1.(float64)
	if !ok {
		return t, errors.New("argument 1 should be resolved as type number (float64)")
	}

	ts := int64(arg1Float)
	_, iw := time.Unix(ts, 0).ISOWeek()
	t = float64(iw)
	return
}

func (ctx EvalContext) dateToStr(exp studyTypes.Expression) (val string, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("dateToStr: expected exactly two arguments")
	}

	// Get the timestamp to convert
	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}

	var timestamp float64
	switch v := arg1.(type) {
	case float64:
		timestamp = v
	case int64:
		timestamp = float64(v)
	default:
		return val, errors.New("dateToStr: first argument must be a number (timestamp)")
	}

	// Get the format string
	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}
	format, ok := arg2.(string)
	if !ok {
		return val, errors.New("dateToStr: second argument must be a string (format)")
	}

	// Convert timestamp to time.Time
	t := time.Unix(int64(timestamp), 0)

	// Use our date-fns style formatter
	val = FormatTimeWithDateFns(t, format)
	return
}

func (ctx EvalContext) parseValueAsNum(exp studyTypes.Expression) (val float64, err error) {
	if len(exp.Data) != 1 {
		return val, errors.New("should have one argument")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}

	if v, ok := arg1.(float64); ok {
		return v, nil
	}

	strVal, ok := arg1.(string)
	if !ok {
		return val, errors.New("argument 1 should be resolved as type string")
	}

	val, err = strconv.ParseFloat(strVal, 64)

	return
}

func (ctx EvalContext) generateRandomNumber(exp studyTypes.Expression) (val float64, err error) {
	if len(exp.Data) != 2 {
		return val, errors.New("should have two arguments")
	}

	arg1, err := ctx.ExpressionArgResolver(exp.Data[0])
	if err != nil {
		return val, err
	}
	arg1Float, ok := arg1.(float64)
	if !ok {
		return val, errors.New("argument 1 should be resolved as type number (float64)")
	}
	min := int(arg1Float)

	arg2, err := ctx.ExpressionArgResolver(exp.Data[1])
	if err != nil {
		return val, err
	}
	arg2Float, ok := arg2.(float64)
	if !ok {
		return val, errors.New("argument 2 should be resolved as type number (float64)")
	}
	max := int(arg2Float)

	rand.Seed(time.Now().UnixNano())
	randomVal := rand.Intn(max-min+1) + min
	return float64(randomVal), nil
}

func (ctx EvalContext) externalEventEval(exp studyTypes.Expression) (val interface{}, err error) {
	if len(exp.Data) < 1 {
		return val, errors.New("should have at least one argument")
	}

	serviceName, err := ctx.mustGetStrValue(exp.Data[0])
	if err != nil {
		return val, err
	}

	serviceConfig, err := getExternalServicesConfigByName(serviceName)
	if err != nil {
		slog.Error("unexpected error during expression eval", slog.String("expression", exp.Name), slog.String("error", err.Error()))
		return val, err
	}

	pathname := ""

	if len(exp.Data) > 1 {
		arg1, err := ctx.ExpressionArgResolver(exp.Data[1])
		if err != nil {
			return val, err
		}

		route := arg1.(string)
		route = strings.TrimPrefix(route, "/")
		pathname = route
	}

	var mTLSConfig *apihelpers.CertificatePaths
	if serviceConfig.MutualTLSConfig != nil {
		mTLSConfig = &apihelpers.CertificatePaths{
			CACertPath:     serviceConfig.MutualTLSConfig.CAFile,
			ServerCertPath: serviceConfig.MutualTLSConfig.CertFile,
			ServerKeyPath:  serviceConfig.MutualTLSConfig.KeyFile,
		}
	}

	httpClient := httpclient.ClientConfig{
		RootURL:                   serviceConfig.URL,
		APIKey:                    serviceConfig.APIKey,
		Timeout:                   time.Duration(serviceConfig.Timeout) * time.Second,
		MutualTLSCertificatePaths: mTLSConfig,
	}

	payload := ExternalEventPayload{
		ParticipantState: ctx.ParticipantState,
		EventType:        ctx.Event.Type,
		StudyKey:         ctx.Event.StudyKey,
		InstanceID:       ctx.Event.InstanceID,
		Response:         ctx.Event.Response,
		EventKey:         ctx.Event.EventKey,
		Payload:          ctx.Event.Payload,
	}

	response, err := httpClient.RunHTTPcall(pathname, payload)
	if err != nil {
		slog.Error("unexpected error during expression eval", slog.String("expression", exp.Name), slog.String("error", err.Error()))
		return val, err
	}

	// if relevant, update participant state:
	value := response["value"]
	if exp.ReturnType == "float" {
		return value.(float64), nil
	}
	return value, nil
}

func (ctx EvalContext) mustGetStrValue(arg studyTypes.ExpressionArg) (string, error) {
	arg1, err := ctx.ExpressionArgResolver(arg)
	if err != nil {
		return "", err
	}
	val, ok := arg1.(string)
	if !ok {
		return "", errors.New("could not cast argument")
	}
	return val, nil
}
