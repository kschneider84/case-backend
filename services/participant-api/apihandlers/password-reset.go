package apihandlers

import (
	"errors"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	mw "github.com/case-framework/case-backend/pkg/apihelpers/middlewares"
	userDB "github.com/case-framework/case-backend/pkg/db/participant-user"
	"github.com/case-framework/case-backend/pkg/user-management/pwhash"
	userTypes "github.com/case-framework/case-backend/pkg/user-management/types"
	umUtils "github.com/case-framework/case-backend/pkg/user-management/utils"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	emailTypes "github.com/case-framework/case-backend/pkg/messaging/types"
)

const (
	PASSWORD_RESET_TOKEN_TTL = 24 * time.Hour
)

func (h *HttpEndpoints) AddPasswordResetAPI(rg *gin.RouterGroup) {
	pwResetGroup := rg.Group("/password-reset")
	{
		pwResetGroup.POST("/initiate", mw.RequirePayload(), h.initiatePasswordReset)
		pwResetGroup.POST("/get-infos", mw.RequirePayload(), h.getPasswordResetInfos)
		pwResetGroup.POST("/reset", mw.RequirePayload(), h.resetPassword)
	}
}

func (h *HttpEndpoints) initiatePasswordReset(c *gin.Context) {
	responseAt := time.Now().Add(time.Duration(1+rand.Intn(3)) * time.Second)
	var req struct {
		Email      string `json:"email"`
		InstanceID string `json:"instanceID"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("bad request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.isInstanceAllowed(req.InstanceID) {
		slog.Error("instance not allowed", slog.String("instanceID", req.InstanceID))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid instance id"})
		return
	}

	req.Email = umUtils.SanitizeEmail(req.Email)

	// Send every outcome after the same target duration, allowing for DB processing
	// time. Operations exceeding the target duration are answered when they finish.
	status := http.StatusOK
	response := gin.H{"message": "password reset initiated"}
	defer func() {
		time.Sleep(time.Until(responseAt))
		c.JSON(status, response)
	}()

	user, err := h.userDBConn.ReservePasswordReset(req.InstanceID, req.Email)
	if errors.Is(err, mongo.ErrNoDocuments) {
		slog.Warn("password reset not accepted: non-existing account", slog.String("email", req.Email), slog.String("instanceID", req.InstanceID))
		status, response = http.StatusInternalServerError, gin.H{"error": "internal server error"}
		return
	}
	if errors.Is(err, userDB.ErrPasswordResetRateLimited) {
		slog.Warn("password reset rate limited", slog.String("email", req.Email), slog.String("instanceID", req.InstanceID))
		status, response = http.StatusTooManyRequests, gin.H{"error": "rate limited"}
		return
	}
	if err != nil {
		slog.Error("password reset reservation failed", slog.String("instanceID", req.InstanceID), slog.String("error", err.Error()))
		status, response = http.StatusInternalServerError, gin.H{"error": "internal server error"}
		return
	}

	h.sendPasswordResetEmail(req.InstanceID, user)

	slog.Info("password reset accepted", slog.String("userID", user.ID.Hex()), slog.String("instanceID", req.InstanceID))
}

func (h *HttpEndpoints) sendPasswordResetEmail(instanceID string, user userTypes.User) {
	go h.prepTokenAndSendEmail(
		user.ID.Hex(),
		instanceID,
		user.Account.AccountID,
		user.Account.PreferredLanguage,
		userTypes.TOKEN_PURPOSE_PASSWORD_RESET,
		PASSWORD_RESET_TOKEN_TTL,
		emailTypes.EMAIL_TYPE_PASSWORD_RESET,
		map[string]string{
			"validUntil": strconv.FormatInt(int64(PASSWORD_RESET_TOKEN_TTL/time.Hour), 10),
		},
	)
}

func (h *HttpEndpoints) getPasswordResetInfos(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("bad request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	tokenInfos, err := h.validateTempToken(
		req.Token, []string{
			userTypes.TOKEN_PURPOSE_PASSWORD_RESET,
			userTypes.TOKEN_PURPOSE_INVITATION,
		})
	if err != nil {
		slog.Error("invalid token", slog.String("error", err.Error()))
		randomWait(5, 10)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}

	user, err := h.userDBConn.GetUser(tokenInfos.InstanceID, tokenInfos.UserID)
	if err != nil {
		slog.Error("failed to get user", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accountId": user.Account.AccountID,
	})
}

func (h *HttpEndpoints) resetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("missing or invalid request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Token == "" {
		randomWait(5, 10)
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	if !umUtils.CheckPasswordFormat(req.NewPassword) {
		slog.Error("invalid password format")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password format"})
		return
	}

	if umUtils.IsPasswordOnBlocklist(req.NewPassword) {
		slog.Error("password on blocklist")
		c.JSON(http.StatusBadRequest, gin.H{"error": "password on blocklist"})
		return
	}

	tokenInfos, err := h.validateTempToken(
		req.Token, []string{
			userTypes.TOKEN_PURPOSE_PASSWORD_RESET,
			userTypes.TOKEN_PURPOSE_INVITATION,
		})
	if err != nil {
		slog.Error("invalid token", slog.String("error", err.Error()))
		randomWait(5, 10)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token"})
		return
	}

	user, err := h.userDBConn.GetUser(tokenInfos.InstanceID, tokenInfos.UserID)
	if err != nil {
		slog.Error("failed to get user", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	password, err := pwhash.HashPassword(req.NewPassword)
	if err != nil {
		slog.Error("failed to hash password", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	update := bson.M{"$set": bson.M{"account.password": password, "timestamps.lastPasswordChange": time.Now().Unix()}}
	err = h.userDBConn.UpdateUser(tokenInfos.InstanceID, user.ID.Hex(), update)
	if err != nil {
		slog.Error("failed to update user", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if tokenInfos.Purpose == userTypes.TOKEN_PURPOSE_INVITATION {
		newContactPrefs := user.ContactPreferences
		newContactPrefs.SubscribedToNewsletter = true
		newContactPrefs.SubscribedToWeekly = true
		contactUpdate := bson.M{"$set": bson.M{"contactPreferences": newContactPrefs, "timestamps.updatedAt": time.Now().Unix()}}
		err := h.userDBConn.UpdateUser(tokenInfos.InstanceID, user.ID.Hex(), contactUpdate)
		if err != nil {
			slog.Error("failed to update contact preferences", slog.String("error", err.Error()))
		}
	}

	go h.sendSimpleEmail(
		tokenInfos.InstanceID,
		[]string{user.Account.AccountID},
		user.ID.Hex(),
		emailTypes.EMAIL_TYPE_PASSWORD_CHANGED,
		"",
		user.Account.PreferredLanguage,
		nil,
		true,
	)

	slog.Info("password reset successful", slog.String("userID", user.ID.Hex()), slog.String("instanceID", tokenInfos.InstanceID))

	if err := h.globalInfosDBConn.DeleteAllTempTokenForUser(tokenInfos.InstanceID, user.ID.Hex(), userTypes.TOKEN_PURPOSE_PASSWORD_RESET); err != nil {
		slog.Error("failed to delete temp token", slog.String("error", err.Error()))
	}

	c.JSON(http.StatusOK, gin.H{"message": "password reset successful"})
}
