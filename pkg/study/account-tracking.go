package study

import (
	"errors"

	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	studyUtils "github.com/case-framework/case-backend/pkg/study/utils"
)

// ComputeHashedAccountID pseudonymizes the stable user document ID for account grouping.
// userID must be the hexadecimal user document ID, not the mutable login email.
func ComputeHashedAccountID(study studyTypes.Study, userID string) (string, error) {
	if userID == "" {
		return "", errors.New("user ID must not be empty")
	}
	return studyUtils.ProfileIDtoParticipantID(userID, globalSecret, study.SecretKey, study.Configs.IdMappingMethod)
}
