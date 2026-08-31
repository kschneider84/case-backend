package apihandlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/case-framework/case-backend/pkg/apihelpers"
	mw "github.com/case-framework/case-backend/pkg/apihelpers/middlewares"
	dbStudy "github.com/case-framework/case-backend/pkg/db/study"
	jwthandling "github.com/case-framework/case-backend/pkg/jwt-handling"
	pc "github.com/case-framework/case-backend/pkg/permission-checker"
	studyService "github.com/case-framework/case-backend/pkg/study"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
)

func (h *HttpEndpoints) addParticipantManagementEndpoints(rg *gin.RouterGroup) {
	participantGroup := rg.Group("/participants")

	participantGroup.POST("/virtual",
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_CREATE_VIRTUAL_PARTICIPANT,
			},
			nil,
			h.createVirtualParticipant,
		))

	participantGroup.POST("/:participantID/responses",
		mw.RequirePayload(),
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_EDIT_PARTICIPANT_DATA,
			},
			nil,
			h.submitParticipantResponse,
		))

	participantGroup.GET("/:participantID/responses", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_GET_RESPONSES,
		},
		nil,
		h.getParticipantResponses,
	))

	participantGroup.POST("/:participantID/events",
		mw.RequirePayload(),
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_EDIT_PARTICIPANT_DATA,
			},
			nil,
			h.submitParticipantEvent,
		))

	participantGroup.POST("/:participantID/remove-session",
		mw.RequirePayload(),
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_EDIT_PARTICIPANT_DATA,
			},
			nil,
			h.removeStudySession,
		))

	participantGroup.POST("/:participantID/reports",
		mw.RequirePayload(),
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_EDIT_PARTICIPANT_DATA,
			},
			nil,
			h.submitParticipantReport,
		))

	participantGroup.PUT("/:participantID/reports/:reportID", mw.RequirePayload(),
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_EDIT_PARTICIPANT_DATA,
			},
			nil,
			h.updateParticipantReport,
		))

	participantGroup.POST("/merge",
		mw.RequirePayload(),
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_MERGE_PARTICIPANTS,
			},
			nil,
			h.mergeParticipants,
		))

	participantGroup.PUT("/:participantID",
		mw.RequirePayload(),
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_EDIT_PARTICIPANT_DATA,
			},
			nil,
			h.editStudyParticipant,
		))
}

func (h *HttpEndpoints) createVirtualParticipant(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("creating virtual participant", slog.String("studyKey", studyKey), slog.String("userID", token.Subject), slog.String("instanceID", token.InstanceID))

	pState, err := studyService.OnRegisterVirtualParticipant(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to create virtual participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create virtual participant"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "virtual participant created",
		"participant": pState,
	})
}

