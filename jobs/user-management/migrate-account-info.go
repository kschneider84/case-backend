package main

import (
	"context"
	"log/slog"

	studyService "github.com/case-framework/case-backend/pkg/study"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	studyUtils "github.com/case-framework/case-backend/pkg/study/utils"
	umTypes "github.com/case-framework/case-backend/pkg/user-management/types"
	umUtils "github.com/case-framework/case-backend/pkg/user-management/utils"
	"go.mongodb.org/mongo-driver/bson"
)

func migrateAccountInfo() {
	for _, instanceID := range conf.InstanceIDs {
		slog.Info("Start migrating account info for participants", slog.String("instanceID", instanceID))

		studies, err := studyDBService.GetStudies(instanceID, studyTypes.STUDY_STATUS_ACTIVE, false)
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

		err = participantUserDBService.FindAndExecuteOnUsers(
			context.Background(),
			instanceID,
			bson.M{},
			nil,
			false,
			func(user umTypes.User, args ...interface{}) error {
				accountID := user.Account.AccountID
				if accountID == "" {
					return nil
				}

				mainProfileID, _ := umUtils.GetMainAndOtherProfiles(user)

				for _, profile := range user.Profiles {
					profileID := profile.ID.Hex()
					isMainProfile := mainProfileID == profileID

					for _, study := range trackingStudies {
						participantID, _, err := studyService.ComputeParticipantIDs(study, profileID)
						if err != nil {
							slog.Error("Error computing participant IDs", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("profileID", profileID), slog.String("error", err.Error()))
							continue
						}

						pState, err := studyDBService.GetParticipantByID(instanceID, study.Key, participantID)
						if err != nil {
							// Participant not in this study, skip
							continue
						}

						if pState.HashedAccountID != nil {
							// Already migrated, skip
							continue
						}

						// Reuse same hashing mechanism to pseudonymize the account ID
						hashedAccountID, err := studyUtils.ProfileIDtoParticipantID(accountID, conf.StudyConfigs.GlobalSecret, study.SecretKey, study.Configs.IdMappingMethod)
						if err != nil {
							slog.Error("Error hashing account ID", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", participantID), slog.String("error", err.Error()))
							continue
						}

						pState.HashedAccountID = &hashedAccountID
						pState.IsMainProfile = &isMainProfile

						_, err = studyDBService.SaveParticipantState(instanceID, study.Key, pState)
						if err != nil {
							slog.Error("Error saving participant state", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", participantID), slog.String("error", err.Error()))
							continue
						}

						migratedCount++
						slog.Debug("Migrated account info for participant", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", participantID))
					}
				}
				return nil
			},
		)
		if err != nil {
			slog.Error("Error iterating users for account info migration", slog.String("instanceID", instanceID), slog.String("error", err.Error()))
		}

		slog.Info("Finished migrating account info for participants", slog.String("instanceID", instanceID), slog.Int("migratedCount", migratedCount))
	}
}
