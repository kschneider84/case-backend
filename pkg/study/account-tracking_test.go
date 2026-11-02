package study

import (
	"reflect"
	"testing"

	studyTypes "github.com/case-framework/case-backend/pkg/study/types"
	studyUtils "github.com/case-framework/case-backend/pkg/study/utils"
	umTypes "github.com/case-framework/case-backend/pkg/user-management/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestAccountGroupingUsesStableUserID(t *testing.T) {
	oldSecret := globalSecret
	globalSecret = "test-global-secret"
	t.Cleanup(func() { globalSecret = oldSecret })
	s := studyTypes.Study{SecretKey: "study-secret", Configs: studyTypes.StudyConfigs{IdMappingMethod: "sha256-b64"}}
	user := umTypes.User{ID: primitive.NewObjectID(), Account: umTypes.Account{AccountID: "old@example.org"}}
	before, err := ComputeHashedAccountID(s, user.ID.Hex())
	if err != nil {
		t.Fatal(err)
	}
	user.Account.AccountID = "new@example.org"
	after, err := ComputeHashedAccountID(s, user.ID.Hex())
	if err != nil || before != after {
		t.Fatalf("email change altered grouping: %q -> %q, %v", before, after, err)
	}
	want, err := studyUtils.ProfileIDtoParticipantID(user.ID.Hex(), globalSecret, s.SecretKey, s.Configs.IdMappingMethod)
	if err != nil || before != want {
		t.Fatalf("grouping = %q, want stable-ID pseudonym %q; %v", before, want, err)
	}
	otherUser, err := ComputeHashedAccountID(s, primitive.NewObjectID().Hex())
	if err != nil || otherUser == before {
		t.Fatalf("different user has same grouping: %q, %v", otherUser, err)
	}
	s.SecretKey = "other-study-secret"
	otherStudy, err := ComputeHashedAccountID(s, user.ID.Hex())
	if err != nil || otherStudy == before {
		t.Fatalf("different study has same grouping: %q, %v", otherStudy, err)
	}
	if _, err := ComputeHashedAccountID(s, ""); err == nil {
		t.Fatal("empty user ID must not produce an account grouping")
	}
}

func TestEnrollmentGroupsProfilesByStableUserID(t *testing.T) {
	for _, tracking := range []bool{true, false} {
		name := "enabled"
		if !tracking {
			name = "disabled"
		}
		t.Run(name, func(t *testing.T) {
			f := newProfileLookupFixture(t, nil)
			f.study.Configs.TrackAccount = tracking
			err := f.db.UpdateStudyTrackAccount(profileLookupInstance, f.study.Key, tracking)
			if err != nil {
				t.Fatal(err)
			}
			userID := primitive.NewObjectID().Hex()
			var group string
			for i := 0; i < 2; i++ {
				profileID := primitive.NewObjectID().Hex()
				if _, err := OnEnterStudy(profileLookupInstance, f.study.Key, profileID, userID, i == 0); err != nil {
					t.Fatal(err)
				}
				participantID, _, err := ComputeParticipantIDs(f.study, profileID)
				if err != nil {
					t.Fatal(err)
				}
				p, err := f.db.GetParticipantByID(profileLookupInstance, f.study.Key, participantID)
				if err != nil {
					t.Fatal(err)
				}
				if !tracking {
					if p.HashedAccountID != nil || p.IsMainProfile != nil {
						t.Fatal("tracking fields were stored while account tracking was disabled")
					}
					continue
				}
				want, err := ComputeHashedAccountID(f.study, userID)
				if err != nil || p.HashedAccountID == nil || *p.HashedAccountID != want {
					t.Fatalf("enrollment did not store stable user grouping: %+v, %v", p, err)
				}
				if p.IsMainProfile == nil || *p.IsMainProfile != (i == 0) {
					t.Fatal("main/other profile distinction was lost")
				}
				if i == 1 && *p.HashedAccountID != group {
					t.Fatal("profiles of the same user received different groupings")
				}
				group = *p.HashedAccountID
			}
		})
	}
}

