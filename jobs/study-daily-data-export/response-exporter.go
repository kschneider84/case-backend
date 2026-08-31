package main

import (
	"context"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	studyDB "github.com/case-framework/case-backend/pkg/db/study"
	surveydefinition "github.com/case-framework/case-backend/pkg/study/exporter/survey-definition"
	surveyresponses "github.com/case-framework/case-backend/pkg/study/exporter/survey-responses"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func runResponseExportsForTask(rExpTask ResponseExportTask) {
	// ensure there is a folder path for the source (export_path/instance_id/study_key)
	relativeFolderPath := path.Join(rExpTask.InstanceID, rExpTask.StudyKey)
	exportFolderPathForTask := path.Join(conf.ExportPath, relativeFolderPath)
	if _, err := os.Stat(exportFolderPathForTask); os.IsNotExist(err) {
		// create folder
		err = os.MkdirAll(exportFolderPathForTask, os.ModePerm)
		if err != nil {
			slog.Error("Error creating export path", slog.String("error", err.Error()))
			return
		}
		slog.Info("Created export path", slog.String("path", exportFolderPathForTask))
	}

	// remove old files (keep only the last retention_days, but at least yesterday and today)
	if err := cleanUpForSource(exportFolderPathForTask); err != nil {
		slog.Error("Error cleaning up old files", slog.String("error", err.Error()))
	}

	for _, surveyKey := range rExpTask.SurveyKeys {
		parser, trackAccount, err := initResponseParser(rExpTask.InstanceID, rExpTask.StudyKey, surveyKey, rExpTask.ShortKeys, rExpTask.Separator)
		if err != nil {
			continue
		}

		if conf.ResponseExports.OverrideOld {
			for i := 0; i < conf.ResponseExports.RetentionDays-1; i++ {
				targetDate := time.Now().Add(
					time.Duration(-(conf.ResponseExports.RetentionDays - i)) * time.Hour * 24,
				)
				generateExportForSurveyForTargetDate(rExpTask.InstanceID, rExpTask.StudyKey, surveyKey, rExpTask.ExportFormat, targetDate, exportFolderPathForTask, parser, rExpTask.CreateEmptyFile, trackAccount)
			}
		}

		// yesterday
		targetDate := time.Now().Add(
			time.Duration(-1 * time.Hour * 24),
		)
		generateExportForSurveyForTargetDate(rExpTask.InstanceID, rExpTask.StudyKey, surveyKey, rExpTask.ExportFormat, targetDate, exportFolderPathForTask, parser, rExpTask.CreateEmptyFile, trackAccount)

		// today
		targetDate = time.Now()
		generateExportForSurveyForTargetDate(rExpTask.InstanceID, rExpTask.StudyKey, surveyKey, rExpTask.ExportFormat, targetDate, exportFolderPathForTask, parser, rExpTask.CreateEmptyFile, trackAccount)
	}
}

func initResponseParser(instanceID string, studyKey string, surveyKey string, shortKeys bool, separator string) (*surveyresponses.ResponseParser, bool, error) {
	study, err := studyDBService.GetStudy(instanceID, studyKey)
	if err != nil {
		slog.Error("failed to get study", slog.String("error", err.Error()))
		return nil, false, err
	}

	surveyVersions, err := surveydefinition.PrepareSurveyInfosFromDB(
		studyDBService,
		instanceID,
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
		return nil, false, err
	}
	extraCols := conf.ResponseExports.ExportTasks[0].ExtraCtxCols
	parser, err := surveyresponses.NewResponseParser(
		surveyKey,
		surveyVersions,
		shortKeys,
		nil,
		separator,
		&extraCols,
	)
	if err != nil {
		slog.Error("failed to create response parser", slog.String("error", err.Error()))
		return nil, false, err
	}
	trackAccount := study.Configs.TrackAccount
	if trackAccount {
		parser.EnableAccountTracking()
	}
	return parser, trackAccount, nil
}

func getResponseAccountTrackingInfo(instanceID, studyKey, participantID string) surveyresponses.AccountTrackingInfo {
	pState, err := studyDBService.GetParticipantByID(instanceID, studyKey, participantID)
	if err != nil {
		slog.Warn("failed to get participant account tracking info", slog.String("error", err.Error()), slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("participantID", participantID))
		return surveyresponses.AccountTrackingInfo{}
	}
	return surveyresponses.AccountTrackingInfoFromParticipant(pState)
}

func generateExportForSurveyForTargetDate(instanceID string, studyKey string, surveyKey string, format string, targetDate time.Time, exportPath string, parser *surveyresponses.ResponseParser, createEmptyFile bool, trackAccount bool) {
	fileName := responseFileName(targetDate, surveyKey, format)
	responseFilePath := filepath.Join(exportPath, fileName)

	if fileExists(responseFilePath) {
		slog.Debug("File already exists, overriding", slog.String("path", responseFilePath))
	}

	filter := bson.M{
		"key": surveyKey,
		"$and": bson.A{
			bson.M{"arrivedAt": bson.M{"$lte": endOfDay(targetDate).Unix()}},
			bson.M{"arrivedAt": bson.M{"$gte": startOfDay(targetDate).Unix()}},
		},
	}
	// count responses for target date and survey key --> if 0, skip
	count, err := studyDBService.GetResponsesCount(instanceID, studyKey, filter)
	if err != nil {
		slog.Error("Error getting responses count", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey), slog.String("error", err.Error()))
		return
	}
	if !createEmptyFile && count == 0 {
		slog.Debug("No responses for target date and survey key, skipping", slog.String("targetDate", targetDate.Format("2006-01-02")), slog.String("surveyKey", surveyKey))
		return
	}

	file, err := os.Create(responseFilePath)
	if err != nil {
		slog.Error("failed to create export file", slog.String("error", err.Error()))
		return
	}

	defer file.Close()

	exporter, err := surveyresponses.NewResponseExporter(
		parser,
		file,
		format,
	)
	if err != nil {
		slog.Error("failed to create response exporter", slog.String("error", err.Error()))
		return
	}

	accountInfoCache := map[string]surveyresponses.AccountTrackingInfo{}
	err = studyDBService.FindAndExecuteOnResponses(
		context.Background(),
		instanceID,
		studyKey,
		filter,
		bson.M{"arrivedAt": 1},
		false,
		func(dbService *studyDB.StudyDBService, r studyTypes.SurveyResponse, instanceID, studyKey string, args ...interface{}) error {
			trackingInfo := surveyresponses.AccountTrackingInfo{}
			if trackAccount {
				var ok bool
				trackingInfo, ok = accountInfoCache[r.ParticipantID]
				if !ok {
					trackingInfo = getResponseAccountTrackingInfo(instanceID, studyKey, r.ParticipantID)
					accountInfoCache[r.ParticipantID] = trackingInfo
				}
			}

			err := exporter.WriteResponse(&r, trackingInfo)
			if err != nil {
				return err
			}
			return nil
		},
		nil,
	)
	if err != nil {
		slog.Error("Error generating response export", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("surveyKey", surveyKey), slog.String("error", err.Error()))
		return
	}

	err = exporter.Finish()
	if err != nil {
		slog.Error("failed to finish export", slog.String("error", err.Error()))
		return
	}
	slog.Info("Generated response export", slog.String("path", responseFilePath))
}

func cleanUpForSource(sourceDir string) error {
	cutoffDate := time.Now().Add(
		time.Duration(-(conf.ResponseExports.RetentionDays + 1)) * time.Hour * 24,
	)

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Parse date from filename (assuming format YYYY-MM-DD##responses##..##..)
		basename := filepath.Base(path)
		parts := strings.Split(basename, "##")
		if len(parts) < 1 {
			return nil
		}
		datePart := parts[0]
		if len(datePart) < 10 {
			return nil
		}

		fileDate, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			return nil
		}

		if fileDate.Before(cutoffDate) {
			if err := os.Remove(path); err != nil {
				slog.Error("Failed to remove old file", slog.String("path", path), slog.String("error", err.Error()))
			}
		}

		return nil
	})
}
