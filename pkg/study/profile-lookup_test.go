package study

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/case-framework/case-backend/pkg/db"
	studydb "github.com/case-framework/case-backend/pkg/db/study"
	"github.com/case-framework/case-backend/pkg/study/studyengine"
	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Set CASE_TEST_MONGO_URI to run these integration tests against MongoDB.
// Each test uses an isolated database and removes it during cleanup.
type profileLookupFixture struct {
	db             *studydb.StudyDBService
	database       *mongo.Database
	study          studyTypes.Study
	profileID      string
	userID         string
	participantID  string
	confidentialID string
	sends          int
}

const profileLookupInstance = "test"

func newProfileLookupFixture(t *testing.T, rules []studyTypes.Expression) *profileLookupFixture {
	t.Helper()
	uri := os.Getenv("CASE_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set CASE_TEST_MONGO_URI to run MongoDB integration tests")
	}
	service, err := studydb.NewStudyDBService(db.DBConfig{
		URI: uri, Timeout: 10, MaxPoolSize: 16,
		DBNamePrefix: "case_profile_lookup_test_" + primitive.NewObjectID().Hex() + "_",
		InstanceIDs:  []string{profileLookupInstance},
	})
	if err != nil {
		t.Fatal(err)
	}
	f := &profileLookupFixture{
		db:        service,
		database:  service.DBClient.Database(service.DBNamePrefix + profileLookupInstance + "_studyDB"),
		profileID: primitive.NewObjectID().Hex(),
		userID:    primitive.NewObjectID().Hex(),
		study: studyTypes.Study{
			Key: "study", Status: studyTypes.STUDY_STATUS_ACTIVE, SecretKey: "test-study-secret",
			Configs: studyTypes.StudyConfigs{IdMappingMethod: "sha256-b64"},
		},
	}
	oldDB, oldSecret, oldEngine := studyDBService, globalSecret, studyengine.CurrentStudyEngine
	t.Cleanup(func() {
		studyDBService, globalSecret, studyengine.CurrentStudyEngine = oldDB, oldSecret, oldEngine
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := f.database.Drop(ctx); err != nil {
			t.Errorf("dropping test database: %v", err)
		}
		if err := service.DBClient.Disconnect(ctx); err != nil {
			t.Errorf("disconnecting test client: %v", err)
		}
	})
	Init(service, "test-global-secret", nil, f)
	f.participantID, f.confidentialID, err = ComputeParticipantIDs(f.study, f.profileID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.CreateStudy(profileLookupInstance, f.study); err != nil {
		t.Fatal(err)
	}
	service.CreateDefaultIndexesForConfidentialIDMapCollection(profileLookupInstance)
	ruleSet := studyTypes.StudyRules{StudyKey: f.study.Key, Rules: rules}
	if err := ruleSet.MarshalRules(); err != nil {
		t.Fatal(err)
	}
	if err := service.SaveStudyRules(profileLookupInstance, f.study.Key, ruleSet); err != nil {
		t.Fatal(err)
	}
	f.saveParticipant(t, f.participantID, studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE)
	return f
}

