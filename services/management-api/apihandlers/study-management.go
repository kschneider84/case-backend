package apihandlers

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/case-framework/case-backend/pkg/apihelpers"
	mw "github.com/case-framework/case-backend/pkg/apihelpers/middlewares"
	managementuser "github.com/case-framework/case-backend/pkg/db/management-user"
	jwthandling "github.com/case-framework/case-backend/pkg/jwt-handling"
	pc "github.com/case-framework/case-backend/pkg/permission-checker"
	studyutils "github.com/case-framework/case-backend/pkg/study/utils"
	"github.com/case-framework/case-backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	studyDB "github.com/case-framework/case-backend/pkg/db/study"
	studyService "github.com/case-framework/case-backend/pkg/study"
	surveydefinition "github.com/case-framework/case-backend/pkg/study/exporter/survey-definition"
	surveyresponses "github.com/case-framework/case-backend/pkg/study/exporter/survey-responses"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
)

const (
	MIN_STUDY_SECRET_KEY_LENGTH = 5
)

func (h *HttpEndpoints) AddStudyManagementAPI(rg *gin.RouterGroup) {
	studiesGroup := rg.Group("/studies")

	studiesGroup.Use(mw.ManagementAuthMiddleware(h.tokenSignKey, h.allowedInstanceIDs, h.muDBConn))
	{
		studiesGroup.GET("/", h.getAllStudies)
		studiesGroup.POST("/", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType: pc.RESOURCE_TYPE_STUDY,
				ResourceKeys: []string{pc.RESOURCE_KEY_STUDY_ALL},
				Action:       pc.ACTION_CREATE_STUDY,
			},
			nil,
			h.createStudy,
		))
	}

	// Study Group
	studyGroup := studiesGroup.Group("/:studyKey")
	{
		h.addGeneralStudyEndpoints(studyGroup)
		h.addStudyConfigEndpoints(studyGroup)
		h.addStudyRuleEndpoints(studyGroup)
		h.addSurveyEndpoints(studyGroup)
		h.addStudyActionEndpoints(studyGroup)
		h.addStudyDataExporterEndpoints(studyGroup)
		h.addStudyDataExplorerEndpoints(studyGroup)
		h.addParticipantManagementEndpoints(studyGroup)
	}
}

func getStudyKeyFromParams(c *gin.Context) []string {
	return []string{c.Param("studyKey")}
}

func getSurveyKeyLimiterFromContext(c *gin.Context) map[string]string {
	return map[string]string{"surveyKey": c.Param("surveyKey")}
}

func getSurveyKeyLimiterFromQuery(c *gin.Context) map[string]string {
	sk := c.DefaultQuery("surveyKey", "")
	if sk == "" {
		return nil
	}
	return map[string]string{"surveyKey": sk}
}

func getReportKeyLimiterFromQuery(c *gin.Context) map[string]string {
	rk := c.DefaultQuery("reportKey", "")
	if rk == "" {
		return nil
	}
	return map[string]string{"reportKey": rk}
}

func (h *HttpEndpoints) addGeneralStudyEndpoints(rg *gin.RouterGroup) {
	rg.GET("/", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_READ_STUDY_CONFIG,
		},
		nil,
		h.getStudyProps,
	))

	rg.GET("/export-config", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_READ_STUDY_CONFIG,
		},
		nil,
		h.exportStudyConfig,
	)) // config=true&survey=true&rules=true

	rg.PUT("/is-default", mw.RequirePayload(), h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_UPDATE_STUDY_PROPS,
		},
		nil,
		h.updateStudyIsDefault,
	))

	// change status
	rg.PUT("/status", mw.RequirePayload(), h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_UPDATE_STUDY_STATUS,
		},
		nil,
		h.updateStudyStatus,
	))

	// update study display props (name, description, tags)
	rg.PUT("/display-props", mw.RequirePayload(), h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_UPDATE_STUDY_PROPS,
		},
		nil,
		h.updateStudyDisplayProps,
	))

	rg.PUT("/file-upload-config", mw.RequirePayload(), h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_UPDATE_STUDY_PROPS,
		},
		nil,
		h.updateStudyFileUploadRule,
	))

	rg.PUT("/track-account", mw.RequirePayload(), h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_UPDATE_STUDY_PROPS,
		},
		nil,
		h.updateStudyTrackAccount,
	))

	rg.DELETE("/", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_DELETE_STUDY,
		},
		nil,
		h.deleteStudy,
	))
}

func (h *HttpEndpoints) addSurveyEndpoints(rg *gin.RouterGroup) {
	surveysGroup := rg.Group("/surveys")
	{
		surveysGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getSurveyInfoList,
		))

		surveysGroup.POST("/", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_CREATE_SURVEY,
			},
			nil,
			h.createSurvey,
		))
	}

	surveyGroup := surveysGroup.Group("/:surveyKey")
	{
		surveyGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getLatestSurvey,
		))

		surveyGroup.POST("/", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_UPDATE_SURVEY,
			},
			getSurveyKeyLimiterFromContext,
			h.updateSurvey,
		))

		surveyGroup.POST("/unpublish", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_UNPUBLISH_SURVEY,
			},
			getSurveyKeyLimiterFromContext,
			h.unpublishSurvey,
		))

		surveyGroup.GET("/versions", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getSurveyVersions,
		))

		surveyGroup.GET("/versions/:versionID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getSurveyVersion,
		))

		surveyGroup.DELETE("/versions/:versionID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_DELETE_SURVEY_VERSION,
			},
			getSurveyKeyLimiterFromContext,
			h.deleteSurveyVersion,
		))

	}
}

func (h *HttpEndpoints) addStudyConfigEndpoints(rg *gin.RouterGroup) {

	permissionsGroup := rg.Group("/permissions")
	{
		permissionsGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getStudyPermissions,
		))

		permissionsGroup.POST("/", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_MANAGE_STUDY_PERMISSIONS,
			},
			nil,
			h.addStudyPermission,
		))

		permissionsGroup.DELETE("/:permissionID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_MANAGE_STUDY_PERMISSIONS,
			},
			nil,
			h.deleteStudyPermission,
		))
	}

	notificationSubGroup := rg.Group("/notification-subscriptions")
	{
		notificationSubGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getNotificationSubscriptions,
		))

		notificationSubGroup.PUT("/", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_UPDATE_NOTIFICATION_SUBSCRIPTIONS,
			},
			nil,
			h.updateNotificationSubscriptions,
		))
	}

	studyCodeListGroup := rg.Group("/study-code-list")
	// get study code list keys
	studyCodeListGroup.GET("/list-keys",
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getStudyCodeListKeysHandler,
		))

	// get study codes for list key
	studyCodeListGroup.GET("/codes",
		h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getStudyCodeListEntriesHandler, // ?listKey=xxx&page=1&limit=10
		))

	// add study codes
	studyCodeListGroup.POST("/codes", mw.RequirePayload(), h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_MANAGE_STUDY_CODE_LISTS,
		},
		nil,
		h.addStudyCodeListEntriesHandler,
	))

	// remove study code
	studyCodeListGroup.DELETE("/codes", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_MANAGE_STUDY_CODE_LISTS,
		},
		nil,
		h.removeStudyCodeListEntryHandler, // ?listKey=xy&code=abc
	))

	studyCounterGroup := rg.Group("/study-counters")
	{
		studyCounterGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getStudyCounterValues,
		))
	}

	studyCounterGroup.POST("/:scope", mw.RequirePayload(), h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_MANAGE_STUDY_COUNTERS,
		},
		nil,
		h.saveStudyCounterValue,
	))

	studyCounterGroup.PUT("/:scope", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_MANAGE_STUDY_COUNTERS,
		},
		nil,
		h.incrementStudyCounter,
	))

	studyCounterGroup.DELETE("/:scope", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_MANAGE_STUDY_COUNTERS,
		},
		nil,
		h.removeStudyCounter,
	))

	studyVariablesGroup := rg.Group("/variables")
	{
		studyVariablesGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getStudyVariables,
		))

		studyVariablesGroup.GET("/:variableKey", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getStudyVariable,
		))

		studyVariablesGroup.POST("/", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_MANAGE_STUDY_VARIABLES,
			},
			nil,
			h.addStudyVariable,
		))

		studyVariablesGroup.PUT("/:variableKey", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_MANAGE_STUDY_VARIABLES,
			},
			nil,
			h.updateStudyVariableDef,
		))

		studyVariablesGroup.PUT("/:variableKey/value", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_MANAGE_STUDY_VARIABLES,
			},
			nil,
			h.updateStudyVariableValue,
		))

		studyVariablesGroup.DELETE("/:variableKey", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_MANAGE_STUDY_VARIABLES,
			},
			nil,
			h.deleteStudyVariable,
		))
	}
}

func (h *HttpEndpoints) addStudyRuleEndpoints(rg *gin.RouterGroup) {
	rulesGroup := rg.Group("/rules")

	rulesGroup.GET("/", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_READ_STUDY_CONFIG,
		},
		nil,
		h.getCurrentStudyRules,
	))

	rulesGroup.POST("/", mw.RequirePayload(), h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_UPDATE_STUDY_RULES,
		},
		nil,
		h.publishNewStudyRulesVersion,
	))

	// get rule history
	rulesGroup.GET("/versions", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_READ_STUDY_CONFIG,
		},
		nil,
		h.getStudyRuleVersions,
	))

	// get specific rule version
	rulesGroup.GET("/versions/:id", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_READ_STUDY_CONFIG,
		},
		nil,
		h.getStudyRuleVersion,
	))

	// delete rule version
	rulesGroup.DELETE("/versions/:id", h.useAuthorisedHandler(
		RequiredPermission{
			ResourceType:        pc.RESOURCE_TYPE_STUDY,
			ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
			ExtractResourceKeys: getStudyKeyFromParams,
			Action:              pc.ACTION_UPDATE_STUDY_RULES,
		},
		nil,
		h.deleteStudyRuleVersion,
	))
}

func (h *HttpEndpoints) addStudyActionEndpoints(rg *gin.RouterGroup) {
	actionsGroup := rg.Group("/run-actions")

	// run actions on current participant states
	participantGroup := actionsGroup.Group("/participants")
	{
		participantGroup.POST("/:participantID", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_RUN_STUDY_ACTION,
			},
			nil,
			h.runActionOnParticipant,
		))

		participantGroup.POST("/", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_RUN_STUDY_ACTION,
			},
			nil,
			h.runActionOnParticipants,
		))

		participantGroup.GET("/task/:taskID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_RUN_STUDY_ACTION,
			},
			nil,
			h.getStudyActionTaskStatus,
		))

		participantGroup.GET("/task/:taskID/result", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_RUN_STUDY_ACTION,
			},
			nil,
			h.getStudyActionTaskResult,
		))
	}

	// run action on previous responses of a participant
	previousResponsesGroup := actionsGroup.Group("/previous-responses")
	{
		previousResponsesGroup.POST("/:participantID", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_RUN_STUDY_ACTION,
			},
			nil,
			h.runActionOnPreviousResponsesForParticipant,
		))

		previousResponsesGroup.POST("/", mw.RequirePayload(), h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_RUN_STUDY_ACTION,
			},
			nil,
			h.runActionOnPreviousResponsesForParticipants,
		))

		previousResponsesGroup.GET("/task/:taskID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_RUN_STUDY_ACTION,
			},
			nil,
			h.getStudyActionTaskStatus,
		))

		previousResponsesGroup.GET("/task/:taskID/result", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_RUN_STUDY_ACTION,
			},
			nil,
			h.getStudyActionTaskResult,
		))
	}
}

