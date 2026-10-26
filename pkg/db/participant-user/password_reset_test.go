package participantuser

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	userTypes "github.com/case-framework/case-backend/pkg/user-management/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestPasswordResetReservation(t *testing.T) {
	uri := os.Getenv("CASE_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set CASE_TEST_MONGO_URI for MongoDB integration tests")
	}
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetServerSelectionTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(ctx) })
	if err = client.Ping(ctx, nil); err != nil {
		t.Fatal(err)
	}
	service := &ParticipantUserDBService{DBClient: client, timeout: 5, DBNamePrefix: "reset_test_" + primitive.NewObjectID().Hex(), InstanceIDs: []string{"test", "other"}}
	for _, instance := range service.InstanceIDs {
		t.Cleanup(func() { _ = client.Database(service.getDBName(instance)).Drop(ctx) })
		if err := service.CreatePasswordResetLimitIndexes(instance); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Unix()
	seed := func(history interface{}, missing bool) userTypes.User {
		t.Helper()
		id := primitive.NewObjectID()
		account := bson.M{"accountID": id.Hex() + "@example.com"}
		if _, err := service.collectionParticipantUsers("test").InsertOne(ctx, bson.M{"_id": id, "account": account}); err != nil {
			t.Fatal(err)
		}
		if !missing {
			expiresAt := now + PasswordResetWindow
			if timestamps, ok := history.([]int64); ok && len(timestamps) > 0 {
				expiresAt = timestamps[len(timestamps)-1] + PasswordResetWindow
			}
			if _, err := service.collectionPasswordResetLimits("test").InsertOne(ctx, bson.M{
				"_id": id, "timestamps": history, "expiresAt": time.Unix(expiresAt, 0),
			}); err != nil {
				t.Fatal(err)
			}
		}
		user, err := service.GetUser("test", id.Hex())
		if err != nil {
			t.Fatal(err)
		}
		return user
	}
	readLimit := func(userID primitive.ObjectID) passwordResetLimit {
		t.Helper()
		var limit passwordResetLimit
		if err := service.collectionPasswordResetLimits("test").FindOne(ctx, bson.M{"_id": userID}).Decode(&limit); err != nil {
			t.Fatal(err)
		}
		return limit
	}
	for _, tc := range []struct {
		name             string
		history          interface{}
		missing, allowed bool
	}{
		{"missing history", nil, true, true}, {"null history", nil, false, true},
		{"inside cooldown", []int64{now - 59}, false, false},
		{"cooldown boundary", []int64{now - 60}, false, true},
		{"fifth request", []int64{now - 300, now - 240, now - 180, now - 120}, false, true},
		{"sixth request", []int64{now - 360, now - 300, now - 240, now - 180, now - 120}, false, false},
		{"hour boundary", []int64{now - 3600, now - 300, now - 240, now - 180, now - 120}, false, true},
		{"old history", []int64{now - 90000, now - 89900, now - 89800, now - 89700, now - 89600, now - 89500}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := seed(tc.history, tc.missing)
			_, err := service.reservePasswordResetAt("test", user.Account.AccountID, now)
			if tc.allowed && err != nil {
				t.Fatal(err)
			}
			if !tc.allowed && !errors.Is(err, ErrPasswordResetRateLimited) {
				t.Fatalf("expected limit rejection, got %v", err)
			}
			limit := readLimit(user.ID)
			if len(limit.Timestamps) > 5 {
				t.Fatalf("unbounded history: %v", limit.Timestamps)
			}
			if tc.allowed && limit.Timestamps[len(limit.Timestamps)-1] != now {
				t.Fatal("reservation not recorded")
			}
			expectedExpiry := limit.Timestamps[len(limit.Timestamps)-1] + PasswordResetWindow
			if limit.ExpiresAt.Unix() != expectedExpiry {
				t.Fatalf("incorrect expiry: %v", limit.ExpiresAt)
			}
			saved, err := service.GetUser("test", user.ID.Hex())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(saved, user) {
				t.Fatal("reservation modified user")
			}
			// Rejection must not change history or push expiration into the future.
			// Retry an already rejected request at the same instant: one second
			// later the 59-second cooldown case is legitimately allowed again.
			retryAt := now
			if tc.allowed {
				retryAt++
			}
			if _, err := service.reservePasswordResetAt("test", user.Account.AccountID, retryAt); !errors.Is(err, ErrPasswordResetRateLimited) {
				t.Fatalf("expected rejection: %v", err)
			}
			if after := readLimit(user.ID); !reflect.DeepEqual(after, limit) {
				t.Fatalf("rejection changed quota: %+v -> %+v", limit, after)
			}
		})
	}
	t.Run("obsolete user history is ignored", func(t *testing.T) {
		user := seed(nil, true)
		if _, err := service.collectionParticipantUsers("test").UpdateOne(ctx, bson.M{"_id": user.ID}, bson.M{
			"$set": bson.M{"account.passwordResetTriggers": bson.A{now, now, now, now, now}},
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := service.reservePasswordResetAt("test", user.Account.AccountID, now); err != nil {
			t.Fatal(err)
		}
		if got := readLimit(user.ID); !reflect.DeepEqual(got.Timestamps, []int64{now}) {
			t.Fatalf("obsolete history was imported: %v", got.Timestamps)
		}
	})
	t.Run("unknown account creates no limit", func(t *testing.T) {
		before, _ := service.collectionPasswordResetLimits("test").CountDocuments(ctx, bson.M{})
		_, err := service.reservePasswordResetAt("test", "unknown@example.com", now)
		if !errors.Is(err, mongo.ErrNoDocuments) {
			t.Fatalf("got %v", err)
		}
		after, _ := service.collectionPasswordResetLimits("test").CountDocuments(ctx, bson.M{})
		if before != after {
			t.Fatal("unknown user created quota document")
		}
	})
	for _, populated := range []bool{false, true} {
		t.Run(map[bool]string{false: "concurrent first requests", true: "concurrent last slot and user writes"}[populated], func(t *testing.T) {
			history := []int64{}
			if populated {
				history = []int64{now - 300, now - 240, now - 180, now - 120}
			}
			user := seed(history, !populated)
			var accepted atomic.Int32
			var wg sync.WaitGroup
			start := make(chan struct{})
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					_, err := service.reservePasswordResetAt("test", user.Account.AccountID, now)
					if err == nil {
						accepted.Add(1)
					} else if !errors.Is(err, ErrPasswordResetRateLimited) {
						t.Errorf("reservation: %v", err)
					}
				}()
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if _, err := service.ReplaceUser("test", user); err != nil {
					t.Error(err)
				}
			}()
			close(start)
			wg.Wait()
			if accepted.Load() != 1 {
				t.Fatalf("accepted %d", accepted.Load())
			}
			limit := readLimit(user.ID)
			if len(limit.Timestamps) != len(history)+1 {
				t.Fatalf("wrong history: %v", limit.Timestamps)
			}
			// A user update cannot erase live collection history.
			user.Account.PreferredLanguage = "de"
			if _, err := service.ReplaceUser("test", user); err != nil {
				t.Fatal(err)
			}
			if after := readLimit(user.ID); !reflect.DeepEqual(limit, after) {
				t.Fatal("user replacement changed reset history")
			}
		})
	}
	t.Run("prune without waiting for TTL then reinitialize", func(t *testing.T) {
		user := seed([]int64{}, false)
		for _, offset := range []int64{0, 60, 120, 180, 240, 3600} {
			if _, err := service.reservePasswordResetAt("test", user.Account.AccountID, now+offset); err != nil {
				t.Fatalf("at %d: %v", offset, err)
			}
		}
		limit := readLimit(user.ID)
		if len(limit.Timestamps) != 5 || limit.Timestamps[0] != now+60 {
			t.Fatalf("old history not pruned: %v", limit.Timestamps)
		}
		if _, err := service.reservePasswordResetAt("test", user.Account.AccountID, now+7200); err != nil {
			t.Fatal(err)
		}
		if got := readLimit(user.ID); !reflect.DeepEqual(got.Timestamps, []int64{now + 7200}) {
			t.Fatalf("expired timestamps retained: %v", got)
		}
		// Simulate TTL deletion after the window and verify safe first-write behavior.
		if _, err := service.collectionPasswordResetLimits("test").DeleteOne(ctx, bson.M{"_id": user.ID}); err != nil {
			t.Fatal(err)
		}
		if _, err := service.reservePasswordResetAt("test", user.Account.AccountID, now+10800); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("same user ID and email isolated by database", func(t *testing.T) {
		user := seed([]int64{now}, false)
		if _, err := service.collectionParticipantUsers("other").InsertOne(ctx, bson.M{"_id": user.ID, "account": bson.M{"accountID": user.Account.AccountID}}); err != nil {
			t.Fatal(err)
		}
		if _, err := service.reservePasswordResetAt("test", user.Account.AccountID, now); !errors.Is(err, ErrPasswordResetRateLimited) {
			t.Fatal(err)
		}
		if _, err := service.reservePasswordResetAt("other", user.Account.AccountID, now); err != nil {
			t.Fatal(err)
		}
	})
	for _, withLimit := range []bool{false, true} {
		name := "delete user without reset history"
		if withLimit {
			name = "delete user with reset history"
		}
		t.Run(name, func(t *testing.T) {
			user := seed(nil, true)
			if err := service.SetUserAttribute("test", user.ID.Hex(), "test", map[string]any{"value": "test"}); err != nil {
				t.Fatal(err)
			}
			if withLimit {
				if _, err := service.reservePasswordResetAt("test", user.Account.AccountID, now); err != nil {
					t.Fatal(err)
				}
			}
			// The same ID in another instance must not be affected.
			if _, err := service.collectionPasswordResetLimits("other").InsertOne(ctx, passwordResetLimit{
				UserID: user.ID, Timestamps: []int64{now}, ExpiresAt: time.Unix(now+PasswordResetWindow, 0),
			}); err != nil {
				t.Fatal(err)
			}
			if err := service.DeleteUser("test", user.ID.Hex()); err != nil {
				t.Fatal(err)
			}
			if _, err := service.GetUser("test", user.ID.Hex()); !errors.Is(err, mongo.ErrNoDocuments) {
				t.Fatalf("user was not deleted: %v", err)
			}
			if attributes, err := service.GetAttributesForUser("test", user.ID.Hex()); err != nil || len(attributes) != 0 {
				t.Fatalf("user attributes were not deleted: %v, error: %v", attributes, err)
			}
			if err := service.collectionPasswordResetLimits("test").FindOne(ctx, bson.M{"_id": user.ID}).Err(); !errors.Is(err, mongo.ErrNoDocuments) {
				t.Fatalf("reset history was not deleted: %v", err)
			}
			if err := service.collectionPasswordResetLimits("other").FindOne(ctx, bson.M{"_id": user.ID}).Err(); err != nil {
				t.Fatalf("other instance affected: %v", err)
			}
		})
	}
	t.Run("TTL and unique indexes", func(t *testing.T) {
		cursor, err := service.collectionPasswordResetLimits("test").Indexes().List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var indexes []bson.M
		if err := cursor.All(ctx, &indexes); err != nil {
			t.Fatal(err)
		}
		foundTTL, foundID := false, false
		for _, index := range indexes {
			if index["name"] == "_id_" {
				foundID = true
			}
			if index["name"] == passwordResetTTLIndex {
				foundTTL = true
				if index["expireAfterSeconds"] != int32(0) {
					t.Fatalf("wrong TTL: %v", index)
				}
			}
		}
		if !foundTTL || !foundID {
			t.Fatalf("missing indexes: %v", indexes)
		}
	})
}