func (h *HttpEndpoints) submitParticipantResponse(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")

	var req studyTypes.SurveyResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("submitting response for participant", slog.String("participantID", participantID), slog.String("studyKey", studyKey), slog.String("userID", token.Subject), slog.String("instanceID", token.InstanceID),
		slog.String("forSurveyKey", req.Key),
	)

	result, err := studyService.OnSubmitResponseOnBehalfOfParticipant(token.InstanceID, studyKey, participantID, req, token.Subject)
	if err != nil {
		slog.Error("failed to submit response", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit response"})
		return
	}

	participant, err := h.studyDBConn.GetParticipantByID(token.InstanceID, studyKey, participantID)
	if err != nil {
		slog.Error("failed to get participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get participant"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "response submitted", "result": result, "participant": participant})
}

func (h *HttpEndpoints) getParticipantResponses(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")

	query, err := apihelpers.ParsePaginatedQueryFromCtx(c)
	if err != nil || query == nil {
		slog.Error("failed to parse paginated query", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	query.Filter["participantID"] = participantID

	slog.Info("getting responses for participant", slog.String("participantID", participantID), slog.String("studyKey", studyKey), slog.String("userID", token.Subject), slog.String("instanceID", token.InstanceID), slog.Any("query", query))

	resps, paginationInfo, err := h.studyDBConn.GetResponses(token.InstanceID, studyKey, query.Filter, query.Sort, query.Page, query.Limit)
	if err != nil {
		slog.Error("failed to get responses", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get responses"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"responses": resps, "pagination": paginationInfo})
}

type RemoveStudySessionRequest struct {
	SessionToRemove    string `json:"sessionToRemove"`
	ReplacementSession string `json:"replacementSession"` // optional; if empty, session association is removed
}

func (h *HttpEndpoints) removeStudySession(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")

	var req RemoveStudySessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.SessionToRemove == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionToRemove is required"})
		return
	}

	// Update participant's current session if it matches the one being removed
	p, err := h.studyDBConn.GetParticipantByID(token.InstanceID, studyKey, participantID)
	if err != nil {
		slog.Error("failed to get participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "participant not found"})
		return
	}

	if p.CurrentStudySession == req.SessionToRemove {
		p.CurrentStudySession = req.ReplacementSession
		p, err = h.studyDBConn.UpdateParticipantIfNotModified(token.InstanceID, studyKey, p)
		if err != nil {
			slog.Error("failed to update participant session", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update participant"})
			return
		}
	}

	// Migrate session label on all responses recorded under the removed session
	count, err := h.studyDBConn.UpdateResponseSessionContext(token.InstanceID, studyKey, participantID, req.SessionToRemove, req.ReplacementSession)
	if err != nil {
		slog.Error("failed to update response sessions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update response sessions"})
		return
	}

	slog.Info("removed study session",
		slog.String("instanceID", token.InstanceID),
		slog.String("studyKey", studyKey),
		slog.String("participantID", participantID),
		slog.String("sessionToRemove", req.SessionToRemove),
		slog.String("replacementSession", req.ReplacementSession),
		slog.Int64("responsesUpdated", count),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":          "session removed",
		"participant":      p,
		"responsesUpdated": count,
	})
}

type ParticipantEventRequest struct {
	EventKey string         `json:"eventKey"`
	Payload  map[string]any `json:"payload"`
}

func (h *HttpEndpoints) submitParticipantEvent(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")

	var req ParticipantEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := studyService.OnCustomStudyEventOnBehalfOfParticipant(token.InstanceID, studyKey, participantID, req.EventKey, req.Payload)
	if err != nil {
		slog.Error("failed to submit event", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "event submitted", "result": result})
}

func (h *HttpEndpoints) submitParticipantReport(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")

	var report studyTypes.Report
	if err := c.ShouldBindJSON(&report); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if report.ParticipantID != participantID {
		slog.Error("participant ID in request does not match participant ID in path", slog.String("requestParticipantID", report.ParticipantID), slog.String("pathParticipantID", participantID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	p, err := h.studyDBConn.GetParticipantByID(token.InstanceID, studyKey, participantID)
	if err != nil {
		slog.Error("failed to get participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "participant not found"})
		return
	}

	if p.StudyStatus == studyTypes.PARTICIPANT_STUDY_STATUS_ACCOUNT_DELETED {
		slog.Error("participant account deleted", slog.String("participantID", participantID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "participant account deleted"})
		return
	}

	slog.Info("submitting report for participant", slog.String("participantID", participantID), slog.String("studyKey", studyKey), slog.String("userID", token.Subject), slog.String("instanceID", token.InstanceID), slog.String("reportKey", report.Key))

	err = h.studyDBConn.SaveReport(token.InstanceID, studyKey, report)
	if err != nil {
		slog.Error("failed to save report", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit report"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "report submitted"})
}

type UpdateParticipantReportRequest struct {
	Data []studyTypes.ReportData             `json:"data"`
	Mode dbStudy.UpdateParticipantReportMode `json:"mode"`
}

func (h *HttpEndpoints) updateParticipantReport(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")
	reportID := c.Param("reportID")

	var req UpdateParticipantReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Mode == "" {
		req.Mode = dbStudy.UpdateParticipantReportModeAppend
	}
	slog.Info("updating report for participant", slog.String("participantID", participantID), slog.String("studyKey", studyKey), slog.String("userID", token.Subject), slog.String("instanceID", token.InstanceID), slog.String("reportID", reportID))

	err := h.studyDBConn.UpdateReportData(token.InstanceID, studyKey, reportID, participantID, req.Data, req.Mode)
	if err != nil {
		slog.Error("failed to update report", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update report"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "report updated"})
}

type MergeParticipantsRequest struct {
	TargetParticipantID string `json:"targetParticipantID"`
	WithParticipantID   string `json:"withParticipantID"`
}

func (h *HttpEndpoints) mergeParticipants(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var req MergeParticipantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TargetParticipantID == req.WithParticipantID {
		slog.Error("target participant ID and with participant ID are the same", slog.String("targetParticipantID", req.TargetParticipantID), slog.String("withParticipantID", req.WithParticipantID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "target participant ID and with participant ID are the same"})
		return
	}

	p, err := studyService.OnForceMergeParticipants(token.InstanceID, studyKey, req.TargetParticipantID, req.WithParticipantID)
	if err != nil {
		slog.Error("failed to merge participants", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to merge participants"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"participant": p})
}

func (h *HttpEndpoints) editStudyParticipant(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")

	slog.Info("updating participant", slog.String("participantID", participantID), slog.String("studyKey", studyKey), slog.String("userID", token.Subject), slog.String("instanceID", token.InstanceID))

	var req studyTypes.Participant
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ParticipantID != participantID {
		slog.Error("participant ID in request does not match participant ID in path", slog.String("requestParticipantID", req.ParticipantID), slog.String("pathParticipantID", participantID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	updatedParticipant, err := h.studyDBConn.UpdateParticipantIfNotModified(token.InstanceID, studyKey, req)
	if err != nil {
		slog.Error("failed to update participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update participant"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "participant updated",
		"participant": updatedParticipant,
	})
}