func (f *profileLookupFixture) saveParticipant(t *testing.T, participantID, status string) {
	t.Helper()
	_, err := f.db.SaveParticipantState(profileLookupInstance, f.study.Key, studyTypes.Participant{
		ParticipantID: participantID, StudyStatus: status, EnteredAt: time.Now().Unix(),
		LinkingCodes: map[string]string{"code": "test-code"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (f *profileLookupFixture) assertLookupUser(t *testing.T) {
	t.Helper()
	entry, err := f.db.GetConfidentialIDMapEntry(profileLookupInstance, f.confidentialID, f.study.Key)
	if err != nil || entry.UserID != f.userID {
		t.Fatalf("lookup user ID = %q, %v; want %q", entry.UserID, err, f.userID)
	}
}

func (f *profileLookupFixture) SendInstantStudyEmail(instanceID, studyKey, confidentialID, messageType string, payload map[string]string, opts studyengine.SendOptions) error {
	f.sends++
	profileID, err := f.db.GetProfileIDFromConfidentialID(instanceID, confidentialID, studyKey)
	if err != nil {
		return fmt.Errorf("mapping must exist before message rule runs: %w", err)
	}
	if profileID != f.profileID {
		return fmt.Errorf("message resolved to profile %q, expected %q", profileID, f.profileID)
	}
	return nil
}

type registeredProfileEvent struct {
	name string
	run  func(*testing.T, *profileLookupFixture) error
}

func registeredProfileEvents() []registeredProfileEvent {
	return []registeredProfileEvent{
		{"enter existing active participant", func(t *testing.T, f *profileLookupFixture) error {
			_, err := OnEnterStudy(profileLookupInstance, f.study.Key, f.profileID, f.userID, true)
			return err
		}},
		{"enter new participant", func(t *testing.T, f *profileLookupFixture) error {
			if err := f.db.DeleteParticipantByID(profileLookupInstance, f.study.Key, f.participantID); err != nil {
				t.Fatal(err)
			}
			_, err := OnEnterStudy(profileLookupInstance, f.study.Key, f.profileID, f.userID, true)
			return err
		}},
		{"reenter exited participant", func(t *testing.T, f *profileLookupFixture) error {
			f.saveParticipant(t, f.participantID, studyTypes.PARTICIPANT_STUDY_STATUS_EXITED)
			_, err := OnEnterStudy(profileLookupInstance, f.study.Key, f.profileID, f.userID, true)
			return err
		}},
		{"custom event", func(t *testing.T, f *profileLookupFixture) error {
			_, err := OnCustomStudyEvent(profileLookupInstance, f.study.Key, f.profileID, f.userID, "test-event", nil)
			return err
		}},
		{"submission", func(t *testing.T, f *profileLookupFixture) error {
			_, err := OnSubmitResponse(profileLookupInstance, f.study.Key, f.profileID, f.userID, studyTypes.SurveyResponse{Key: "survey"})
			return err
		}},
		{"leave", func(t *testing.T, f *profileLookupFixture) error {
			_, err := OnLeaveStudy(profileLookupInstance, f.study.Key, f.profileID, f.userID)
			return err
		}},
		{"merge temporary into existing participant", func(t *testing.T, f *profileLookupFixture) error {
			f.saveParticipant(t, "temporary", studyTypes.PARTICIPANT_STUDY_STATUS_TEMPORARY)
			_, err := OnMergeTempParticipant(profileLookupInstance, f.study.Key, f.profileID, f.userID, "temporary")
			return err
		}},
		{"merge virtual into existing participant", func(t *testing.T, f *profileLookupFixture) error {
			f.saveParticipant(t, "virtual", studyTypes.PARTICIPANT_STUDY_STATUS_VIRTUAL)
			_, err := OnMergeVirtualParticipant(profileLookupInstance, f.study.Key, f.profileID, f.userID, "virtual", "code", "test-code")
			return err
		}},
	}
}

func TestRegisteredProfileEventsRecoverLookupBeforeRules(t *testing.T) {
	for _, event := range registeredProfileEvents() {
		t.Run(event.name, func(t *testing.T) {
			f := newProfileLookupFixture(t, []studyTypes.Expression{{
				Name: "SEND_MESSAGE_NOW", Data: []studyTypes.ExpressionArg{{DType: "str", Str: "test-message"}},
			}})
			if err := event.run(t, f); err != nil {
				t.Fatal(err)
			}
			profileID, err := f.db.GetProfileIDFromConfidentialID(profileLookupInstance, f.confidentialID, f.study.Key)
			if err != nil || profileID != f.profileID {
				t.Fatalf("recovered mapping = %q, %v; want %q", profileID, err, f.profileID)
			}
			f.assertLookupUser(t)
			wantSends := 1
			if event.name == "enter existing active participant" {
				wantSends = 0 // Recovery must happen even before the existing-participant early return.
			}
			if f.sends != wantSends {
				t.Fatalf("message rule calls = %d, want %d", f.sends, wantSends)
			}
		})
	}
}

func TestRegisteredProfileEventsAddUserIDToExistingLookup(t *testing.T) {
	for _, event := range registeredProfileEvents() {
		t.Run(event.name, func(t *testing.T) {
			f := newProfileLookupFixture(t, nil)
			// Existing entries from earlier versions contain no userID.
			_, err := f.database.Collection(studydb.COLLECTION_NAME_CONFIDENTIAL_ID_MAP).InsertOne(context.Background(), bson.M{
				"confidentialID": f.confidentialID, "profileID": f.profileID, "studyKey": f.study.Key,
			})
			if err != nil {
				t.Fatal(err)
			}
			entry, err := f.db.GetConfidentialIDMapEntry(profileLookupInstance, f.confidentialID, f.study.Key)
			if err != nil || entry.ProfileID != f.profileID || entry.UserID != "" {
				t.Fatalf("could not read legacy mapping: %+v, %v", entry, err)
			}
			if err := event.run(t, f); err != nil {
				t.Fatal(err)
			}
			f.assertLookupUser(t)
		})
	}
}

func TestRegisteredProfileEventsContinueAfterLookupWriteFailure(t *testing.T) {
	for _, event := range registeredProfileEvents() {
		t.Run(event.name, func(t *testing.T) {
			f := newProfileLookupFixture(t, []studyTypes.Expression{{
				Name: "UPDATE_FLAG", Data: []studyTypes.ExpressionArg{{DType: "str", Str: "event-ran"}, {DType: "str", Str: "yes"}},
			}})
			var logs bytes.Buffer
			oldLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(oldLogger) })
			// Reject only mapping writes; normal event reads and writes can still succeed.
			err := f.database.RunCommand(context.Background(), bson.D{
				{Key: "collMod", Value: studydb.COLLECTION_NAME_CONFIDENTIAL_ID_MAP},
				{Key: "validator", Value: bson.M{"profileID": "reject-real-profile"}},
			}).Err()
			if err != nil {
				t.Fatal(err)
			}
			if err := event.run(t, f); err != nil {
				t.Fatalf("lookup repair failure blocked the event: %v", err)
			}
			if !strings.Contains(logs.String(), "Error ensuring participant ID profile lookup") || !strings.Contains(logs.String(), f.participantID) {
				t.Fatalf("missing lookup failure log with participant context: %s", logs.String())
			}
			_, err = f.db.GetProfileIDFromConfidentialID(profileLookupInstance, f.confidentialID, f.study.Key)
			if !errors.Is(err, mongo.ErrNoDocuments) {
				t.Fatalf("expected mapping write to fail, got %v", err)
			}
			participant, err := f.db.GetParticipantByID(profileLookupInstance, f.study.Key, f.participantID)
			if err != nil {
				t.Fatal(err)
			}
			if event.name == "enter existing active participant" {
				if participant.Flags["event-ran"] != "" {
					t.Fatal("existing active participant should still skip entry rules")
				}
			} else if participant.Flags["event-ran"] != "yes" {
				t.Fatal("study rule changes were not saved after lookup failure")
			}
			if event.name == "leave" && participant.StudyStatus != studyTypes.PARTICIPANT_STUDY_STATUS_EXITED {
				t.Fatal("participant did not leave after lookup failure")
			}
			if event.name == "submission" {
				count, err := f.database.Collection(f.study.Key+"_"+studydb.COLLECTION_NAME_SUFFIX_RESPONSES).CountDocuments(context.Background(), bson.M{"participantID": f.participantID, "key": "survey"})
				if err != nil || count != 1 {
					t.Fatalf("response was not saved after lookup failure: count %d, error %v", count, err)
				}
			}
		})
	}
}

func TestEnsureProfileLookupPreservesExistingMappingAndIsConcurrent(t *testing.T) {
	f := newProfileLookupFixture(t, nil)
	ensure := func(confidentialID, profileID, studyKey string) error {
		return f.db.EnsureConfidentialIDMapEntry(profileLookupInstance, confidentialID, profileID, f.userID, studyKey)
	}
	if err := ensure(f.confidentialID, f.profileID, f.study.Key); err != nil {
		t.Fatal(err)
	}
	f.assertLookupUser(t)
	if err := ensure(f.confidentialID, "different-profile", f.study.Key); !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("expected a conflicting profile mapping to be rejected, got %v", err)
	}
	profileID, err := f.db.GetProfileIDFromConfidentialID(profileLookupInstance, f.confidentialID, f.study.Key)
	if err != nil || profileID != f.profileID {
		t.Fatalf("existing mapping was changed: %q, %v", profileID, err)
	}
	f.assertLookupUser(t)
	if err := ensure(f.confidentialID, "other-study-profile", "other-study"); err != nil {
		t.Fatal(err)
	}
	profileID, err = f.db.GetProfileIDFromConfidentialID(profileLookupInstance, f.confidentialID, "other-study")
	if err != nil || profileID != "other-study-profile" {
		t.Fatalf("mapping was not scoped to study: %q, %v", profileID, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ensure("concurrent-confidential-id", f.profileID, f.study.Key); err != nil {
				t.Errorf("concurrent upsert: %v", err)
			}
		}()
	}
	wg.Wait()
	count, err := f.database.Collection(studydb.COLLECTION_NAME_CONFIDENTIAL_ID_MAP).CountDocuments(context.Background(), bson.M{
		"confidentialID": "concurrent-confidential-id", "studyKey": f.study.Key,
	})
	if err != nil || count != 1 {
		t.Fatalf("concurrent mapping count = %d, %v; want 1", count, err)
	}
}

func TestEventsWithoutRegisteredProfileDoNotCreateLookup(t *testing.T) {
	for _, name := range []string{"temporary registration", "virtual registration", "admin custom event", "admin submission", "timer", "profile deletion"} {
		t.Run(name, func(t *testing.T) {
			f := newProfileLookupFixture(t, []studyTypes.Expression{{
				Name: "IFTHEN", Data: []studyTypes.ExpressionArg{
					{DType: "exp", Exp: &studyTypes.Expression{Name: "checkEventType", Data: []studyTypes.ExpressionArg{{DType: "str", Str: "TIMER"}}}},
					{DType: "exp", Exp: &studyTypes.Expression{Name: "UPDATE_FLAG", Data: []studyTypes.ExpressionArg{{DType: "str", Str: "timer-ran"}, {DType: "str", Str: "yes"}}}},
				},
			}})
			var err error
			switch name {
			case "temporary registration":
				_, err = OnRegisterTempParticipant(profileLookupInstance, f.study.Key)
			case "virtual registration":
				_, err = OnRegisterVirtualParticipant(profileLookupInstance, f.study.Key)
			case "admin custom event":
				_, err = OnCustomStudyEventOnBehalfOfParticipant(profileLookupInstance, f.study.Key, f.participantID, "event", nil)
			case "admin submission":
				_, err = OnSubmitResponseOnBehalfOfParticipant(profileLookupInstance, f.study.Key, f.participantID, studyTypes.SurveyResponse{Key: "survey"}, "admin")
			case "timer":
				OnStudyTimer(profileLookupInstance, &f.study)
				participant, getErr := f.db.GetParticipantByID(profileLookupInstance, f.study.Key, f.participantID)
				if getErr != nil || participant.Flags["timer-ran"] != "yes" {
					t.Fatalf("timer rule did not run: %v, %v", participant.Flags, getErr)
				}
			case "profile deletion":
				if err := f.db.EnsureConfidentialIDMapEntry(profileLookupInstance, f.confidentialID, f.profileID, f.userID, f.study.Key); err != nil {
					t.Fatal(err)
				}
				OnProfileDeleted(profileLookupInstance, f.profileID, nil)
				participant, getErr := f.db.GetParticipantByID(profileLookupInstance, f.study.Key, f.participantID)
				if getErr != nil || participant.StudyStatus != studyTypes.PARTICIPANT_STUDY_STATUS_ACCOUNT_DELETED {
					t.Fatalf("participant was not marked deleted: %q, %v", participant.StudyStatus, getErr)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			count, err := f.database.Collection(studydb.COLLECTION_NAME_CONFIDENTIAL_ID_MAP).CountDocuments(context.Background(), bson.M{})
			if err != nil || count != 0 {
				t.Fatalf("unexpected profile mapping: count %d, error %v", count, err)
			}
		})
	}
}
