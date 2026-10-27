package apihandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/case-framework/case-backend/pkg/db"
	globalinfos "github.com/case-framework/case-backend/pkg/db/global-infos"
	participantuser "github.com/case-framework/case-backend/pkg/db/participant-user"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestPasswordResetResponses(t *testing.T) {
	uri := os.Getenv("CASE_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set CASE_TEST_MONGO_URI to run MongoDB integration tests")
	}
	ctx := context.Background()
	prefix := "reset_http_test_" + primitive.NewObjectID().Hex()
	service, err := participantuser.NewParticipantUserDBService(db.DBConfig{URI: uri, Timeout: 5, DBNamePrefix: prefix, InstanceIDs: []string{"test"}})
	if err != nil {
		t.Fatal(err)
	}

	database := service.DBClient.Database(prefix + "test_users")
	t.Cleanup(func() { _ = database.Drop(ctx); _ = service.DBClient.Disconnect(ctx) })
	// Force token creation to fail safely: no real email is sent. The accepted
	// request must still retain its quota after this downstream failure.
	disconnected, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	if err = disconnected.Disconnect(ctx); err != nil {
		t.Fatal(err)
	}
	h := &HttpEndpoints{userDBConn: service, allowedInstanceIDs: []string{"test"}, globalInfosDBConn: &globalinfos.GlobalInfosDBService{DBClient: disconnected}}
	now := time.Now().Unix()
	for email, history := range map[string][]int64{
		"accepted@example.com": nil,
		"cooldown@example.com": {now},
		"hourly@example.com":   {now - 300, now - 240, now - 180, now - 120, now - 60},
	} {
		id := primitive.NewObjectID()
		if _, err = database.Collection("users").InsertOne(ctx, bson.M{"_id": id, "account": bson.M{"accountID": email}}); err != nil {
			t.Fatal(err)
		}
		if history != nil {
			if _, err = database.Collection("passwordResetLimits").InsertOne(ctx, bson.M{
				"_id": id, "timestamps": history, "expiresAt": time.Unix(history[len(history)-1]+participantuser.PasswordResetWindow, 0),
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		email        string
		status       int
		key, message string
	}{
		{"accepted@example.com", http.StatusOK, "message", "password reset initiated"},
		{"unknown@example.com", http.StatusInternalServerError, "error", "internal server error"},
		{"cooldown@example.com", http.StatusTooManyRequests, "error", "rate limited"},
		{"hourly@example.com", http.StatusTooManyRequests, "error", "rate limited"},
		{"db-error@example.com", http.StatusInternalServerError, "error", "internal server error"},
	} {
		email := tc.email
		t.Run(email, func(t *testing.T) {
			if email == "db-error@example.com" {
				h.userDBConn = &participantuser.ParticipantUserDBService{DBClient: disconnected, InstanceIDs: []string{"test"}}
			}
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/password-reset/initiate", strings.NewReader(`{"instanceID":"test","email":"`+email+`"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			start := time.Now()
			h.initiatePasswordReset(c)
			elapsed := time.Since(start)
			if recorder.Code != tc.status {
				t.Fatalf("status %d", recorder.Code)
			}
			if elapsed < time.Second || elapsed > 4500*time.Millisecond {
				t.Fatalf("unexpected response duration %s", elapsed)
			}
			var body map[string]string
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || len(body) != 1 || body[tc.key] != tc.message {
				t.Fatalf("unexpected response %s", recorder.Body.String())
			}
		})
	}
	for email, want := range map[string]int{"accepted@example.com": 1, "cooldown@example.com": 1, "hourly@example.com": 5} {
		user, err := service.GetUserByAccountID("test", email)
		if err != nil {
			t.Fatal(err)
		}
		var limit struct {
			Timestamps []int64 `bson:"timestamps"`
		}
		if err := database.Collection("passwordResetLimits").FindOne(ctx, bson.M{"_id": user.ID}).Decode(&limit); err != nil {
			t.Fatal(err)
		}
		if len(limit.Timestamps) != want {
			t.Fatalf("quota after downstream failure/rejection: %v", limit.Timestamps)
		}
	}
}
