package main

import (
	"context"
	"log/slog"
	"sync"
	"time"

	studyDB "github.com/case-framework/case-backend/pkg/db/study"
	emailsending "github.com/case-framework/case-backend/pkg/messaging/email-sending"
	messagingTypes "github.com/case-framework/case-backend/pkg/messaging/types"
	studyservice "github.com/case-framework/case-backend/pkg/study"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func handleParticipantMessages(wg *sync.WaitGroup) {
	defer wg.Done()
	slog.Info("Start handling participant messages")

	for _, instanceID := range conf.InstanceIDs {
		counters := InitMessageCounter()

		messageTemplateCache := map[string]messagingTypes.EmailTemplate{}

		studies, err := studyDBService.GetStudies(instanceID, "", false)
		if err != nil {
			slog.Error("Error getting studies", slog.String("instanceID", instanceID), slog.String("error", err.Error()))
			continue
		}

		for _, study := range studies {
			filter := bson.M{
				"studyStatus":           studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE,
				"messages.scheduledFor": bson.M{"$lt": time.Now().Unix()},
			}
			err := studyDBService.FindAndExecuteOnParticipantsStates(
				context.Background(),
				instanceID,
				study.Key,
				filter,
				nil,
				false,
				func(dbService *studyDB.StudyDBService, p studyTypes.Participant, instanceID, studyKey string, args ...interface{}) error {
					// relevant messages:
					messages := getRelevantMessages(p)
					if len(messages) == 0 {
						// no messages to send
						slog.Error("unexpected issue: no messages to send", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", p.ParticipantID))
						return nil
					}

					// find user:
					profileID, err := getProfileID(instanceID, study, p)
					if err != nil {
						slog.Error("Error getting profileID", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", p.ParticipantID), slog.String("error", err.Error()))
						return nil
					}

					user, err := participantUserDBService.GetUserByProfileID(instanceID, profileID)
					if err != nil {
						slog.Error("Error getting user", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", p.ParticipantID), slog.String("error", err.Error()))
						return nil
					}

					currentProfile := user.Profiles[0]
					for _, profile := range user.Profiles {
						if profile.ID.Hex() == profileID {
							currentProfile = profile
							break
						}
					}

					sentMessages := []string{}
					for _, message := range messages {
						// Retrieve the study email template
						templateName := message.Type + study.Key
						template, ok := messageTemplateCache[templateName]
						if !ok {
							t, err := messagingDBService.GetStudyEmailTemplateByMessageType(instanceID, study.Key, message.Type)
							if err != nil {
								counters.IncreaseCounter(false)
								slog.Error("Error getting study email template", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("messageType", message.Type), slog.String("error", err.Error()))
								continue
							}
							messageTemplateCache[templateName] = *t
							template = *t
						}

						payload := map[string]string{
							"studyKey":     study.Key,
							"profileAlias": currentProfile.Alias,
							"profileId":    currentProfile.ID.Hex(),
							"language":     user.Account.PreferredLanguage,
						}

						loginToken, err := getTemploginToken(instanceID, user, study.Key)
						if err != nil {
							slog.Error("Error getting login token", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", p.ParticipantID), slog.String("error", err.Error()))
						} else {
							payload["loginToken"] = loginToken
						}

						// include participant flags into payload:
						for k, v := range p.Flags {
							payload["flags."+k] = v
						}

						// include linking codes into payload
						for k, v := range p.LinkingCodes {
							payload["linkingCodes."+k] = v
						}

						subject, content, err := emailsending.GenerateEmailContent(template, user.Account.PreferredLanguage, payload)
						if err != nil {
							counters.IncreaseCounter(false)
							slog.Error("Error generating email content", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("messageType", message.Type), slog.String("error", err.Error()))
							continue
						}

						to := []string{
							user.Account.AccountID,
						}

						outgoingEmail := messagingTypes.OutgoingEmail{
							MessageType:     message.Type,
							HeaderOverrides: template.HeaderOverrides,
							To:              to,
							UserID:          user.ID.Hex(),
							Subject:         subject,
							Content:         content,
						}

						_, err = messagingDBService.AddToOutgoingEmails(instanceID, outgoingEmail)
						if err != nil {
							slog.Error("Failed to save outgoing email", slog.String("error", err.Error()), slog.String("instanceID", instanceID))
							counters.IncreaseCounter(false)
							continue
						}
						counters.IncreaseCounter(true)
						sentMessages = append(sentMessages, message.ID.Hex())
					}

					// delete messages from participant
					if len(sentMessages) > 0 {
						err = studyDBService.DeleteMessagesFromParticipant(instanceID, study.Key, p.ParticipantID, sentMessages)
						if err != nil {
							slog.Error("Error deleting participant messages", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("participantID", p.ParticipantID), slog.String("error", err.Error()))
						}
					}

					return nil
				},
			)
			if err != nil {
				slog.Debug("Error getting participant states", slog.String("instanceID", instanceID), slog.String("studyKey", study.Key), slog.String("error", err.Error()))
				continue
			}

		}

		counters.Stop()
		slog.Info("Finished handling participant messages for instance", slog.String("instanceID", instanceID), slog.Int("failed", counters.Failed), slog.Int("success", counters.Success))
	}

	slog.Info("Finished handling participant messages")
}

func getRelevantMessages(p studyTypes.Participant) []studyTypes.StudyMessage {
	messages := []studyTypes.StudyMessage{}

	for _, message := range p.Messages {
		if message.ScheduledFor > time.Now().Unix() {
			continue
		}
		_id, err := primitive.ObjectIDFromHex(message.ID)
		if err != nil {
			slog.Error("Error parsing message id", slog.String("messageID", message.ID), slog.String("error", err.Error()))
			continue
		}
		messages = append(messages, studyTypes.StudyMessage{
			ID:      _id,
			Type:    message.Type,
			Payload: p.Flags,
		})
	}

	return messages
}

func getProfileID(instanceID string, study studyTypes.Study, p studyTypes.Participant) (string, error) {
	confidentialPID, err := studyservice.ComputeConfidentialIDForParticipant(study, p.ParticipantID)
	if err != nil {
		return "", err
	}

	return studyDBService.GetProfileIDFromConfidentialID(
		instanceID,
		confidentialPID,
		study.Key,
	)
}