func (h *HttpEndpoints) addStudyDataExporterEndpoints(rg *gin.RouterGroup) {
	exporterGroup := rg.Group("/data-exporter")

	surveyInfoGroup := exporterGroup.Group("/survey-info")
	{
		// get survey info
		surveyInfoGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_READ_STUDY_CONFIG,
			},
			nil,
			h.getSurveyInfo,
		))
	}

	responsesGroup := exporterGroup.Group("/responses")
	{
		// count responses
		responsesGroup.GET("/count", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_RESPONSES,
			},
			getSurveyKeyLimiterFromQuery,
			h.getResponsesCount,
		))

		// start export generation for responses
		responsesGroup.POST("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_RESPONSES,
			},
			getSurveyKeyLimiterFromQuery,
			h.generateResponsesExport,
		))

		// get export status
		responsesGroup.GET("/task/:taskID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_RESPONSES,
			},
			nil,
			h.getExportTaskStatus,
		))

		// get export result
		responsesGroup.GET("/task/:taskID/result", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_RESPONSES,
			},
			nil,
			h.getExportTaskResult,
		))

		responsesGroup.GET("/daily-exports", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_RESPONSES,
			},
			nil,
			h.getDailyExports,
		))

		responsesGroup.GET("/daily-exports/:exportID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_RESPONSES,
			},
			nil,
			h.getDailyExport,
		))
	}

	participantsGroup := exporterGroup.Group("/participants")
	{
		// count participants of the query
		participantsGroup.GET("/count", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_PARTICIPANT_STATES,
			},
			nil,
			h.getParticipantsCount,
		))

		// start export generation for participants
		participantsGroup.POST("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_PARTICIPANT_STATES,
			},
			nil,
			h.generateParticipantsExport,
		))

		// get export status
		participantsGroup.GET("/task/:taskID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_PARTICIPANT_STATES,
			},
			nil,
			h.getExportTaskStatus,
		))

		// get export result
		participantsGroup.GET("/task/:taskID/result", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_PARTICIPANT_STATES,
			},
			nil,
			h.getExportTaskResult,
		))
	}

	reportsGroup := exporterGroup.Group("/reports")
	{
		// count reports
		reportsGroup.GET("/count", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_REPORTS,
			},
			getReportKeyLimiterFromQuery,
			h.getReportsCount,
		))

		// start export generation for reports
		reportsGroup.POST("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_REPORTS,
			},
			getReportKeyLimiterFromQuery,
			h.generateReportsExport,
		))

		// get export status
		reportsGroup.GET("/task/:taskID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_REPORTS,
			},
			nil,
			h.getExportTaskStatus,
		))

		// get export result
		reportsGroup.GET("/task/:taskID/result", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_REPORTS,
			},
			nil,
			h.getExportTaskResult,
		))
	}

	confidentialResponsesGroup := exporterGroup.Group("/confidential-responses")
	{

		// start export generation for confidential responses
		confidentialResponsesGroup.POST("/",
			mw.RequirePayload(),
			h.useAuthorisedHandler(
				RequiredPermission{
					ResourceType:        pc.RESOURCE_TYPE_STUDY,
					ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
					ExtractResourceKeys: getStudyKeyFromParams,
					Action:              pc.ACTION_GET_CONFIDENTIAL_RESPONSES,
				},
				nil,
				h.getConfidentialResponses,
			),
		)

		confidentialResponsesGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_CONFIDENTIAL_RESPONSES,
			},
			nil,
			h.getAvailableConfidentailDataExports,
		))

		confidentialResponsesGroup.GET("/:exportID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_CONFIDENTIAL_RESPONSES,
			},
			nil,
			h.getAvailableConfidentailDataExport,
		))

	}
}

func (h *HttpEndpoints) addStudyDataExplorerEndpoints(rg *gin.RouterGroup) {
	dataExplGroup := rg.Group("/data-explorer")

	responsesGroup := dataExplGroup.Group("/responses")
	{
		// get responses with pagination
		responsesGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_RESPONSES,
			},
			getSurveyKeyLimiterFromQuery,
			h.getStudyResponses,
		))

		responsesGroup.GET("/:responseID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_RESPONSES,
			},
			getSurveyKeyLimiterFromQuery,
			h.getStudyResponseById,
		))

		// delete responses
		responsesGroup.DELETE("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_DELETE_RESPONSES,
			},
			nil,
			h.deleteStudyResponses,
		))

		responsesGroup.DELETE("/:responseID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_DELETE_RESPONSES,
			},
			nil,
			h.deleteStudyResponse,
		))
	}

	participantsGroup := dataExplGroup.Group("/participants")
	{
		// get participants with pagination
		participantsGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_PARTICIPANT_STATES,
			},
			nil,
			h.getStudyParticipants,
		))

		// get single participant
		participantsGroup.GET("/:participantID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_PARTICIPANT_STATES,
			},
			nil,
			h.getStudyParticipant,
		))
	}

	reportsGroup := dataExplGroup.Group("/reports")
	{
		// get available report keys
		reportsGroup.GET("/keys", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_REPORTS,
			},
			nil,
			h.getReportKeys,
		)) // ?pid=<pid>&from=<ts1>&until=<ts2>

		// get reports with pagination
		reportsGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_REPORTS,
			},
			getReportKeyLimiterFromQuery,
			h.getStudyReports,
		))

		// get single report by ID
		reportsGroup.GET("/:reportID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_REPORTS,
			},
			nil,
			h.getStudyReport,
		))
	}

	filesGroup := dataExplGroup.Group("/files")
	{
		// get files with pagination
		filesGroup.GET("/", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_FILES,
			},
			nil,
			h.getStudyFiles,
		))

		// get single file by ID
		filesGroup.GET("/:fileID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_GET_FILES,
			},
			nil,
			h.getStudyFile,
		))

		// delete file by ID
		filesGroup.DELETE("/:fileID", h.useAuthorisedHandler(
			RequiredPermission{
				ResourceType:        pc.RESOURCE_TYPE_STUDY,
				ResourceKeys:        []string{pc.RESOURCE_KEY_STUDY_ALL},
				ExtractResourceKeys: getStudyKeyFromParams,
				Action:              pc.ACTION_DELETE_FILES,
			},
			nil,
			h.deleteStudyFile,
		))
	}
}

func (h *HttpEndpoints) getAllStudies(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	slog.Info("getting all studies", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject))

	studies, err := h.studyDBConn.GetStudies(token.InstanceID, "", false)
	if err != nil {
		slog.Error("failed to get all studies", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get studies"})
		return
	}

	for i := range studies {
		studies[i].SecretKey = ""
		studies[i].Rules = nil
		studies[i].NotificationSubscriptions = nil
	}

	c.JSON(http.StatusOK, gin.H{"studies": studies})
}

type NewStudyReq struct {
	StudyKey             string `json:"studyKey"`
	SecretKey            string `json:"secretKey"`
	IsSystemDefaultStudy bool   `json:"isSystemDefaultStudy"`
}

