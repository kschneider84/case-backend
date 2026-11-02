package main

import (
	"context"
	"log/slog"

	studyService "github.com/case-framework/case-backend/pkg/study"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	umTypes "github.com/case-framework/case-backend/pkg/user-management/types"
	"go.mongodb.org/mongo-driver/bson"
)

func generateProfileIDLookup() {
	for _, instanceID := range conf.InstanceIDs {
		slog.Debug("Start generating profile ID lookup", slog.String("instanceID", instanceID))

		// get studies for this instance
		studies, err := studyDBService.GetStudies(instanceID, studyTypes.STUDY_STATUS_ACTIVE, false)
		if err != nil {
			slog.Error("Failed to get studies", slog.String("instanceID", instanceID), slog.String("error", err.Error()))
			continue
		}

		// call DB method participantUserDBService
		filter := bson.M{}
		err = participantUserDBService.FindAndExecuteOnUsers(
			context.Background(),
			instanceID,
			filter,
			nil,
			false,
			func(user umTypes.User, args ...interface{}) error {
				for _, profile := range user.Profiles {
					profileID := profile.ID.Hex()
					for _, study := range studies {
						studyKey := study.Key
						_, confidentialID, err := studyService.ComputeParticipantIDs(study, profileID)
						if err != nil {
							slog.Error("Error computing participant IDs", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("error", err.Error()))
							continue
						}

						_, err = studyDBService.GetProfileIDFromConfidentialID(instanceID, confidentialID, studyKey)
						if err != nil {
							// Create new lookup entry
							if err = studyDBService.AddConfidentialIDMapEntry(instanceID, confidentialID, profileID, studyKey); err != nil {
								slog.Error("Error saving participant ID profile lookup", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("error", err.Error()))
								continue
							}
						}
					}
				}
				return nil
			},
		)
		if err != nil {
			slog.Error("Error generating profile ID lookup", slog.String("instanceID", instanceID), slog.String("error", err.Error()))
			continue
		}
	}
}
