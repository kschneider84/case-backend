package apihandlers

import (
	"encoding/json"
	"fmt"
	umUtils "github.com/case-framework/case-backend/pkg/user-management/utils"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	jwthandling "github.com/case-framework/case-backend/pkg/jwt-handling"
	"github.com/gin-gonic/gin"
)

func TestProfileHandlersRejectInvalidAlias(t *testing.T) {
	// No database is configured: invalid aliases must be rejected before DB access.
	h := &HttpEndpoints{}
	for _, endpoint := range []struct {
		method  string
		handler gin.HandlerFunc
	}{
		{http.MethodPost, h.addNewProfileHandl},
		{http.MethodPut, h.updateProfileHandl},
	} {
		for _, tc := range []struct{ name, payload, message string }{
			{"long ASCII", `{"alias":"` + strings.Repeat("a", umUtils.ProfileAliasMaxUTF16Length+1) + `"}`, fmt.Sprintf("profile alias must not exceed %d UTF-16 code units", umUtils.ProfileAliasMaxUTF16Length)},
			{"long emoji", `{"alias":"` + strings.Repeat("😀", umUtils.ProfileAliasMaxUTF16Length/2+1) + `"}`, fmt.Sprintf("profile alias must not exceed %d UTF-16 code units", umUtils.ProfileAliasMaxUTF16Length)},
			{"empty", `{"alias":""}`, "profile alias must not be blank"},
			{"spaces", `{"alias":"   "}`, "profile alias must not be blank"},
			{"Unicode whitespace", `{"alias":"\u00a0\t\n"}`, "profile alias must not be blank"},
			{"missing", `{}`, "profile alias must not be blank"},
			{"null", `{"alias":null}`, "profile alias must not be blank"},
		} {
			t.Run(endpoint.method+"/"+tc.name, func(t *testing.T) {
				body, err := json.Marshal(json.RawMessage(tc.payload))
				if err != nil {
					t.Fatal(err)
				}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Set("validatedToken", &jwthandling.ParticipantUserClaims{})
				c.Request = httptest.NewRequest(endpoint.method, "/v1/user/profiles", strings.NewReader(string(body)))
				endpoint.handler(c)
				if w.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, want 400; body: %s", w.Code, w.Body.String())
				}
				var response map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response["error"] != tc.message {
					t.Fatalf("unexpected validation error: %v", response)
				}
			})
		}
	}
}
