package utils

import (
	"errors"
	"fmt"
	"strings"

	userTypes "github.com/case-framework/case-backend/pkg/user-management/types"
)

const ProfileAliasMaxUTF16Length = 20

// ValidateProfileAlias counts UTF-16 code units, matching JavaScript string.length.
// Blank aliases are rejected; accepted aliases are not normalized.
func ValidateProfileAlias(alias string) error {
	if strings.TrimSpace(alias) == "" {
		return errors.New("profile alias must not be blank")
	}
	length := 0
	for _, r := range alias {
		length++
		if r > 0xFFFF {
			length++
		}
		if length > ProfileAliasMaxUTF16Length {
			return fmt.Errorf("profile alias must not exceed %d UTF-16 code units", ProfileAliasMaxUTF16Length)
		}
	}
	return nil
}

// GetMainAndOtherProfiles extract main profile ID and secondary profiles from a user account info
func GetMainAndOtherProfiles(user userTypes.User) (mainProfileID string, otherProfileIDs []string) {
	mainProfileID = ""
	otherProfileIDs = []string{}
	for _, p := range user.Profiles {
		if !p.MainProfile {
			otherProfileIDs = append(otherProfileIDs, p.ID.Hex())
		} else {
			mainProfileID = p.ID.Hex()
		}
	}
	if mainProfileID == "" {
		if len(otherProfileIDs) == 0 {
			return "", []string{}
		}
		mainProfileID = otherProfileIDs[0]
		otherProfileIDs = otherProfileIDs[1:]
	}
	return mainProfileID, otherProfileIDs
}
