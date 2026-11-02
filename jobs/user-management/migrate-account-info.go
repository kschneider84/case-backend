package main

import (
	"context"
	"errors"
	"log/slog"

	studyService "github.com/case-framework/case-backend/pkg/study"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	umTypes "github.com/case-framework/case-backend/pkg/user-management/types"
	umUtils "github.com/case-framework/case-backend/pkg/user-management/utils"
	"go.mongodb.org/mongo-driver/bson"
)

func migrateAccountInfo() {
	for _, instanceID := range conf.InstanceIDs {
		slog.Info("Start migrating account info for participants", slog.String("instanceID", instanceID))

		// Include inactive studies so historical exports use the same account grouping.
		studies, err := studyDBService.GetStudies(instanceID, "", false)
		if err != nil {
			slog.Error("Failed to get studies", slog.String("instanceID", instanceID), slog.String("error", err.Error()))
			continue
		}

		// Only process studies that have account tracking enabled
		trackingStudies := []studyTypes.Study{}
		for _, s := range studies {
			if s.Configs.TrackAccount {
				trackingStudies = append(trackingStudies, s)
			}
		}

		if len(trackingStudies) == 0 {
			slog.Info("No studies with account tracking enabled", slog.String("instanceID", instanceID))
			continue
		}

		slog.Info("Found studies with account tracking", slog.String("instanceID", instanceID), slog.Int("count", len(trackingStudies)))

		migratedCount := 0
		failedCount := 0

		err = participantUserDBService.FindAndExecuteOnUsers(
			context.Background(),
			instanceID,
			bson.M{},
			nil,
			false,
			func(user umTypes.User, args ...interface{}) error {
				if user.ID.IsZero() {
					failedCount++
					return errors.New("cannot migrate account info without a user document ID")
				}

				mainProfileID, _ := umUtils.GetMainAndOtherProfiles(user)

				for _, profile := range user.Profiles {
					profileID := profile.ID.Hex()
					isMainProfile := mainProfileID == profileID

					for _, study := range trackingStudies {
						participantID, _, err := studyService.ComputeParticipantIDs(study, profileID)
						if err != nil {
							failedCount++
							slog.Error("Error computing participant IDs", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("profileID", profileID), slog.String("error", err.Error()))
							continue
						}

						hashedAccountID, err := studyService.ComputeHashedAccountID(study, user.ID.Hex())
						if err != nil {
							failedCount++
							slog.Error("Error hashing account ID", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", participantID), slog.String("error", err.Error()))
							continue
						}

						// Replace legacy email-derived values as well as filling missing fields.
						updated, err := studyDBService.UpdateParticipantAccountInfo(instanceID, study.Key, participantID, hashedAccountID, isMainProfile)
						if err != nil {
							failedCount++
							slog.Error("Error updating participant account info", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", participantID), slog.String("error", err.Error()))
							continue
						}

						if updated {
							migratedCount++
							slog.Debug("Migrated account info for participant", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", participantID))
						}
					}
				}
				return nil
			},
		)
		if err != nil {
			slog.Error("Error iterating users for account info migration", slog.String("instanceID", instanceID), slog.String("error", err.Error()))
			continue
		}

		slog.Info("Finished migrating account info for participants", slog.String("instanceID", instanceID), slog.Int("migratedCount", migratedCount), slog.Int("failedCount", failedCount))
	}
}
