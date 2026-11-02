package participantuser

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/case-framework/case-backend/pkg/db"
	umTypes "github.com/case-framework/case-backend/pkg/user-management/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGetUserAndProfile(t *testing.T) {
	uri := os.Getenv("CASE_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set CASE_TEST_MONGO_URI to run MongoDB integration tests")
	}
	const instanceID = "test"
	service, err := NewParticipantUserDBService(db.DBConfig{
		URI: uri, Timeout: 10, MaxPoolSize: 4,
		DBNamePrefix: "case_user_lookup_test_" + primitive.NewObjectID().Hex() + "_",
		InstanceIDs:  []string{instanceID},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := service.DBClient.Database(service.getDBName(instanceID)).Drop(ctx); err != nil {
			t.Error(err)
		}
		if err := service.DBClient.Disconnect(ctx); err != nil {
			t.Error(err)
		}
	})

	targetProfile := umTypes.Profile{ID: primitive.NewObjectID(), Alias: "target"}
	owner := umTypes.User{
		ID: primitive.NewObjectID(),
		Profiles: []umTypes.Profile{
			{ID: primitive.NewObjectID(), Alias: "first profile"},
			targetProfile,
		},
	}
	otherUser := umTypes.User{ID: primitive.NewObjectID(), Profiles: []umTypes.Profile{{ID: primitive.NewObjectID()}}}
	emptyUser := umTypes.User{ID: primitive.NewObjectID()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = service.collectionParticipantUsers(instanceID).InsertMany(ctx, []interface{}{owner, otherUser, emptyUser})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		userID    string
		profileID string
		wantError string
	}{
		{"recorded user", owner.ID.Hex(), targetProfile.ID.Hex(), ""},
		{"legacy mapping", "", targetProfile.ID.Hex(), ""},
		{"wrong user does not fall back", otherUser.ID.Hex(), targetProfile.ID.Hex(), "is missing from user"},
		{"missing user does not fall back", primitive.NewObjectID().Hex(), targetProfile.ID.Hex(), "mapped user"},
		{"invalid user does not fall back", "invalid-id", targetProfile.ID.Hex(), "mapped user"},
		{"no profiles does not use a substitute", emptyUser.ID.Hex(), targetProfile.ID.Hex(), "is missing from user"},
		{"missing profile", owner.ID.Hex(), primitive.NewObjectID().Hex(), "is missing from user"},
		{"empty profile", owner.ID.Hex(), "", "profile ID must not be empty"},
		{"legacy missing profile", "", primitive.NewObjectID().Hex(), "legacy user lookup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user, profile, err := service.GetUserAndProfile(instanceID, tc.userID, tc.profileID)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("expected error containing %q, got %v", tc.wantError, err)
				}
				if !user.ID.IsZero() || !profile.ID.IsZero() {
					t.Fatal("failed lookup returned a usable user or profile")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if user.ID != owner.ID || profile.ID != targetProfile.ID || profile.Alias != targetProfile.Alias {
				t.Fatalf("lookup resolved the wrong user/profile: %+v, %+v", user, profile)
			}
		})
	}
}
