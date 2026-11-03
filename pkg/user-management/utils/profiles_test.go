package utils

import (
	"strings"
	"testing"

	userTypes "github.com/case-framework/case-backend/pkg/user-management/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestValidateProfileAlias(t *testing.T) {
	for _, tc := range []struct {
		name, alias string
		valid       bool
	}{
		{"empty", "", false},
		{"whitespace", "   ", false},
		{"tabs and newlines", "\t\r\n", false},
		{"Unicode whitespace", "\u00a0\u2003", false},
		{"surrounding whitespace", " Alice ", true},
		{"ASCII boundary", strings.Repeat("a", ProfileAliasMaxUTF16Length), true},
		{"ASCII too long", strings.Repeat("a", ProfileAliasMaxUTF16Length+1), false},
		{"BMP boundary", strings.Repeat("é", ProfileAliasMaxUTF16Length), true},
		{"BMP too long", strings.Repeat("é", ProfileAliasMaxUTF16Length+1), false},
		{"emoji boundary", strings.Repeat("😀", ProfileAliasMaxUTF16Length/2), true},
		{"emoji too long", strings.Repeat("😀", ProfileAliasMaxUTF16Length/2+1), false},
		{"mixed boundary", strings.Repeat("a", ProfileAliasMaxUTF16Length-2) + "😀", true},
		{"mixed too long", strings.Repeat("a", ProfileAliasMaxUTF16Length-1) + "😀", false},
		{"combining boundary", strings.Repeat("e\u0301", ProfileAliasMaxUTF16Length/2), true},
		{"combining too long", strings.Repeat("e\u0301", ProfileAliasMaxUTF16Length/2) + "a", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateProfileAlias(tc.alias)
			if (err == nil) != tc.valid {
				t.Fatalf("ValidateProfileAlias(%q) = %v, want valid=%v", tc.alias, err, tc.valid)
			}
		})
	}
}

func TestGetMainAndOtherProfiles(t *testing.T) {
	t.Run("with a single profile with main flag", func(t *testing.T) {
		user := userTypes.User{
			Profiles: []userTypes.Profile{
				{ID: primitive.NewObjectID(), MainProfile: true},
			},
		}
		main, others := GetMainAndOtherProfiles(user)
		if main != user.Profiles[0].ID.Hex() {
			t.Errorf("unexpected main id %s", main)
		}
		if len(others) != 0 {
			t.Errorf("unexpected number of other profiles %d", len(others))
		}
	})

	t.Run("with a single profile without main flag", func(t *testing.T) {
		user := userTypes.User{
			Profiles: []userTypes.Profile{
				{ID: primitive.NewObjectID()},
			},
		}
		main, others := GetMainAndOtherProfiles(user)
		if main != user.Profiles[0].ID.Hex() {
			t.Errorf("unexpected main id %s", main)
		}
		if len(others) != 0 {
			t.Errorf("unexpected number of other profiles %d", len(others))
		}
	})

	t.Run("with mulitple profiles without main flag", func(t *testing.T) {
		user := userTypes.User{
			Profiles: []userTypes.Profile{
				{ID: primitive.NewObjectID(), MainProfile: false},
				{ID: primitive.NewObjectID(), MainProfile: false},
				{ID: primitive.NewObjectID(), MainProfile: false},
				{ID: primitive.NewObjectID(), MainProfile: false},
			},
		}
		main, others := GetMainAndOtherProfiles(user)
		if main != user.Profiles[0].ID.Hex() {
			t.Errorf("unexpected main id %s", main)
		}
		if len(others) != 3 || others[0] == main {
			t.Errorf("unexpected number of other profiles %d or wrong ids", len(others))
		}
	})

	t.Run("with mulitple profiles one main flag", func(t *testing.T) {
		user := userTypes.User{
			Profiles: []userTypes.Profile{
				{ID: primitive.NewObjectID(), MainProfile: false},
				{ID: primitive.NewObjectID(), MainProfile: true},
				{ID: primitive.NewObjectID(), MainProfile: false},
			},
		}
		main, others := GetMainAndOtherProfiles(user)
		if main != user.Profiles[1].ID.Hex() {
			t.Errorf("unexpected main id %s", main)
		}
		if len(others) != 2 {
			t.Errorf("unexpected number of other profiles %d", len(others))
		}
	})

	t.Run("with mulitple profiles multiply main flag", func(t *testing.T) {
		user := userTypes.User{
			Profiles: []userTypes.Profile{
				{ID: primitive.NewObjectID(), MainProfile: false},
				{ID: primitive.NewObjectID(), MainProfile: true},
				{ID: primitive.NewObjectID(), MainProfile: true},
			},
		}
		main, others := GetMainAndOtherProfiles(user)
		if main != user.Profiles[2].ID.Hex() {
			t.Errorf("unexpected main id %s", main)
		}
		if len(others) != 1 {
			t.Errorf("unexpected number of other profiles %d", len(others))
		}
	})

}
