package sender

import (
	"errors"
	"log/slog"
	"maps"
	"time"

	globalinfosDB "github.com/case-framework/case-backend/pkg/db/global-infos"
	messagingDB "github.com/case-framework/case-backend/pkg/db/messaging"
	participantuser "github.com/case-framework/case-backend/pkg/db/participant-user"
	studydb "github.com/case-framework/case-backend/pkg/db/study"
	emailsending "github.com/case-framework/case-backend/pkg/messaging/email-sending"
	"github.com/case-framework/case-backend/pkg/study/studyengine"
	umTypes "github.com/case-framework/case-backend/pkg/user-management/types"
	umUtils "github.com/case-framework/case-backend/pkg/user-management/utils"
)

// StudyMessageSender implements studyengine.StudyMessageSender using the platform services.
type StudyMessageSender struct {
	studyDB                      *studydb.StudyDBService
	participantUserDB            *participantuser.ParticipantUserDBService
	messagingDB                  *messagingDB.MessagingDBService
	globalInfosDB                *globalinfosDB.GlobalInfosDBService
	loginTokenTTL                time.Duration
	globalEmailTemplateConstants map[string]string
}

type MessageSenderConfig struct {
	LoginTokenTTL                time.Duration     `json:"login_token_ttl" yaml:"login_token_ttl"`
	GlobalEmailTemplateConstants map[string]string `json:"global_email_template_constants" yaml:"global_email_template_constants"`
}

func NewStudyMessageSender(
	studyDB *studydb.StudyDBService,
	participantUserDB *participantuser.ParticipantUserDBService,
	messagingDB *messagingDB.MessagingDBService,
	globalInfosDB *globalinfosDB.GlobalInfosDBService,
	messageSenderConfig MessageSenderConfig,
) *StudyMessageSender {
	return &StudyMessageSender{
		studyDB:                      studyDB,
		participantUserDB:            participantUserDB,
		messagingDB:                  messagingDB,
		globalInfosDB:                globalInfosDB,
		loginTokenTTL:                messageSenderConfig.LoginTokenTTL,
		globalEmailTemplateConstants: messageSenderConfig.GlobalEmailTemplateConstants,
	}
}

// SendInstantStudyEmail prepares and sends an email immediately using a study template.
func (s *StudyMessageSender) SendInstantStudyEmail(
	instanceID string,
	studyKey string,
	confidentialPID string,
	messageType string,
	extraPayload map[string]string,
	opts studyengine.SendOptions,
) error {
	if s.studyDB == nil || s.participantUserDB == nil || s.messagingDB == nil {
		return errors.New("sender not initialized correctly")
	}

	// Map confidential participant id to profile id
	profileID, err := s.studyDB.GetProfileIDFromConfidentialID(instanceID, confidentialPID, studyKey)
	if err != nil || profileID == "" {
		slog.Error("profileID lookup failed", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("confidentialPID", confidentialPID))
		return errors.New("profileID lookup failed")
	}

	// Load user
	user, err := s.participantUserDB.GetUserByProfileID(instanceID, profileID)
	if err != nil {
		slog.Error("user lookup failed", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("profileID", profileID), slog.String("error", err.Error()))
		return err
	}

	// Determine recipient and profile info
	email, err := user.GetEmail()
	if err != nil {
		slog.Error("email lookup failed", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("profileID", profileID), slog.String("error", err.Error()))
		return err
	}
	to := []string{email.Email}

	if len(user.Profiles) == 0 {
		slog.Error("user has no profiles", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("userID", user.ID.Hex()))
		return errors.New("no profiles found for user")
	}

	currentProfile := user.Profiles[0]
	for _, p := range user.Profiles {
		if p.ID.Hex() == profileID {
			currentProfile = p
			break
		}
	}

	// Build payload
	payload := map[string]string{
		"profileAlias": currentProfile.Alias,
		"profileId":    currentProfile.ID.Hex(),
	}

	maps.Copy(payload, s.globalEmailTemplateConstants)

	// Only generate a login token if the globalInfosDB is configured and TTL is positive
	if s.globalInfosDB != nil && s.loginTokenTTL > 0 {
		loginToken, err := s.getTemploginToken(instanceID, user, studyKey)
		if err != nil {
			slog.Error("Error getting login token", slog.String("instanceID", instanceID), slog.String("studyKey", studyKey), slog.String("participantID", confidentialPID), slog.String("error", err.Error()))
		} else {
			payload["loginToken"] = loginToken
		}
	}
	// Merge extra payload (action-provided)
	maps.Copy(payload, extraPayload)

	// Determine language
	lang := user.Account.PreferredLanguage
	if opts.LanguageOverride != "" {
		lang = opts.LanguageOverride
	}

	expiresAt := opts.ExpiresAt
	if expiresAt == 0 {
		expiresAt = time.Now().Add(time.Hour * 24).Unix()
	}

	// Send immediately using the templating system; default to high priority
	err = emailsending.SendInstantEmailByTemplate(
		instanceID,
		to,
		user.ID.Hex(),
		messageType,
		studyKey,
		lang,
		payload,
		false, // useLowPrio
		expiresAt,
	)
	if err != nil {
		// The email-sending module already stores to outgoing on error
		return err
	}
	return nil
}

func (s *StudyMessageSender) getTemploginToken(instanceID string, user umTypes.User, studyKey string) (string, error) {
	tempTokenInfos := umTypes.TempToken{
		UserID:     user.ID.Hex(),
		InstanceID: instanceID,
		Purpose:    umTypes.TOKEN_PURPOSE_SURVEY_LOGIN,
		Info:       map[string]string{"studyKey": studyKey},
		Expiration: umUtils.GetExpirationTime(s.loginTokenTTL),
	}
	tempToken, err := s.globalInfosDB.AddTempToken(tempTokenInfos)
	if err != nil {
		slog.Error("failed to create login token", slog.String("error", err.Error()))
		return "", err
	}

	return tempToken, nil
}