func TestAccountInfoMigrationPreservesParticipantStateAndIsIdempotent(t *testing.T) {
	for _, status := range []string{studyTypes.PARTICIPANT_STUDY_STATUS_ACTIVE, studyTypes.PARTICIPANT_STUDY_STATUS_EXITED} {
		for _, legacy := range []bool{false, true} {
			name := status + "/missing"
			if legacy {
				name = status + "/email-derived"
			}
			t.Run(name, func(t *testing.T) {
				f := newProfileLookupFixture(t, nil)
				p, err := f.db.GetParticipantByID(profileLookupInstance, f.study.Key, f.participantID)
				if err != nil {
					t.Fatal(err)
				}
				p.StudyStatus = status
				p.Flags = map[string]string{"keep": "yes"}
				p.Messages = []studyTypes.ParticipantMessage{{ID: "pending-message", Type: "reminder", ScheduledFor: 123}}
				p.AssignedSurveys = []studyTypes.AssignedSurvey{{SurveyKey: "weekly"}}
				if legacy {
					oldHash, err := studyUtils.ProfileIDtoParticipantID("old@example.org", globalSecret, f.study.SecretKey, f.study.Configs.IdMappingMethod)
					if err != nil {
						t.Fatal(err)
					}
					p.HashedAccountID = &oldHash
				}
				p, err = f.db.SaveParticipantState(profileLookupInstance, f.study.Key, p)
				if err != nil {
					t.Fatal(err)
				}
				want, err := ComputeHashedAccountID(f.study, primitive.NewObjectID().Hex())
				if err != nil {
					t.Fatal(err)
				}
				updated, err := f.db.UpdateParticipantAccountInfo(profileLookupInstance, f.study.Key, f.participantID, want, true)
				if err != nil || !updated {
					t.Fatalf("migration did not update participant: %v, %v", updated, err)
				}
				after, err := f.db.GetParticipantByID(profileLookupInstance, f.study.Key, f.participantID)
				if err != nil {
					t.Fatal(err)
				}
				mainProfile := true
				p.HashedAccountID, p.IsMainProfile, p.ModifiedAt = &want, &mainProfile, after.ModifiedAt
				if !reflect.DeepEqual(after, p) {
					t.Fatalf("migration changed unrelated participant fields:\ngot  %+v\nwant %+v", after, p)
				}
				updated, err = f.db.UpdateParticipantAccountInfo(profileLookupInstance, f.study.Key, f.participantID, want, true)
				if err != nil || updated {
					t.Fatalf("repeat migration should make no changes: %v, %v", updated, err)
				}
			})
		}
	}
}

func TestAccountInfoMigrationSkipsMissingAndNonRegisteredParticipants(t *testing.T) {
	f := newProfileLookupFixture(t, nil)
	for _, status := range []string{studyTypes.PARTICIPANT_STUDY_STATUS_ACCOUNT_DELETED, studyTypes.PARTICIPANT_STUDY_STATUS_TEMPORARY, studyTypes.PARTICIPANT_STUDY_STATUS_VIRTUAL} {
		f.saveParticipant(t, f.participantID, status)
		updated, err := f.db.UpdateParticipantAccountInfo(profileLookupInstance, f.study.Key, f.participantID, "new-grouping", true)
		if err != nil || updated {
			t.Fatalf("migration updated %s participant: %v, %v", status, updated, err)
		}
		p, err := f.db.GetParticipantByID(profileLookupInstance, f.study.Key, f.participantID)
		if err != nil || p.HashedAccountID != nil || p.IsMainProfile != nil || p.StudyStatus != status {
			t.Fatalf("migration altered %s participant: %+v, %v", status, p, err)
		}
	}
	updated, err := f.db.UpdateParticipantAccountInfo(profileLookupInstance, f.study.Key, "missing", "new-grouping", true)
	if err != nil || updated {
		t.Fatalf("migration updated missing participant: %v, %v", updated, err)
	}
	if _, err := f.db.GetParticipantByID(profileLookupInstance, f.study.Key, "missing"); err != mongo.ErrNoDocuments {
		t.Fatalf("migration created a missing participant: %v", err)
	}
}
