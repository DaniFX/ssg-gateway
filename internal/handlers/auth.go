package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ssg/ssg-gateway/internal/services"
)

type AuthHandler struct {
	firebaseService *services.FirebaseService
	appID           string
	adminEmails     []string
}

func NewAuthHandler(firebaseService *services.FirebaseService, appID string, adminEmails []string) *AuthHandler {
	return &AuthHandler{
		firebaseService: firebaseService,
		appID:           appID,
		adminEmails:     adminEmails,
	}
}

type LoginRequest struct {
	IDToken string `json:"idToken" binding:"required"`
}

type LoginResponse struct {
	UID   string `json:"uid"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to bind login request", "Missing idToken", err)
		return
	}

	token, err := h.firebaseService.VerifyIDToken(c.Request.Context(), req.IDToken)
	if err != nil {
		HandleError(c, http.StatusUnauthorized, "INVALID_TOKEN", "Failed to verify Firebase ID token", "Invalid Firebase ID token", err)
		return
	}

	userID := token.UID
	userEmail := token.Claims["email"].(string)

	userRole, err := h.firebaseService.GetUserAppRole(c.Request.Context(), userID, h.appID)
	if err != nil || userRole == nil {
		userRole = h.checkAutoProvision(c.Request.Context(), userID, userEmail)
	}

	role := "viewer"
	if userRole != nil {
		role = userRole.Role
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": LoginResponse{
			UID:   userID,
			Email: userEmail,
			Role:  role,
		},
	})
}

func (h *AuthHandler) checkAutoProvision(ctx context.Context, userID, userEmail string) *services.UserAppRole {
	emailLower := strings.ToLower(userEmail)

	for _, adminEmail := range h.adminEmails {
		if strings.ToLower(adminEmail) == emailLower {
			role := "admin"
			err := h.firebaseService.SetUserAppRole(ctx, userID, h.appID, role)
			if err == nil {
				return &services.UserAppRole{Role: role}
			}
		}
	}

	return nil
}