func (h *HttpEndpoints) createStudy(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	var req NewStudyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("creating new study", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", req.StudyKey))

	// check if study key is URL safe
	if !utils.IsURLSafe(req.StudyKey) {
		slog.Error("study key is not URL safe", slog.String("studyKey", req.StudyKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "study key is not URL safe"})
		return
	}

	if len(req.SecretKey) < MIN_STUDY_SECRET_KEY_LENGTH {
		slog.Error("secret key is too short", slog.String("studyKey", req.StudyKey), slog.Int("length", len(req.SecretKey)))
		c.JSON(http.StatusBadRequest, gin.H{"error": "secret key is too short"})
		return
	}

	study := studyTypes.Study{
		Key:       req.StudyKey,
		SecretKey: req.SecretKey,
		Status:    studyTypes.STUDY_STATUS_INACTIVE,
		Props: studyTypes.StudyProps{
			SystemDefaultStudy: req.IsSystemDefaultStudy,
		},
		Configs: studyTypes.StudyConfigs{
			IdMappingMethod: studyTypes.DEFAULT_ID_MAPPING_METHOD,
			ParticipantFileUploadRule: &studyTypes.Expression{
				Name: "gt",
				Data: []studyTypes.ExpressionArg{
					{Num: 0, DType: "num"},
					{Num: 1, DType: "num"},
				},
			}, // default rule: file upload is not allowed
		},
	}

	err := h.studyDBConn.CreateStudy(token.InstanceID, study)
	if err != nil {
		slog.Error("failed to create study", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create study"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"study": study})
}

func (h *HttpEndpoints) getStudyProps(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("getting study props", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	study, err := h.studyDBConn.GetStudy(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"study": study})
}

type studyConfigWriter struct {
	Encoder *json.Encoder
	Writer  gin.ResponseWriter
}

func (w *studyConfigWriter) WriteString(text string) {
	if _, err := w.Writer.WriteString(text); err != nil {
		slog.Error("could not write simple string into config export", slog.String("text", text), slog.String("error", err.Error()))
	}
}

func (w *studyConfigWriter) Start() {
	w.WriteString("{")
}
func (w *studyConfigWriter) Finish() {
	w.WriteString("}")
}

func (w *studyConfigWriter) WriteKeyValue(key string, value interface{}) {
	if _, err := w.Writer.WriteString(fmt.Sprintf("\"%s\":", key)); err != nil {
		slog.Error("could not write key into config export", slog.String("key", key), slog.String("error", err.Error()))
		return
	}
	if err := w.Encoder.Encode(value); err != nil {
		slog.Error("could not write value into config export", slog.String("key", key), slog.String("error", err.Error()))
		return
	}
}

func (h *HttpEndpoints) exportStudyConfig(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	includeConfig := c.DefaultQuery("config", "false") == "true"
	includeSurveys := c.DefaultQuery("surveys", "false") == "true"
	includeRules := c.DefaultQuery("rules", "false") == "true"

	slog.Info("exporting study config", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.Bool("includeConfig", includeConfig), slog.Bool("includeSurveys", includeSurveys), slog.Bool("includeRules", includeRules))

	if !includeConfig && !includeSurveys && !includeRules {
		msg := "at least one of the following query parameters must be set: config, surveys, rules"
		slog.Error(msg)
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	study, err := h.studyDBConn.GetStudy(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study"})
		return
	}

	// Set headers for file download
	filename := fmt.Sprintf("study_config_%s_%s.json", studyKey, time.Now().Format("2006-01-02"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/json")
	c.Header("Transfer-Encoding", "chunked")

	// Create a streaming JSON encoder that writes directly to the response
	configWriter := studyConfigWriter{
		Writer:  c.Writer,
		Encoder: json.NewEncoder(c.Writer),
	}

	// Begin the JSON object
	configWriter.Start()
	configWriter.WriteKeyValue("exportedAt", time.Now())

	if includeConfig {
		study.ID = bson.NilObjectID
		study.NextTimerEvent = 0
		study.Stats = studyTypes.StudyStats{}
		configWriter.WriteString(",")
		configWriter.WriteKeyValue("config", study)
	}

	if includeRules {

		rules, err := h.studyDBConn.GetCurrentStudyRules(token.InstanceID, studyKey)
		if err == nil {
			rules.ID = bson.NilObjectID
			rules.UploadedBy = ""
			configWriter.WriteString(",")
			configWriter.WriteKeyValue("rules", rules)
		} else {
			slog.Error("failed to get rules for study", slog.String("error", err.Error()), slog.String("studyKey", studyKey), slog.String("instanceID", token.InstanceID))
		}
	}

	if includeSurveys {
		surveyKeys, err := h.studyDBConn.GetSurveyKeysForStudy(token.InstanceID, studyKey, true)
		if err == nil {
			surveys := []*studyTypes.Survey{}
			for _, surveyKey := range surveyKeys {
				survey, err := h.studyDBConn.GetCurrentSurveyVersion(token.InstanceID, studyKey, surveyKey)
				if err != nil {
					slog.Error("failed to get latest survey", slog.String("error", err.Error()))
					continue
				}
				survey.ID = bson.NilObjectID
				surveys = append(surveys, survey)
			}

			configWriter.WriteString(",")
			configWriter.WriteKeyValue("surveys", surveys)
		} else {
			slog.Error("failed to get survey infos for study", slog.String("error", err.Error()), slog.String("studyKey", studyKey), slog.String("instanceID", token.InstanceID))
		}
	}

	// Close the JSON object
	configWriter.Finish()
}

type StudyIsDefaultUpdateReq struct {
	IsDefault bool `json:"isDefault"`
}

func (h *HttpEndpoints) updateStudyIsDefault(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var req StudyIsDefaultUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("updating study is default", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.Bool("isDefault", req.IsDefault))

	err := h.studyDBConn.UpdateStudyIsDefault(token.InstanceID, studyKey, req.IsDefault)
	if err != nil {
		slog.Error("failed to update study is default", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update study is default"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study is default updated"})
}

type StudyStatusUpdateReq struct {
	Status string `json:"status"`
}

func (h *HttpEndpoints) updateStudyStatus(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var req StudyStatusUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("updating study status", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("status", req.Status))

	err := h.studyDBConn.UpdateStudyStatus(token.InstanceID, studyKey, req.Status)
	if err != nil {
		slog.Error("failed to update study status", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update study status"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "study status updated"})
}

type StudyDisplayPropsUpdateReq struct {
	Name        []studyTypes.LocalisedObject `bson:"name" json:"name"`
	Description []studyTypes.LocalisedObject `bson:"description" json:"description"`
	Tags        []studyTypes.Tag             `bson:"tags" json:"tags"`
}

func (h *HttpEndpoints) updateStudyDisplayProps(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var req StudyDisplayPropsUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("updating study display props", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	err := h.studyDBConn.UpdateStudyDisplayProps(token.InstanceID, studyKey, req.Name, req.Description, req.Tags)
	if err != nil {
		slog.Error("failed to update study display props", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update study display props"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "study display props updated"})
}

type FileUploadRuleUpdateReq struct {
	SimplifiedAllow bool                   `json:"simplifiedAllowedUpload"`
	Expression      *studyTypes.Expression `json:"expression,omitempty"`
}

func (h *HttpEndpoints) updateStudyFileUploadRule(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var req FileUploadRuleUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	newRule := req.Expression
	if newRule == nil {
		if req.SimplifiedAllow {
			newRule = &studyTypes.Expression{
				Name: "gt",
				Data: []studyTypes.ExpressionArg{
					{Num: 1, DType: "num"},
					{Num: 0, DType: "num"},
				},
			}
		} else {
			newRule = &studyTypes.Expression{
				Name: "gt",
				Data: []studyTypes.ExpressionArg{
					{Num: 0, DType: "num"},
					{Num: 1, DType: "num"},
				},
			}
		}
	}

	slog.Info("updating study file upload rule", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	err := h.studyDBConn.UpdateStudyFileUploadRule(token.InstanceID, studyKey, newRule)
	if err != nil {
		slog.Error("failed to update study file upload rule", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update study file upload rule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "study file upload rule updated"})
}

type StudyTrackAccountUpdateReq struct {
	TrackAccount bool `json:"trackAccount"`
}

func (h *HttpEndpoints) updateStudyTrackAccount(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var req StudyTrackAccountUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("updating study track account", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.Bool("trackAccount", req.TrackAccount))

	err := h.studyDBConn.UpdateStudyTrackAccount(token.InstanceID, studyKey, req.TrackAccount)
	if err != nil {
		slog.Error("failed to update study track account", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update study track account"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "study track account updated"})
}

func (h *HttpEndpoints) deleteStudy(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("deleting study", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	err := h.studyDBConn.DeleteStudy(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to delete study", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete study"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study deleted"})
}

type SurveyInfo struct {
	Key string `json:"key"`
}

func (h *HttpEndpoints) getSurveyInfoList(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("getting survey info list", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	surveyKeys, err := h.studyDBConn.GetSurveyKeysForStudy(token.InstanceID, studyKey, true)
	if err != nil {
		slog.Error("failed to get survey info list", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey info list"})
		return
	}

	surveyInfos := make([]SurveyInfo, len(surveyKeys))
	for i, key := range surveyKeys {
		surveyInfos[i] = SurveyInfo{Key: key}
	}

	c.JSON(http.StatusOK, gin.H{"surveys": surveyInfos})
}

func (h *HttpEndpoints) createSurvey(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var survey studyTypes.Survey
	if err := c.ShouldBindJSON(&survey); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	survey.SurveyKey = survey.SurveyDefinition.Key

	slog.Info("creating survey", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", survey.SurveyDefinition.Key))

	surveyKeys, err := h.studyDBConn.GetSurveyKeysForStudy(token.InstanceID, studyKey, true)
	if err != nil {
		slog.Error("failed to get survey info list", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey info list"})
		return
	}

	for _, key := range surveyKeys {
		if key == survey.SurveyKey {
			slog.Error("survey key already exists", slog.String("key", survey.SurveyKey))
			c.JSON(http.StatusBadRequest, gin.H{"error": "survey key already exists"})
			return
		}
	}

	if survey.VersionID == "" {
		surveyHistory, err := h.studyDBConn.GetSurveyVersions(token.InstanceID, studyKey, survey.SurveyKey)
		if err != nil {
			slog.Error("failed to get survey versions", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey versions"})
			return
		}
		survey.VersionID = utils.GenerateSurveyVersionID(surveyHistory)
	}

	survey.Published = time.Now().Unix()

	err = h.studyDBConn.SaveSurveyVersion(token.InstanceID, studyKey, &survey)
	if err != nil {
		slog.Error("failed to create survey", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create survey"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"survey": survey})
}

func (h *HttpEndpoints) getLatestSurvey(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	surveyKey := c.Param("surveyKey")

	slog.Info("getting latest survey", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey))

	survey, err := h.studyDBConn.GetCurrentSurveyVersion(token.InstanceID, studyKey, surveyKey)
	if err != nil {
		slog.Error("failed to get latest survey", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get latest survey"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"survey": survey})
}

func (h *HttpEndpoints) updateSurvey(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	surveyKey := c.Param("surveyKey")

	var survey studyTypes.Survey
	if err := c.ShouldBindJSON(&survey); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	survey.SurveyKey = survey.SurveyDefinition.Key

	if survey.SurveyKey != surveyKey {
		slog.Error("survey key in request does not match", slog.String("key", survey.SurveyKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "survey key in request does not match"})
		return
	}

	if survey.VersionID == "" {
		surveyHistory, err := h.studyDBConn.GetSurveyVersions(token.InstanceID, studyKey, survey.SurveyKey)
		if err != nil {
			slog.Error("failed to get survey versions", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey versions"})
			return
		}
		survey.VersionID = utils.GenerateSurveyVersionID(surveyHistory)
	}

	survey.Published = time.Now().Unix()

	slog.Info("updating survey", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey))

	err := h.studyDBConn.SaveSurveyVersion(token.InstanceID, studyKey, &survey)
	if err != nil {
		slog.Error("failed to update survey", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update survey"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"survey": survey})
}

func (h *HttpEndpoints) unpublishSurvey(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	surveyKey := c.Param("surveyKey")

	slog.Info("unpublishing survey", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey))

	err := h.studyDBConn.UnpublishSurvey(token.InstanceID, studyKey, surveyKey)
	if err != nil {
		slog.Error("failed to unpublish survey", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unpublish survey"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "survey unpublished"})
}

func (h *HttpEndpoints) getSurveyVersions(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	surveyKey := c.Param("surveyKey")

	slog.Info("getting survey versions", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey))

	versions, err := h.studyDBConn.GetSurveyVersions(token.InstanceID, studyKey, surveyKey)
	if err != nil {
		slog.Error("failed to get survey versions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey versions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

func (h *HttpEndpoints) getSurveyVersion(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	surveyKey := c.Param("surveyKey")
	versionID := c.Param("versionID")

	slog.Info("getting survey version", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey), slog.String("versionID", versionID))

	version, err := h.studyDBConn.GetSurveyVersion(token.InstanceID, studyKey, surveyKey, versionID)

	if err != nil {
		slog.Error("failed to get survey version", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey version"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"survey": version})
}

func (h *HttpEndpoints) deleteSurveyVersion(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	surveyKey := c.Param("surveyKey")
	versionID := c.Param("versionID")

	slog.Info("deleting survey version", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey), slog.String("versionID", versionID))

	err := h.studyDBConn.DeleteSurveyVersion(token.InstanceID, studyKey, surveyKey, versionID)
	if err != nil {
		slog.Error("failed to delete survey version", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete survey version"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "survey version deleted"})
}

type StudyUserPermissionInfo struct {
	User        *managementuser.ManagementUser `json:"user"`
	Permissions []managementuser.Permission    `json:"permissions"`
}

func (h *HttpEndpoints) getStudyPermissions(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	permissions, err := h.muDBConn.GetPermissionByResource(token.InstanceID, pc.RESOURCE_TYPE_STUDY, studyKey)
	if err != nil {
		slog.Error("failed to get study permissions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study permissions"})
		return
	}

	// check if user has "manage study permissions" permission
	// or is admin
	allowedToManagePermissions := false
	if token.IsAdmin {
		allowedToManagePermissions = true
	} else {
		for _, permission := range permissions {
			if permission.SubjectID == token.Subject &&
				permission.SubjectType == pc.SUBJECT_TYPE_MANAGEMENT_USER &&
				permission.Action == pc.ACTION_MANAGE_STUDY_PERMISSIONS {
				allowedToManagePermissions = true
				break
			}
		}
	}

	studyUserPermissionInfos := map[string]*StudyUserPermissionInfo{}

	for _, permission := range permissions {
		userID := permission.SubjectID

		if permission.SubjectType != pc.SUBJECT_TYPE_MANAGEMENT_USER {
			continue
		}

		var user *managementuser.ManagementUser

		// Check if user ID already exists in the map
		_, ok := studyUserPermissionInfos[userID]
		if !ok {
			// Get user info
			var err error
			user, err = h.muDBConn.GetUserByID(token.InstanceID, permission.SubjectID)
			if err != nil {
				slog.Error("failed to get user info", slog.String("error", err.Error()))
				continue
			}
			studyUserPermissionInfos[userID] = &StudyUserPermissionInfo{
				User: &managementuser.ManagementUser{
					ID:       user.ID,
					Username: user.Username,
					Email:    user.Email,
					ImageURL: user.ImageURL,
				},
				Permissions: []managementuser.Permission{},
			}
		}

		if allowedToManagePermissions {
			studyUserPermissionInfos[userID].Permissions = append(studyUserPermissionInfos[userID].Permissions, *permission)
		}
	}

	c.JSON(http.StatusOK, gin.H{"permissions": studyUserPermissionInfos})
}

func (h *HttpEndpoints) addStudyPermission(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var permission managementuser.Permission
	if err := c.ShouldBindJSON(&permission); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("adding study permission", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("subjectID", permission.SubjectID), slog.String("action", permission.Action))

	permission.SubjectType = pc.SUBJECT_TYPE_MANAGEMENT_USER
	permission.ResourceType = pc.RESOURCE_TYPE_STUDY
	permission.ResourceKey = studyKey

	_, err := h.muDBConn.CreatePermission(
		token.InstanceID,
		permission.SubjectID,
		permission.SubjectType,
		permission.ResourceType,
		permission.ResourceKey,
		permission.Action,
		permission.Limiter,
	)

	if err != nil {
		slog.Error("failed to add study permission", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add study permission"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study permission added"})
}

func (h *HttpEndpoints) deleteStudyPermission(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	permissionID := c.Param("permissionID")

	slog.Info("deleting study permission", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("permissionID", permissionID))

	permission, err := h.muDBConn.GetPermissionByID(token.InstanceID, permissionID)
	if err != nil {
		slog.Error("failed to get study permission", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study permission"})
		return
	}

	if permission.ResourceType != pc.RESOURCE_TYPE_STUDY || permission.ResourceKey != studyKey {
		slog.Warn("permission does not belong to the study", slog.String("permissionID", permissionID))
		c.JSON(http.StatusBadRequest, gin.H{"error": "permission does not belong to the study"})
		return
	}

	err = h.muDBConn.DeletePermission(token.InstanceID, permissionID)
	if err != nil {
		slog.Error("failed to delete study permission", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete study permission"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study permission deleted"})
}

func (h *HttpEndpoints) getNotificationSubscriptions(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("getting notification subscriptions", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	subscriptions, err := h.studyDBConn.GetNotificationSubscriptions(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get notification subscriptions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get notification subscriptions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscriptions": subscriptions})
}

type NotificationSubscriptionsUpdateReq struct {
	Subscriptions []studyTypes.NotificationSubscription `json:"subscriptions"`
}

func (h *HttpEndpoints) updateNotificationSubscriptions(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var req NotificationSubscriptionsUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("updating notification subscriptions", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	err := h.studyDBConn.UpdateStudyNotificationSubscriptions(token.InstanceID, studyKey, req.Subscriptions)
	if err != nil {
		slog.Error("failed to update notification subscriptions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update notification subscriptions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "notification subscriptions updated"})
}

func (h *HttpEndpoints) getStudyCodeListKeysHandler(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("getting study code list keys", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	keys, err := h.studyDBConn.GetUniqueStudyCodeListKeysForStudy(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study code list keys", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study code list keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"listKeys": keys})
}

func (h *HttpEndpoints) getStudyCodeListEntriesHandler(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	listKey := c.DefaultQuery("listKey", "")

	query, err := apihelpers.ParsePaginatedQueryFromCtx(c)
	if err != nil {
		slog.Error("failed to parse query", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if query == nil {
		slog.Error("failed to parse query", slog.String("error", "query is nil"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if studyKey == "" || listKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "studyKey and listKey must be provided"})
		return
	}

	slog.Info("getting study code list entries", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("listKey", listKey))

	entries, paginationInfo, err := h.studyDBConn.GetStudyCodeListEntries(token.InstanceID, studyKey, listKey, query.Page, query.Limit)
	if err != nil {
		slog.Error("failed to get study code list entries", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("listKey", listKey), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"codeList": entries, "pagination": paginationInfo})
}

type AddStudyCodeListEntriesRequest struct {
	ListKey string   `json:"listKey"`
	Codes   []string `json:"codes"`
}

func (h *HttpEndpoints) addStudyCodeListEntriesHandler(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	if studyKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "studyKey is required"})
		return
	}

	var req AddStudyCodeListEntriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.ListKey = strings.TrimSpace(req.ListKey)
	if req.ListKey == "" {
		slog.Error("list key is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("adding new study code list entries", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("listKey", req.ListKey))

	errors := []string{}
	for _, code := range req.Codes {
		code = strings.TrimSpace(code)
		if code == "" {
			errors = append(errors, "empty code cannot be added")
			continue
		}

		err := h.studyDBConn.AddStudyCodeListEntry(token.InstanceID, studyKey, req.ListKey, code)
		if err != nil {
			slog.Error("failed to add study code list entry", slog.String("error", err.Error()))
			errors = append(errors, fmt.Sprintf("failed to add study code list entry '%s': %s", code, err.Error()))
			continue
		}
	}

	c.JSON(http.StatusOK, gin.H{"errors": errors})
}

func (h *HttpEndpoints) removeStudyCodeListEntryHandler(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := strings.TrimSpace(c.Param("studyKey"))
	listKey := strings.TrimSpace(c.DefaultQuery("listKey", ""))
	code := strings.TrimSpace(c.DefaultQuery("code", ""))

	if studyKey == "" || listKey == "" {
		slog.Error("Missing required parameters", slog.String("studyKey", studyKey), slog.String("listKey", listKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters"})
		return
	}

	if code == "" {
		// remove full list
		slog.Info("deleting study code list (full)", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("listKey", listKey))
		err := h.studyDBConn.DeleteStudyCodeListEntries(token.InstanceID, studyKey, listKey)
		if err != nil {
			slog.Error("Error deleting study code list", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		slog.Info("deleting study code list entry", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("listKey", listKey), slog.String("code", code))

		err := h.studyDBConn.DeleteStudyCodeListEntry(token.InstanceID, studyKey, listKey, code)
		if err != nil {
			slog.Error("Error deleting study code list entry", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *HttpEndpoints) getStudyCounterValues(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")

	slog.Info("getting study counter values", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	values, err := h.studyDBConn.GetAllStudyCounterValues(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study counter values", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study counter values"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"values": values})
}

func (h *HttpEndpoints) saveStudyCounterValue(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")
	scope := strings.TrimSpace(c.Param("scope"))

	if scope == "" {
		slog.Error("scope is required", slog.String("studyKey", studyKey), slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject))
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope is required"})
		return
	}

	var req struct {
		Value int64 `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("saving study counter value", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("scope", scope), slog.String("value", fmt.Sprintf("%d", req.Value)))

	value, err := h.studyDBConn.SaveStudyCounterValue(token.InstanceID, studyKey, scope, req.Value)
	if err != nil {
		slog.Error("failed to save study counter value", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save study counter value"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"value": value})
}

func (h *HttpEndpoints) incrementStudyCounter(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	scope := strings.TrimSpace(c.Param("scope"))

	if scope == "" {
		slog.Error("scope is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope is required"})
		return
	}

	slog.Info("incrementing study counter", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("scope", scope))

	value, err := h.studyDBConn.IncrementAndGetStudyCounterValue(token.InstanceID, studyKey, scope)
	if err != nil {
		slog.Error("failed to increment study counter", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to increment study counter"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"value": value})
}

func (h *HttpEndpoints) removeStudyCounter(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	scope := strings.TrimSpace(c.Param("scope"))

	if scope == "" {
		slog.Error("scope is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope is required"})
		return
	}

	slog.Info("removing study counter", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("scope", scope))

	err := h.studyDBConn.RemoveStudyCounterValue(token.InstanceID, studyKey, scope)
	if err != nil {
		slog.Error("failed to remove study counter", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove study counter"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *HttpEndpoints) getStudyVariables(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")

	slog.Info("getting study variables", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	variables, err := h.studyDBConn.GetStudyVariablesByStudyKey(token.InstanceID, studyKey, false)
	if err != nil {
		slog.Error("failed to get study variables", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study variables"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"variables": variables})
}

func (h *HttpEndpoints) getStudyVariable(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")
	variableKey := c.Param("variableKey")

	if studyKey == "" || variableKey == "" {
		slog.Error("studyKey and variableKey are required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "studyKey and variableKey are required"})
		return
	}

	slog.Info("getting study variable", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("variableKey", variableKey))

	variable, err := h.studyDBConn.GetStudyVariableByStudyKeyAndKey(token.InstanceID, studyKey, variableKey, false)
	if err != nil {
		slog.Error("failed to get study variable", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study variable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"variable": variable})
}

type AddStudyVariableRequest struct {
	VariableDef studyTypes.StudyVariables `json:"variableDef"`
}

func (h *HttpEndpoints) addStudyVariable(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")

	if studyKey == "" {
		slog.Error("studyKey is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "studyKey is required"})
		return
	}

	var req AddStudyVariableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.VariableDef.StudyKey = studyKey

	slog.Info("creating study variable", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("variableKey", req.VariableDef.Key))

	id, err := h.studyDBConn.CreateStudyVariable(token.InstanceID, req.VariableDef)
	if err != nil {
		slog.Error("failed to create study variable", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create study variable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id})
}

type UpdateStudyVariableDefRequest struct {
	VariableDef studyTypes.StudyVariables `json:"variableDef"` // ignore value and key fields
}

func (h *HttpEndpoints) updateStudyVariableDef(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")
	variableKey := c.Param("variableKey")

	if studyKey == "" || variableKey == "" {
		slog.Error("studyKey and variableKey are required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "studyKey and variableKey are required"})
		return
	}

	var req UpdateStudyVariableDefRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("updating study variable definition", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("variableKey", variableKey))

	_, err := h.studyDBConn.UpdateStudyVariableConfig(token.InstanceID, studyKey, variableKey, req.VariableDef.Label, req.VariableDef.Description, req.VariableDef.UIType, req.VariableDef.UIPriority, req.VariableDef.Configs)
	if err != nil {
		slog.Error("failed to update study variable definition", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update study variable definition"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study variable definition updated"})
}

type UpdateStudyVariableValueRequest struct {
	Variable studyTypes.StudyVariables `json:"variable"` // only value is updated
}

func (h *HttpEndpoints) updateStudyVariableValue(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")
	variableKey := c.Param("variableKey")

	if studyKey == "" || variableKey == "" {
		slog.Error("studyKey and variableKey are required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "studyKey and variableKey are required"})
		return
	}

	var req UpdateStudyVariableValueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("updating study variable value", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("variableKey", variableKey))

	_, err := h.studyDBConn.UpdateStudyVariableValue(token.InstanceID, studyKey, variableKey, req.Variable.Value)
	if err != nil {
		slog.Error("failed to update study variable value", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update study variable value"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study variable value updated"})
}

func (h *HttpEndpoints) deleteStudyVariable(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")
	variableKey := c.Param("variableKey")

	if studyKey == "" || variableKey == "" {
		slog.Error("studyKey and variableKey are required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "studyKey and variableKey are required"})
		return
	}

	slog.Info("deleting study variable", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("variableKey", variableKey))

	err := h.studyDBConn.DeleteStudyVariableByStudyKeyAndKey(token.InstanceID, studyKey, variableKey)
	if err != nil {
		slog.Error("failed to delete study variable", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete study variable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *HttpEndpoints) getCurrentStudyRules(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("getting current study rules", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	rules, err := h.studyDBConn.GetCurrentStudyRules(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get current study rules", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get current study rules"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"studyRules": rules})
}

func (h *HttpEndpoints) publishNewStudyRulesVersion(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var rules studyTypes.StudyRules
	if err := c.ShouldBindJSON(&rules); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	rules.StudyKey = studyKey
	rules.UploadedAt = time.Now().Unix()
	rules.UploadedBy = token.Subject

	err := rules.MarshalRules()
	if err != nil {
		slog.Error("failed to marshal study rules", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid study rules"})
		return
	}

	slog.Info("publishing new study rules version", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	err = h.studyDBConn.SaveStudyRules(token.InstanceID, studyKey, rules)
	if err != nil {
		slog.Error("failed to publish new study rules version", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish new study rules version"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "new study rules version published"})
}

func (h *HttpEndpoints) getStudyRuleVersions(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("getting study rule versions", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	versions, err := h.studyDBConn.GetStudyRulesHistory(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study rule versions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study rule versions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"versions": versions})
}

func (h *HttpEndpoints) getStudyRuleVersion(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	versionID := c.Param("id")

	slog.Info("getting study rule version", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("versionID", versionID))

	version, err := h.studyDBConn.GetStudyRulesByID(token.InstanceID, studyKey, versionID)
	if err != nil {
		slog.Error("failed to get study rule version", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study rule version"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"studyRules": version})
}

func (h *HttpEndpoints) deleteStudyRuleVersion(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	versionID := c.Param("id")

	slog.Info("deleting study rule version", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("versionID", versionID))

	err := h.studyDBConn.DeleteStudyRulesByID(token.InstanceID, studyKey, versionID)
	if err != nil {
		slog.Error("failed to delete study rule version", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete study rule version"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study rule version deleted"})
}

func (h *HttpEndpoints) runActionOnParticipant(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")

	var req struct {
		Rules []studyTypes.Expression `json:"rules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("running study action on participant", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("participantID", participantID))

	result, err := studyService.OnRunStudyAction(studyService.RunStudyActionReq{
		InstanceID:           token.InstanceID,
		StudyKey:             studyKey,
		OnlyForParticipantID: participantID,
		Rules:                req.Rules,
		OnProgressFn:         nil,
	})

	if err != nil {
		slog.Error("failed to run study action", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"participantCount": result.ParticipantCount,
		"duration":         result.Duration,
		"ruleResults":      result.ParticipantStateChangedPerRule,
	})
}

func (h *HttpEndpoints) taskFailed(
	instanceID string,
	taskID string,
	errMsg string,
) {
	err := h.studyDBConn.UpdateTaskCompleted(
		instanceID,
		taskID,
		studyTypes.TASK_STATUS_COMPLETED,
		0,
		errMsg,
		"",
	)
	if err != nil {
		slog.Error("failed to update task status on faied task", slog.String("error", err.Error()))
		return
	}
}

func (h *HttpEndpoints) onActionTaskCompleted(
	taskID string,
	results *studyService.RunStudyActionResult,
	err error,
	instanceID string,
	relativeFolderName string,
) {
	if err != nil {
		slog.Error("failed to run study actions", slog.String("error", err.Error()))
		h.taskFailed(instanceID, taskID, err.Error())
		return
	}

	// create file write
	relativeFilepath := filepath.Join(relativeFolderName, "results_"+taskID+".json")
	exportFilePath := filepath.Join(h.filestorePath, relativeFilepath)
	file, err := os.Create(exportFilePath)
	if err != nil {
		slog.Error("failed to create action run results file", slog.String("error", err.Error()))
		h.taskFailed(instanceID, taskID, err.Error())
		return
	}
	defer file.Close()

	// write to json
	err = json.NewEncoder(file).Encode(results)
	if err != nil {
		slog.Error("failed to write to action run results file", slog.String("error", err.Error()))
		h.taskFailed(instanceID, taskID, err.Error())
		return
	}

	err = h.studyDBConn.UpdateTaskCompleted(
		instanceID,
		taskID,
		studyTypes.TASK_STATUS_COMPLETED,
		int(results.ParticipantCount),
		"",
		relativeFilepath,
	)
	if err != nil {
		slog.Error("failed to update task status", slog.String("error", err.Error()))
		return
	}
}

func (h *HttpEndpoints) runActionOnParticipants(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")

	var req struct {
		Rules []studyTypes.Expression `json:"rules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("running study action on participants", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	relativeFolderName := filepath.Join(token.InstanceID, "actionRuns")
	exportFolder := filepath.Join(h.filestorePath, relativeFolderName)
	if err := os.MkdirAll(exportFolder, os.ModePerm); err != nil {
		slog.Error("failed to create actionRuns folder", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create actionRuns folder"})
		return
	}

	task, err := h.studyDBConn.CreateTask(
		token.InstanceID,
		token.Subject,
		10000000000000, // just a large number, should be updated in next step
		studyTypes.TASK_FILE_TYPE_JSON,
	)
	if err != nil {
		slog.Error("failed to create task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	go func() {
		first := true

		results, err := studyService.OnRunStudyAction(studyService.RunStudyActionReq{
			InstanceID: token.InstanceID,
			StudyKey:   studyKey,
			Rules:      req.Rules,
			OnProgressFn: func(totalCount int64, processedCount int64) {
				if first {
					err = h.studyDBConn.UpdateTaskTotalCount(
						token.InstanceID,
						task.ID.Hex(),
						int(totalCount),
					)
					if err != nil {
						slog.Error("failed to update task total count", slog.String("error", err.Error()))
						return
					}
					first = false
				}

				err := h.studyDBConn.UpdateTaskProgress(
					token.InstanceID,
					task.ID.Hex(),
					int(processedCount),
				)
				if err != nil {
					slog.Error("failed to update task progress", slog.String("error", err.Error()))
					// not a big issue, so let's try next time
					return
				}
			},
		})
		if err != nil {
			slog.Error("running study actions resulted in error", slog.String("error", err.Error()))
			return
		}

		h.onActionTaskCompleted(task.ID.Hex(), results, err, token.InstanceID, relativeFolderName)

	}()

	c.JSON(http.StatusOK, gin.H{"task": task})
}

func (h *HttpEndpoints) getStudyActionTaskStatus(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	taskID := c.Param("taskID")

	slog.Info("getting study action task status", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("taskID", taskID))

	task, err := h.studyDBConn.GetTaskByID(token.InstanceID, taskID)
	if err != nil {
		slog.Error("failed to get export task status", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get export task status"})
		return
	}

	if task.CreatedBy != token.Subject && !token.IsAdmin {
		slog.Warn("user is not allowed to get task status", slog.String("userID", token.Subject), slog.String("taskID", taskID))
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task": task})
}

func (h *HttpEndpoints) getStudyActionTaskResult(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	taskID := c.Param("taskID")

	slog.Info("getting export task result", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("taskID", taskID))

	task, err := h.studyDBConn.GetTaskByID(token.InstanceID, taskID)
	if err != nil {
		slog.Error("failed to get export task result", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get export task result"})
		return
	}

	if task.CreatedBy != token.Subject && !token.IsAdmin {
		slog.Warn("user is not allowed to get task result", slog.String("userID", token.Subject), slog.String("taskID", taskID))
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if task.Status != studyTypes.TASK_STATUS_COMPLETED {
		slog.Error("task is not completed", slog.String("taskID", taskID), slog.String("status", task.Status))
		c.JSON(http.StatusBadRequest, gin.H{"error": "task is not completed"})
		return
	}

	resultFilePath := filepath.Join(h.filestorePath, task.ResultFile)

	// file exists?
	if _, err := os.Stat(resultFilePath); os.IsNotExist(err) {
		slog.Error("file does not exist", slog.String("path", resultFilePath))
		c.JSON(http.StatusNotFound, gin.H{"error": "file does not exist"})
		return
	}

	// read JSON file and send back
	file, err := os.Open(resultFilePath)
	if err != nil {
		slog.Error("failed to open file", slog.String("path", resultFilePath), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open file"})
		return
	}
	defer file.Close()

	var result map[string]interface{}
	err = json.NewDecoder(file).Decode(&result)
	if err != nil {
		slog.Error("failed to decode JSON file", slog.String("path", resultFilePath), slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode JSON file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": result})
}

func (h *HttpEndpoints) runActionOnPreviousResponsesForParticipant(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")

	var req struct {
		SurveyKeys []string                `json:"surveyKeys"`
		From       int64                   `json:"from"`
		To         int64                   `json:"to"`
		Rules      []studyTypes.Expression `json:"rules"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("running study action on previous responses for participant", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("participantID", participantID))

	result, err := studyService.OnRunStudyActionForPreviousResponses(studyService.RunStudyActionReq{
		InstanceID:           token.InstanceID,
		StudyKey:             studyKey,
		OnlyForParticipantID: participantID,
		Rules:                req.Rules,
		OnProgressFn:         nil,
	}, req.SurveyKeys, req.From, req.To)

	if err != nil {
		slog.Error("failed to run study action", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"participantCount": result.ParticipantCount,
		"duration":         result.Duration,
		"ruleResults":      result.ParticipantStateChangedPerRule,
	})
}

func (h *HttpEndpoints) runActionOnPreviousResponsesForParticipants(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")

	var req struct {
		Rules      []studyTypes.Expression `json:"rules"`
		SurveyKeys []string                `json:"surveyKeys"`
		From       int64                   `json:"from"`
		To         int64                   `json:"to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("running study action on participants", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	relativeFolderName := filepath.Join(token.InstanceID, "actionRuns")
	exportFolder := filepath.Join(h.filestorePath, relativeFolderName)
	if err := os.MkdirAll(exportFolder, os.ModePerm); err != nil {
		slog.Error("failed to create actionRuns folder", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create actionRuns folder"})
		return
	}

	task, err := h.studyDBConn.CreateTask(
		token.InstanceID,
		token.Subject,
		10000000000000, // just a large number, should be updated in next step
		studyTypes.TASK_FILE_TYPE_JSON,
	)
	if err != nil {
		slog.Error("failed to create task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	go func() {
		first := true

		results, err := studyService.OnRunStudyActionForPreviousResponses(
			studyService.RunStudyActionReq{
				InstanceID: token.InstanceID,
				StudyKey:   studyKey,
				Rules:      req.Rules,
				OnProgressFn: func(totalCount int64, processedCount int64) {
					if first {
						err = h.studyDBConn.UpdateTaskTotalCount(
							token.InstanceID,
							task.ID.Hex(),
							int(totalCount),
						)
						if err != nil {
							slog.Error("failed to update task total count", slog.String("error", err.Error()))
							return
						}
						first = false
					}

					err := h.studyDBConn.UpdateTaskProgress(
						token.InstanceID,
						task.ID.Hex(),
						int(processedCount),
					)
					if err != nil {
						slog.Error("failed to update task progress", slog.String("error", err.Error()))
						// not a big issue, so let's try next time
						return
					}
				},
			},
			req.SurveyKeys,
			req.From,
			req.To,
		)
		if err != nil {
			slog.Error("running study actions resulted in error", slog.String("error", err.Error()))
			return
		}

		h.onActionTaskCompleted(task.ID.Hex(), results, err, token.InstanceID, relativeFolderName)

	}()

	c.JSON(http.StatusOK, gin.H{"task": task})
}

func (h *HttpEndpoints) getSurveyInfo(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	surveyKey := c.DefaultQuery("surveyKey", "")
	if surveyKey == "" {
		slog.Error("surveyKey is required", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "surveyKey is required"})
		return
	}

	format := c.DefaultQuery("format", "json")
	if format != "json" && format != "csv" {
		slog.Error("invalid format", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey), slog.String("format", format))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid format query parameter"})
		return
	}
	// includeItems, excludeItems
	language := c.DefaultQuery("language", "en")
	shortKeys := c.DefaultQuery("shortKeys", "false") == "true"

	slog.Info("getting survey info", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey))

	sInfos, err := surveydefinition.PrepareSurveyInfosFromDB(
		h.studyDBConn,
		token.InstanceID,
		studyKey,
		surveyKey,
		&surveydefinition.ExtractOptions{
			UseLabelLang: language,
		},
	)
	if err != nil {
		slog.Error("failed to get survey info", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey info"})
		return
	}

	siExp := surveydefinition.NewSurveyInfoExporter(
		sInfos,
		surveyKey,
		shortKeys,
	)

	if format == "json" {
		c.Header("Content-Disposition", `attachment; filename=`+fmt.Sprintf("survey-infos_%s_%s.json", studyKey, surveyKey))
		c.JSON(http.StatusOK, gin.H{"versions": siExp.GetSurveyInfos(), "key": surveyKey})
		return
	}

	// CSV:
	c.Header("Content-Disposition", `attachment; filename=`+fmt.Sprintf("survey-infos_%s_%s.csv", studyKey, surveyKey))
	c.Header("Content-Type", "text/csv")
	err = siExp.GetSurveyInfoCSV(c.Writer)
	if err != nil {
		slog.Error("failed to get survey info csv", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey info csv"})
		return
	}
}

func (h *HttpEndpoints) getResponsesCount(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	filter, err := apihelpers.ParseFilterQueryFromCtx(c)
	if err != nil {
		slog.Error("failed to parse filter", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	filter["key"] = c.DefaultQuery("surveyKey", "")

	slog.Info("getting responses count", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	count, err := h.studyDBConn.GetResponsesCount(token.InstanceID, studyKey, filter)
	if err != nil {
		slog.Error("failed to get responses count", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get responses count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *HttpEndpoints) generateResponsesExport(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")

	query, err := apihelpers.ParseResponseExportQueryFromCtx(c)
	if err != nil || query == nil {
		slog.Error("failed to parse query", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if query.SurveyKey == "" {
		slog.Error("surveyKey is required", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "surveyKey is required"})
		return
	}

	study, err := h.studyDBConn.GetStudy(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study"})
		return
	}

	slog.Info("generating responses export", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", query.SurveyKey))

	count, err := h.studyDBConn.GetResponsesCount(token.InstanceID, studyKey, query.PaginationInfos.Filter)
	if err != nil {
		slog.Error("failed to get responses count", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get responses count"})
		return
	}
	if count == 0 {
		c.JSON(http.StatusOK, gin.H{
			"error": "no responses to export",
		})
		return
	}

	surveyVersions, err := surveydefinition.PrepareSurveyInfosFromDB(
		h.studyDBConn,
		token.InstanceID,
		studyKey,
		query.SurveyKey,
		&surveydefinition.ExtractOptions{
			UseLabelLang: "",
			IncludeItems: nil,
			ExcludeItems: nil,
		},
	)
	if err != nil {
		slog.Error("failed to get survey versions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey versions"})
		return
	}

	respParser, err := surveyresponses.NewResponseParser(
		query.SurveyKey,
		surveyVersions,
		query.UseShortKeys,
		query.IncludeMeta,
		query.QuestionOptionSep,
		query.ExtraCtxCols,
	)
	if err != nil {
		slog.Error("failed to create response parser", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create response parser"})
		return
	}
	if study.Configs.TrackAccount {
		respParser.EnableAccountTracking()
	}

	fileType := studyTypes.TASK_FILE_TYPE_CSV
	if query.Format == "json" {
		fileType = studyTypes.TASK_FILE_TYPE_JSON
	}

	exportTask, err := h.studyDBConn.CreateTask(
		token.InstanceID,
		token.Subject,
		int(count),
		fileType,
	)

	if err != nil {
		slog.Error("failed to create export task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create export task"})
		return
	}

	relativeFolderName := filepath.Join(token.InstanceID, "exports")
	exportFolder := filepath.Join(h.filestorePath, relativeFolderName)
	if err := os.MkdirAll(exportFolder, os.ModePerm); err != nil {
		slog.Error("failed to create export folder", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create export folder"})
		return
	}

	go func() {
		// create file write
		ext := ".csv"
		if query.Format == "json" {
			ext = ".json"
		}

		relativeFilepath := filepath.Join(relativeFolderName, "responses_"+exportTask.ID.Hex()+ext)
		exportFilePath := filepath.Join(h.filestorePath, relativeFilepath)
		file, err := os.Create(exportFilePath)
		if err != nil {
			slog.Error("failed to create export file", slog.String("error", err.Error()))

			h.onExportTaskFailed(token.InstanceID, exportTask.ID.Hex(), "failed to create export file")
			return
		}

		defer file.Close()

		exporter, err := surveyresponses.NewResponseExporter(
			respParser,
			file,
			query.Format,
		)
		if err != nil {
			slog.Error("failed to create response exporter", slog.String("error", err.Error()))

			h.onExportTaskFailed(token.InstanceID, exportTask.ID.Hex(), "failed to create response exporter")
			return
		}

		ctx := context.Background()
		counter := 0
		accountInfoCache := map[string]surveyresponses.AccountTrackingInfo{}

		err = h.studyDBConn.FindAndExecuteOnResponses(
			ctx,
			token.InstanceID,
			studyKey,
			query.PaginationInfos.Filter,
			query.PaginationInfos.Sort,
			true,
			func(dbService *studyDB.StudyDBService, r studyTypes.SurveyResponse, instanceID, studyKey string, args ...interface{}) error {
				task := args[0].(*studyTypes.Task)
				exporter := args[1].(*surveyresponses.ResponseExporter)

				trackingInfo := surveyresponses.AccountTrackingInfo{}
				if study.Configs.TrackAccount {
					var ok bool
					trackingInfo, ok = accountInfoCache[r.ParticipantID]
					if !ok {
						trackingInfo = h.getResponseAccountTrackingInfo(instanceID, studyKey, r.ParticipantID)
						accountInfoCache[r.ParticipantID] = trackingInfo
					}
				}

				err := exporter.WriteResponse(&r, trackingInfo)
				if err != nil {
					return err
				}
				counter += 1

				err = dbService.UpdateTaskProgress(
					instanceID,
					task.ID.Hex(),
					counter,
				)
				if err != nil {
					slog.Error("failed to update task progress", slog.String("error", err.Error()))
					// not a big issue, so let's try next time
					return nil
				}

				return nil
			},
			&exportTask,
			exporter,
		)

		if err != nil {
			slog.Error("failed to export responses", slog.String("error", err.Error()))
			h.onExportTaskFailed(token.InstanceID, exportTask.ID.Hex(), err.Error())
			return
		}

		err = exporter.Finish()
		if err != nil {
			slog.Error("failed to finish export", slog.String("error", err.Error()))
			h.onExportTaskFailed(token.InstanceID, exportTask.ID.Hex(), err.Error())
			return
		}

		err = h.studyDBConn.UpdateTaskCompleted(
			token.InstanceID,
			exportTask.ID.Hex(),
			studyTypes.TASK_STATUS_COMPLETED,
			counter,
			"",
			relativeFilepath,
		)
		if err != nil {
			slog.Error("failed to update task status", slog.String("error", err.Error()))
			return
		}

	}()

	c.JSON(http.StatusOK, gin.H{"task": exportTask})
}

func (h *HttpEndpoints) getParticipantsCount(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	filter, err := apihelpers.ParseFilterQueryFromCtx(c)
	if err != nil {
		slog.Error("failed to parse filter", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("getting participants count", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	count, err := h.studyDBConn.GetParticipantCount(token.InstanceID, studyKey, filter)
	if err != nil {
		slog.Error("failed to get participants count", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get participants count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *HttpEndpoints) generateParticipantsExport(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	filter, err := apihelpers.ParseFilterQueryFromCtx(c)
	if err != nil {
		slog.Error("failed to parse filter", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	sort, err := apihelpers.ParseSortQueryFromCtx(c)
	if err != nil {
		slog.Error("failed to parse sort", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("generating participants export", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	count, err := h.studyDBConn.GetParticipantCount(token.InstanceID, studyKey, filter)
	if err != nil {
		slog.Error("failed to get participants count", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get participants count"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusOK, gin.H{
			"error": "no participants to export",
		})
		return
	}

	exportTask, err := h.studyDBConn.CreateTask(
		token.InstanceID,
		token.Subject,
		int(count),
		studyTypes.TASK_FILE_TYPE_JSON,
	)

	if err != nil {
		slog.Error("failed to create export task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create export task"})
		return
	}

	relativeFolderName := filepath.Join(token.InstanceID, "exports")
	exportFolder := filepath.Join(h.filestorePath, relativeFolderName)
	if err := os.MkdirAll(exportFolder, os.ModePerm); err != nil {
		slog.Error("failed to create export folder", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create export folder"})
		return
	}

	go func() {

		// create file write
		relativeFilepath := filepath.Join(relativeFolderName, "participants_"+exportTask.ID.Hex()+".json")
		exportFilePath := filepath.Join(h.filestorePath, relativeFilepath)
		file, err := os.Create(exportFilePath)
		if err != nil {
			slog.Error("failed to create export file", slog.String("error", err.Error()))

			h.onExportTaskFailed(token.InstanceID, exportTask.ID.Hex(), "failed to create export file")
			return
		}

		defer file.Close()

		_, err = file.WriteString("{\"participants\": [")
		if err != nil {
			slog.Error("failed to write header", slog.String("error", err.Error()))
			h.onExportTaskFailed(token.InstanceID, exportTask.ID.Hex(), "failed to write to export file")
			return
		}

		ctx := context.Background()
		counter := 0

		err = h.studyDBConn.FindAndExecuteOnParticipantsStates(
			ctx,
			token.InstanceID,
			studyKey,
			filter,
			sort,
			true,
			func(dbService *studyDB.StudyDBService, p studyTypes.Participant, instanceID, studyKey string, args ...interface{}) error {
				task := args[0].(*studyTypes.Task)

				if counter > 0 {
					_, err = file.WriteString(",")
					if err != nil {
						slog.Error("failed to write to export file", slog.String("error", err.Error()))
						return err
					}
				}

				// p to JSON
				pJSON, err := json.Marshal(p)
				if err != nil {
					slog.Error("failed to marshal participant", slog.String("error", err.Error()))
					return err
				}
				_, err = file.Write(pJSON)
				if err != nil {
					slog.Error("failed to write to export file", slog.String("error", err.Error()))
					return err
				}

				counter += 1

				err = dbService.UpdateTaskProgress(
					instanceID,
					task.ID.Hex(),
					counter,
				)
				if err != nil {
					slog.Error("failed to update task progress", slog.String("error", err.Error()))
					// not a big issue, so let's try next time
					return nil
				}

				return nil
			},
			&exportTask,
		)
		if err != nil {
			slog.Error("failed to export participants", slog.String("error", err.Error()))
			h.onExportTaskFailed(token.InstanceID, exportTask.ID.Hex(), err.Error())
			return
		}

		_, err = file.WriteString("]}")
		if err != nil {
			slog.Error("failed to write footer", slog.String("error", err.Error()))
			h.onExportTaskFailed(token.InstanceID, exportTask.ID.Hex(), "failed to write to export file")
			return
		}

		err = h.studyDBConn.UpdateTaskCompleted(
			token.InstanceID,
			exportTask.ID.Hex(),
			studyTypes.TASK_STATUS_COMPLETED,
			counter,
			"",
			relativeFilepath,
		)
		if err != nil {
			slog.Error("failed to update task status", slog.String("error", err.Error()))
			return
		}
	}()

	c.JSON(http.StatusOK, gin.H{"task": exportTask})
}

func (h *HttpEndpoints) getReportsCount(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	filter, err := apihelpers.ParseFilterQueryFromCtx(c)
	if err != nil {
		slog.Error("failed to parse filter", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	filter, _, err = h.parseReportQueryParams(c, filter, false)
	if err != nil {
		slog.Error("failed to parse report query params", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("getting reports count", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	count, err := h.studyDBConn.GetReportCountForQuery(token.InstanceID, studyKey, filter)
	if err != nil {
		slog.Error("failed to get reports count", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get reports count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *HttpEndpoints) generateRawJSONReportExport(
	instanceID string,
	studyKey string,
	filter bson.M,
	taskID string,
	relativeFolderName string,
) {
	// create file write
	relativeFilepath := filepath.Join(relativeFolderName, "reports_"+taskID+".json")
	exportFilePath := filepath.Join(h.filestorePath, relativeFilepath)
	file, err := os.Create(exportFilePath)
	if err != nil {
		slog.Error("failed to create export file", slog.String("error", err.Error()))

		h.onExportTaskFailed(instanceID, taskID, "failed to create export file")
		return
	}

	defer file.Close()

	_, err = file.WriteString("{\"reports\": [")
	if err != nil {
		slog.Error("failed to write header", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, "failed to write to export file")
		return
	}

	ctx := context.Background()
	counter := 0

	err = h.studyDBConn.FindAndExecuteOnReports(
		ctx,
		instanceID,
		studyKey,
		filter,
		true,
		func(instanceID, studyKey string, r studyTypes.Report, args ...interface{}) error {
			taskID := args[0].(string)

			if counter > 0 {
				_, err = file.WriteString(",")
				if err != nil {
					slog.Error("failed to write to export file", slog.String("error", err.Error()))
					return err
				}
			}

			// r to JSON
			rJSON, err := json.Marshal(r)
			if err != nil {
				slog.Error("failed to marshal report", slog.String("error", err.Error()))
				return err
			}
			_, err = file.Write(rJSON)
			if err != nil {
				slog.Error("failed to write to export file", slog.String("error", err.Error()))
				return err
			}

			counter += 1

			err = h.studyDBConn.UpdateTaskProgress(
				instanceID,
				taskID,
				counter,
			)
			if err != nil {
				slog.Error("failed to update task progress", slog.String("error", err.Error()))
				// not a big issue, so let's try next time
				return nil
			}
			return nil
		},
		taskID,
	)

	if err != nil {
		slog.Error("failed to export reports", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, err.Error())
		return
	}

	_, err = file.WriteString("]}")
	if err != nil {
		slog.Error("failed to write footer", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, "failed to write to export file")
		return
	}

	err = h.studyDBConn.UpdateTaskCompleted(
		instanceID,
		taskID,
		studyTypes.TASK_STATUS_COMPLETED,
		counter,
		"",
		relativeFilepath,
	)
	if err != nil {
		slog.Error("failed to update task status", slog.String("error", err.Error()))
		return
	}
}

func (h *HttpEndpoints) generateCSVReportExport(
	instanceID string,
	studyKey string,
	filter bson.M,
	taskID string,
	relativeFolderName string,
) {
	const dataKeyPrefix = "data_"
	reservedColumns := []string{"id", "key", "participantID", "timestamp", "responseID"}

	// Stage 1: Write reports to temp file as JSON and collect unique column names
	tempFilePath := filepath.Join(h.filestorePath, relativeFolderName, "temp_reports_"+taskID+".jsonl")
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		slog.Error("failed to create temp file", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, "failed to create temp file")
		return
	}
	defer os.Remove(tempFilePath) // Clean up temp file at the end

	ctx := context.Background()
	counter := 0
	uniqueDataKeys := make(map[string]bool)

	// Helper function to check if a column is reserved
	isReservedColumn := func(key string) bool {
		return slices.Contains(reservedColumns, key)
	}

	// Stage 1: Iterate over DB entries and write as JSON lines
	err = h.studyDBConn.FindAndExecuteOnReports(
		ctx,
		instanceID,
		studyKey,
		filter,
		true,
		func(instanceID, studyKey string, r studyTypes.Report, args ...any) error {
			taskID := args[0].(string)

			// Collect unique data keys
			for _, data := range r.Data {
				dataKey := data.Key
				if isReservedColumn(dataKey) {
					dataKey = dataKeyPrefix + dataKey
				}
				uniqueDataKeys[dataKey] = true
			}

			// Write report as JSON line
			rJSON, err := json.Marshal(r)
			if err != nil {
				slog.Error("failed to marshal report", slog.String("error", err.Error()))
				return err
			}
			_, err = tempFile.Write(rJSON)
			if err != nil {
				slog.Error("failed to write to temp file", slog.String("error", err.Error()))
				return err
			}
			_, err = tempFile.WriteString("\n")
			if err != nil {
				slog.Error("failed to write newline to temp file", slog.String("error", err.Error()))
				return err
			}

			counter += 1

			err = h.studyDBConn.UpdateTaskProgress(
				instanceID,
				taskID,
				counter,
			)
			if err != nil {
				slog.Error("failed to update task progress", slog.String("error", err.Error()))
				// not a big issue, so let's try next time
				return nil
			}
			return nil
		},
		taskID,
	)

	tempFile.Close()

	if err != nil {
		slog.Error("failed to export reports", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, err.Error())
		return
	}

	// Stage 2: Read temp file and write CSV
	relativeFilepath := filepath.Join(relativeFolderName, "reports_"+taskID+".csv")
	exportFilePath := filepath.Join(h.filestorePath, relativeFilepath)
	csvFile, err := os.Create(exportFilePath)
	if err != nil {
		slog.Error("failed to create CSV file", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, "failed to create CSV file")
		return
	}
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Build CSV header - reuse reservedColumns to avoid duplication
	headers := append([]string{}, reservedColumns...)
	// Add sorted data keys for consistent column order
	dataKeys := make([]string, 0, len(uniqueDataKeys))
	for key := range uniqueDataKeys {
		dataKeys = append(dataKeys, key)
	}
	sort.Strings(dataKeys)
	headers = append(headers, dataKeys...)

	// Write header
	if err := writer.Write(headers); err != nil {
		slog.Error("failed to write CSV header", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, "failed to write CSV header")
		return
	}

	// Read temp file and write CSV rows
	tempFileRead, err := os.Open(tempFilePath)
	if err != nil {
		slog.Error("failed to open temp file for reading", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, "failed to open temp file")
		return
	}
	defer tempFileRead.Close()

	scanner := bufio.NewScanner(tempFileRead)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var report studyTypes.Report
		if err := json.Unmarshal(line, &report); err != nil {
			slog.Error("failed to unmarshal report", slog.String("error", err.Error()))
			continue
		}

		// Build data map for quick lookup
		dataMap := make(map[string]string)
		for _, data := range report.Data {
			dataKey := data.Key
			if isReservedColumn(dataKey) {
				dataKey = dataKeyPrefix + dataKey
			}

			// Convert value based on dtype
			value := data.Value
			switch data.Dtype {
			case "date":
				// Validate it's a number
				f64, err := strconv.ParseFloat(value, 64)
				if err == nil {
					dataMap[dataKey] = time.Unix(int64(f64), 0).UTC().Format(time.RFC3339)
				} else {
					dataMap[dataKey] = value
				}
			default:
				dataMap[dataKey] = value
			}
		}

		// Build CSV row
		row := make([]string, len(headers))
		row[0] = report.ID.Hex()
		row[1] = report.Key
		row[2] = report.ParticipantID
		row[3] = time.Unix(report.Timestamp, 0).UTC().Format(time.RFC3339)
		row[4] = report.ResponseID

		// Fill in data columns
		for i, header := range headers[5:] {
			if val, ok := dataMap[header]; ok {
				row[5+i] = val
			} else {
				row[5+i] = ""
			}
		}

		if err := writer.Write(row); err != nil {
			slog.Error("failed to write CSV row", slog.String("error", err.Error()))
			h.onExportTaskFailed(instanceID, taskID, "failed to write CSV row")
			return
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("error reading temp file", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, "error reading temp file")
		return
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		slog.Error("error flushing CSV writer", slog.String("error", err.Error()))
		h.onExportTaskFailed(instanceID, taskID, "error writing CSV")
		return
	}

	err = h.studyDBConn.UpdateTaskCompleted(
		instanceID,
		taskID,
		studyTypes.TASK_STATUS_COMPLETED,
		counter,
		"",
		relativeFilepath,
	)
	if err != nil {
		slog.Error("failed to update task status", slog.String("error", err.Error()))
		return
	}
}

func (h *HttpEndpoints) generateReportsExport(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	filter, err := apihelpers.ParseFilterQueryFromCtx(c)
	if err != nil {
		slog.Error("failed to parse filter", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	filter, exportType, err := h.parseReportQueryParams(c, filter, true)
	if err != nil {
		slog.Error("failed to parse report query params", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Info("generating reports export", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("exportType", exportType))

	count, err := h.studyDBConn.GetReportCountForQuery(token.InstanceID, studyKey, filter)
	if err != nil {
		slog.Error("failed to get reports count", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get reports count"})
		return
	}

	if count == 0 {
		c.JSON(http.StatusOK, gin.H{
			"error": "no reports to export",
		})
		return
	}

	targetCount := int(count)
	taskFileType := studyTypes.TASK_FILE_TYPE_JSON
	if exportType == "csv" {
		taskFileType = studyTypes.TASK_FILE_TYPE_CSV
	}

	exportTask, err := h.studyDBConn.CreateTask(
		token.InstanceID,
		token.Subject,
		targetCount,
		taskFileType,
	)

	if err != nil {
		slog.Error("failed to create export task", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create export task"})
		return
	}

	relativeFolderName := filepath.Join(token.InstanceID, "exports")
	exportFolder := filepath.Join(h.filestorePath, relativeFolderName)
	if err := os.MkdirAll(exportFolder, os.ModePerm); err != nil {
		slog.Error("failed to create export folder", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create export folder"})
		return
	}

	if exportType == "csv" {
		go h.generateCSVReportExport(
			token.InstanceID,
			studyKey,
			filter,
			exportTask.ID.Hex(),
			relativeFolderName,
		)
	} else {
		go h.generateRawJSONReportExport(
			token.InstanceID,
			studyKey,
			filter,
			exportTask.ID.Hex(),
			relativeFolderName,
		)
	}

	c.JSON(http.StatusOK, gin.H{"task": exportTask})
}

type ConfidentialResponsesExportQuery struct {
	ParticipantIDs []string `json:"participantIDs"`
	KeyFilter      string   `json:"keyFilter"`
}

func (h *HttpEndpoints) getConfidentialResponses(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	var query ConfidentialResponsesExportQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		slog.Error("failed to bind request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if len(query.ParticipantIDs) == 0 {
		slog.Error("participantIDs is required", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "participantIDs is required"})
		return
	}

	slog.Info("getting confidential responses", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	study, err := h.studyDBConn.GetStudy(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study"})
		return
	}

	studySecretKey := study.SecretKey
	idMappingMethod := study.Configs.IdMappingMethod
	globalSecret := h.globalStudySecret

	results := []studyutils.ConfidentialResponsesExportEntry{}

	for _, pID := range query.ParticipantIDs {
		// confidentialID := studyTypes.GetConfidentialParticipantID(token.InstanceID, studyKey, pID)
		confidentialID, err := studyutils.ProfileIDtoParticipantID(pID, globalSecret, studySecretKey, idMappingMethod)
		if err != nil {
			slog.Error("failed to get confidential participantID", slog.String("error", err.Error()))
			continue
		}
		slog.Info("getting confidential responses for participant", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("participantID", pID))

		responses, err := h.studyDBConn.FindConfidentialResponses(token.InstanceID, studyKey, confidentialID, query.KeyFilter)
		if err != nil {
			slog.Error("failed to get confidential responses", slog.String("error", err.Error()))
			continue
		}

		for _, r := range responses {
			results = append(results, studyutils.PrepConfidentialResponseExport(r, pID, nil)...)
		}
	}

	c.JSON(http.StatusOK, gin.H{"responses": results})
}

func (h *HttpEndpoints) getAvailableConfidentailDataExports(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("getting available confidential response exports", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	if h.dailyFileExportPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "daily file export path not set"})
		return
	}

	targetFolderPath := filepath.Join(h.dailyFileExportPath, token.InstanceID, studyKey)

	dailyExports := []string{}
	// collect all files

	err := filepath.Walk(targetFolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Parse date from filename (assuming format YYYY-MM-DD##responses##..##..)
		basename := filepath.Base(path)
		parts := strings.Split(basename, "##")
		if len(parts) < 2 {
			return nil
		}
		if parts[1] != "confidential-responses" {
			return nil
		}
		dailyExports = append(dailyExports, basename)
		return nil
	})
	if err != nil {
		slog.Error("unexpected error when reading confidential file exports", slog.String("error", err.Error()))
	}

	c.JSON(http.StatusOK, gin.H{"availableFiles": dailyExports})
}

func (h *HttpEndpoints) getAvailableConfidentailDataExport(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	exportID := c.Param("exportID")
	studyKey := c.Param("studyKey")

	if h.dailyFileExportPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "daily file export path not set"})
		return
	}

	decoded, err := base64.URLEncoding.DecodeString(exportID)
	if err != nil {
		slog.Error("error decoding exportID", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "error decoding exportID"})
	}
	filename := string(decoded)

	slog.Info("downloading prepared confidential response export file", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("fileName", filename))

	resultFilePath := filepath.Join(h.dailyFileExportPath, token.InstanceID, studyKey, filename)

	// file exists?
	if _, err := os.Stat(resultFilePath); os.IsNotExist(err) {
		slog.Error("file does not exist", slog.String("path", resultFilePath))
		c.JSON(http.StatusNotFound, gin.H{"error": "file does not exist"})
		return
	}

	// Return file from file system
	ext := filepath.Ext(resultFilePath)
	contentType := "application/json"
	if ext == ".csv" {
		contentType = "text/csv"
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	c.File(resultFilePath)
}

func (h *HttpEndpoints) getExportTaskStatus(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	taskID := c.Param("taskID")

	slog.Info("getting export task status", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("taskID", taskID))

	task, err := h.studyDBConn.GetTaskByID(token.InstanceID, taskID)
	if err != nil {
		slog.Error("failed to get export task status", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get export task status"})
		return
	}

	if task.CreatedBy != token.Subject && !token.IsAdmin {
		slog.Warn("user is not allowed to get task status", slog.String("userID", token.Subject), slog.String("taskID", taskID))
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task": task})
}

func (h *HttpEndpoints) getExportTaskResult(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	taskID := c.Param("taskID")

	slog.Info("getting export task result", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("taskID", taskID))

	task, err := h.studyDBConn.GetTaskByID(token.InstanceID, taskID)
	if err != nil {
		slog.Error("failed to get export task result", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get export task result"})
		return
	}

	if task.CreatedBy != token.Subject && !token.IsAdmin {
		slog.Warn("user is not allowed to get task result", slog.String("userID", token.Subject), slog.String("taskID", taskID))
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	if task.Status != studyTypes.TASK_STATUS_COMPLETED {
		slog.Error("task is not completed", slog.String("taskID", taskID), slog.String("status", task.Status))
		c.JSON(http.StatusBadRequest, gin.H{"error": "task is not completed"})
		return
	}

	resultFilePath := filepath.Join(h.filestorePath, task.ResultFile)

	// file exists?
	if _, err := os.Stat(resultFilePath); os.IsNotExist(err) {
		slog.Error("file does not exist", slog.String("path", resultFilePath))
		c.JSON(http.StatusNotFound, gin.H{"error": "file does not exist"})
		return
	}

	// Return file from file system
	filenameToSave := filepath.Base(task.ResultFile)
	c.Header("Content-Disposition", "attachment; filename="+filenameToSave)
	c.Header("Content-Type", task.FileType)
	c.File(resultFilePath)
}

func (h *HttpEndpoints) getDailyExports(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	slog.Info("getting daily exports", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	if h.dailyFileExportPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "daily file export path not set"})
		return
	}

	targetFolderPath := filepath.Join(h.dailyFileExportPath, token.InstanceID, studyKey)

	dailyExports := []string{}
	// collect all files

	err := filepath.Walk(targetFolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Parse date from filename (assuming format YYYY-MM-DD##responses##..##..)
		basename := filepath.Base(path)
		parts := strings.Split(basename, "##")
		if len(parts) < 2 {
			return nil
		}
		if parts[1] != "responses" {
			return nil
		}
		dailyExports = append(dailyExports, basename)
		return nil
	})
	if err != nil {
		slog.Error("unexpected error when reading daily file exports", slog.String("error", err.Error()))
	}

	c.JSON(http.StatusOK, gin.H{"dailyExports": dailyExports})
}

func (h *HttpEndpoints) getDailyExport(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	exportID := c.Param("exportID")
	studyKey := c.Param("studyKey")

	if h.dailyFileExportPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "daily file export path not set"})
		return
	}

	decoded, err := base64.URLEncoding.DecodeString(exportID)
	if err != nil {
		slog.Error("error decoding exportID", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "error decoding exportID"})
	}
	filename := string(decoded)

	slog.Info("downloading daily export file", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("fileName", filename))

	resultFilePath := filepath.Join(h.dailyFileExportPath, token.InstanceID, studyKey, filename)

	// file exists?
	if _, err := os.Stat(resultFilePath); os.IsNotExist(err) {
		slog.Error("file does not exist", slog.String("path", resultFilePath))
		c.JSON(http.StatusNotFound, gin.H{"error": "file does not exist"})
		return
	}

	// Return file from file system
	ext := filepath.Ext(resultFilePath)
	contentType := "application/json"
	if ext == ".csv" {
		contentType = "text/csv"
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", contentType)
	c.File(resultFilePath)
}

func (h *HttpEndpoints) getResponseAccountTrackingInfo(instanceID, studyKey, participantID string) surveyresponses.AccountTrackingInfo {
	pState, err := h.studyDBConn.GetParticipantByID(instanceID, studyKey, participantID)
	if err != nil {
		slog.Warn("failed to get participant account tracking info", slog.String("error", err.Error()), slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("participantID", participantID))
		return surveyresponses.AccountTrackingInfo{}
	}
	return surveyresponses.AccountTrackingInfoFromParticipant(pState)
}

func (h *HttpEndpoints) getStudyResponses(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	query, err := apihelpers.ParseResponseExportQueryFromCtx(c)
	if err != nil || query == nil {
		slog.Error("failed to parse query", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	surveyKey := query.SurveyKey
	if surveyKey == "" {
		slog.Error("surveyKey is required", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "surveyKey is required"})
		return
	}

	study, err := h.studyDBConn.GetStudy(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study"})
		return
	}

	slog.Info("getting study responses", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey))

	rawResponses, paginationInfo, err := h.studyDBConn.GetResponses(
		token.InstanceID,
		studyKey,
		query.PaginationInfos.Filter,
		query.PaginationInfos.Sort,
		query.PaginationInfos.Page,
		query.PaginationInfos.Limit,
	)
	if err != nil {
		slog.Error("failed to get study responses", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study responses"})
		return
	}

	surveyVersions, err := surveydefinition.PrepareSurveyInfosFromDB(
		h.studyDBConn,
		token.InstanceID,
		studyKey,
		surveyKey,
		&surveydefinition.ExtractOptions{
			UseLabelLang: "",
			IncludeItems: nil,
			ExcludeItems: nil,
		},
	)
	if err != nil {
		slog.Error("failed to get survey versions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey versions"})
		return
	}

	respParser, err := surveyresponses.NewResponseParser(
		surveyKey,
		surveyVersions,
		query.UseShortKeys,
		query.IncludeMeta,
		query.QuestionOptionSep,
		query.ExtraCtxCols,
	)
	if err != nil {
		slog.Error("failed to create response parser", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create response parser"})
		return
	}
	if study.Configs.TrackAccount {
		respParser.EnableAccountTracking()
	}

	responses := make([]map[string]interface{}, len(rawResponses))
	accountInfoCache := map[string]surveyresponses.AccountTrackingInfo{}

	for i, rawResp := range rawResponses {
		trackingInfo := surveyresponses.AccountTrackingInfo{}
		if study.Configs.TrackAccount {
			var ok bool
			trackingInfo, ok = accountInfoCache[rawResp.ParticipantID]
			if !ok {
				trackingInfo = h.getResponseAccountTrackingInfo(token.InstanceID, studyKey, rawResp.ParticipantID)
				accountInfoCache[rawResp.ParticipantID] = trackingInfo
			}
		}
		resp, err := respParser.ParseResponse(&rawResp, trackingInfo)
		if err != nil {
			slog.Error("failed to parse response", slog.String("error", err.Error()))
			continue
		}
		output, err := respParser.ResponseToFlatObj(resp)
		if err != nil {
			slog.Error("failed to convert response to flat object", slog.String("error", err.Error()))
			continue
		}
		responses[i] = output
	}

	c.JSON(http.StatusOK, gin.H{
		"responses":  responses,
		"pagination": paginationInfo,
	})
}

func (h *HttpEndpoints) getStudyResponseById(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	responseID := c.Param("responseID")

	slog.Info("getting study response by ID", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("responseID", responseID))

	query, err := apihelpers.ParseResponseExportQueryFromCtx(c)
	if err != nil {
		slog.Error("failed to parse response export query")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	rawResponse, err := h.studyDBConn.GetResponseByID(token.InstanceID, studyKey, responseID)
	if err != nil {
		slog.Error("failed to get study response by ID", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study response by ID"})
		return
	}

	study, err := h.studyDBConn.GetStudy(token.InstanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study"})
		return
	}

	surveyVersions, err := surveydefinition.PrepareSurveyInfosFromDB(
		h.studyDBConn,
		token.InstanceID,
		studyKey,
		rawResponse.Key,
		&surveydefinition.ExtractOptions{
			UseLabelLang: "",
			IncludeItems: nil,
			ExcludeItems: nil,
		},
	)
	if err != nil {
		slog.Error("failed to get survey versions", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get survey versions"})
		return
	}

	respParser, err := surveyresponses.NewResponseParser(
		rawResponse.Key,
		surveyVersions,
		query.UseShortKeys,
		query.IncludeMeta,
		query.QuestionOptionSep,
		nil, // TODO: add extra context columns optionally
	)
	if err != nil {
		slog.Error("failed to create response parser", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create response parser"})
		return
	}
	if study.Configs.TrackAccount {
		respParser.EnableAccountTracking()
	}

	trackingInfo := surveyresponses.AccountTrackingInfo{}
	if study.Configs.TrackAccount {
		trackingInfo = h.getResponseAccountTrackingInfo(token.InstanceID, studyKey, rawResponse.ParticipantID)
	}
	resp, err := respParser.ParseResponse(&rawResponse, trackingInfo)
	if err != nil {
		slog.Error("failed to parse response", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}

	output, err := respParser.ResponseToFlatObj(resp)
	if err != nil {
		slog.Error("failed to convert response to flat object", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to convert response to flat object"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"response": output})
}

func (h *HttpEndpoints) deleteStudyResponses(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	query, err := apihelpers.ParseResponseExportQueryFromCtx(c)
	if err != nil {
		slog.Error("failed to parse response export query")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	controlField := c.DefaultQuery("controlField", "")
	if controlField != studyKey {
		slog.Error("controlField does not match studyKey", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to delete study responses"})
		return
	}

	surveyKey := query.SurveyKey
	if surveyKey == "" {
		slog.Error("surveyKey is required", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "surveyKey is required"})
		return
	}

	filter := query.PaginationInfos.Filter
	filter["key"] = surveyKey // ensure surveyKey is included in the filter

	slog.Info("deleting study responses", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey))

	err = h.studyDBConn.DeleteResponses(token.InstanceID, studyKey, filter)
	if err != nil {
		slog.Error("failed to delete study responses", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete study responses"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study responses deleted"})
}

func (h *HttpEndpoints) deleteStudyResponse(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	responseID := c.Param("responseID")

	if responseID == "" {
		slog.Error("responseID is required", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))
		c.JSON(http.StatusBadRequest, gin.H{"error": "responseID is required"})
		return
	}

	slog.Info("deleting study response", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("responseID", responseID))

	err := h.studyDBConn.DeleteResponseByID(token.InstanceID, studyKey, responseID)
	if err != nil {
		slog.Error("failed to delete study response", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete study response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study response deleted"})
}

func (h *HttpEndpoints) getStudyParticipants(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")

	slog.Info("getting study participants", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	query, err := apihelpers.ParsePaginatedQueryFromCtx(c)
	if err != nil || query == nil {
		slog.Error("failed to parse paginated query", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	participants, paginationInfo, err := h.studyDBConn.GetParticipants(
		token.InstanceID,
		studyKey,
		query.Filter,
		query.Sort,
		query.Page,
		query.Limit,
	)
	if err != nil {
		slog.Error("failed to get study participants", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study participants"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"participants": participants,
		"pagination":   paginationInfo,
	})
}

func (h *HttpEndpoints) getStudyParticipant(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	participantID := c.Param("participantID")

	slog.Info("getting study participant", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("participantID", participantID))

	participant, err := h.studyDBConn.GetParticipantByID(token.InstanceID, studyKey, participantID)
	if err != nil {
		slog.Error("failed to get study participant", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study participant"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"participant": participant})
}

func (h *HttpEndpoints) getReportKeys(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")

	pid := c.DefaultQuery("pid", "")
	fromTsQuery := c.DefaultQuery("from", "")
	fromTs := int64(0)
	var err error
	if fromTsQuery != "" {
		fromTs, err = strconv.ParseInt(fromTsQuery, 10, 64)
		if err != nil {
			slog.Error("error parsing fromTS", slog.String("error", err.Error()))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid fromTS"})
			return
		}
	}

	toTSQuery := c.DefaultQuery("until", "")
	toTs := int64(0)
	if toTSQuery != "" {
		toTs, err = strconv.ParseInt(toTSQuery, 10, 64)
		if err != nil {
			slog.Error("error parsing toTS", slog.String("error", err.Error()))
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid toTS"})
			return
		}
	}

	slog.Info("fetching available report keys",
		slog.String("instanceID", token.InstanceID),
		slog.String("userID", token.Subject),
		slog.String("participantID", pid),
		slog.String("studyKey", studyKey),
		slog.Int64("from", fromTs),
		slog.Int64("until", toTs),
	)

	var filter *studyDB.ReportKeyFilters
	if pid != "" || toTSQuery != "" || fromTsQuery != "" {
		filter = &studyDB.ReportKeyFilters{
			ParticipantID: pid,
			FromTS:        fromTs,
			ToTS:          toTs,
		}
	}

	keys, err := h.studyDBConn.GetUniqueReportKeysForStudy(
		token.InstanceID,
		studyKey,
		filter,
	)
	if err != nil {
		slog.Error("error retrieving unique report keys", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study report keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reportKeys": keys,
	})
}

// parseReportQueryParams parses common report query parameters (reportKey, pid, from, until)
// and optionally the type parameter. It modifies the provided filter map in place.
// Returns the modified filter, the export type (defaults to "raw" if includeType is true), and any error.
func (h *HttpEndpoints) parseReportQueryParams(c *gin.Context, filter bson.M, includeType bool) (bson.M, string, error) {
	reportKey := c.DefaultQuery("reportKey", "")
	if reportKey != "" {
		filter["key"] = reportKey
	}
	pid := c.DefaultQuery("pid", "")
	if pid != "" {
		filter["participantID"] = pid
	}

	fromTsQuery := c.DefaultQuery("from", "")
	fromTs := int64(0)
	if fromTsQuery != "" {
		var err error
		fromTs, err = strconv.ParseInt(fromTsQuery, 10, 64)
		if err != nil {
			return nil, "", fmt.Errorf("invalid fromTS: %w", err)
		}
	}

	toTSQuery := c.DefaultQuery("until", "")
	toTs := int64(0)
	if toTSQuery != "" {
		var err error
		toTs, err = strconv.ParseInt(toTSQuery, 10, 64)
		if err != nil {
			return nil, "", fmt.Errorf("invalid toTS: %w", err)
		}
	}

	tsFilter := bson.M{}
	if fromTsQuery != "" {
		tsFilter["$gte"] = fromTs
	}
	if toTSQuery != "" {
		tsFilter["$lte"] = toTs
	}

	if len(tsFilter) > 0 {
		filter["timestamp"] = tsFilter
	}

	exportType := ""
	if includeType {
		exportType = c.DefaultQuery("type", "raw")
		// Validate type is either "raw" or "csv"
		if exportType != "raw" && exportType != "csv" {
			exportType = "raw" // Default to "raw" if invalid
		}
	}

	return filter, exportType, nil
}

func (h *HttpEndpoints) getStudyReports(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)
	studyKey := c.Param("studyKey")

	slog.Info("getting study reports", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	query, err := apihelpers.ParsePaginatedQueryFromCtx(c)
	if err != nil || query == nil {
		slog.Error("failed to parse paginated query", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	_, _, err = h.parseReportQueryParams(c, query.Filter, false)
	if err != nil {
		slog.Error("failed to parse report query params", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reports, paginationInfo, err := h.studyDBConn.GetReports(
		token.InstanceID,
		studyKey,
		query.Filter,
		query.Page,
		query.Limit,
	)
	if err != nil {
		slog.Error("failed to get study reports", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study reports"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reports":    reports,
		"pagination": paginationInfo,
	})
}

func (h *HttpEndpoints) getStudyReport(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	reportID := c.Param("reportID")

	slog.Info("getting study report", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("reportID", reportID))

	report, err := h.studyDBConn.GetReportByID(token.InstanceID, studyKey, reportID)
	if err != nil {
		slog.Error("failed to get study report", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study report"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"report": report})
}

func (h *HttpEndpoints) getStudyFiles(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")

	query, err := apihelpers.ParsePaginatedQueryFromCtx(c)
	if err != nil || query == nil {
		slog.Error("failed to parse query", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("getting study files", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey))

	files, paginationInfo, err := h.studyDBConn.GetParticipantFileInfos(
		token.InstanceID,
		studyKey,
		query.Filter,
		query.Page,
		query.Limit,
	)
	if err != nil {
		slog.Error("failed to get study files", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"fileInfos":  files,
		"pagination": paginationInfo,
	})
}

// download file
func (h *HttpEndpoints) getStudyFile(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	fileID := c.Param("fileID")

	slog.Info("getting study file", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("fileID", fileID))

	fileInfo, err := h.studyDBConn.GetParticipantFileInfoByID(token.InstanceID, studyKey, fileID)
	if err != nil {
		slog.Error("failed to get study file info", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study file info"})
		return
	}

	filePath := filepath.Join(h.filestorePath, fileInfo.Path)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		slog.Error("file does not exist", slog.String("path", filePath))
		c.JSON(http.StatusNotFound, gin.H{"error": "file does not exist"})
		return
	}

	// Return file from file system
	filenameToSave := filepath.Base(fileInfo.Path)
	c.Header("Content-Disposition", "attachment; filename="+filenameToSave)
	c.File(filePath)
}

func (h *HttpEndpoints) deleteStudyFile(c *gin.Context) {
	token := c.MustGet("validatedToken").(*jwthandling.ManagementUserClaims)

	studyKey := c.Param("studyKey")
	fileID := c.Param("fileID")

	slog.Info("deleting study file", slog.String("instanceID", token.InstanceID), slog.String("userID", token.Subject), slog.String("studyKey", studyKey), slog.String("fileID", fileID))

	fileInfo, err := h.studyDBConn.GetParticipantFileInfoByID(token.InstanceID, studyKey, fileID)
	if err != nil {
		slog.Error("failed to get study file info", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get study file info"})
		return
	}

	// remove file from file system
	err = os.Remove(filepath.Join(h.filestorePath, fileInfo.Path))
	if err != nil {
		slog.Error("failed to delete study file", slog.String("error", err.Error()), slog.String("path", fileInfo.Path))
	}
	if fileInfo.PreviewPath != "" {
		err := os.Remove(filepath.Join(h.filestorePath, fileInfo.PreviewPath))
		if err != nil {
			slog.Error("failed to delete study file preview", slog.String("error", err.Error()), slog.String("path", fileInfo.PreviewPath))
		}
	}

	// delete file info from database
	err = h.studyDBConn.DeleteParticipantFileInfoByID(token.InstanceID, studyKey, fileID)
	if err != nil {
		slog.Error("failed to delete study file", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete study file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "study file deleted"})
}
