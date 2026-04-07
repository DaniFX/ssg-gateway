package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ssg/ssg-gateway/internal/services"
)

type AuthHandler struct {
	firebaseService *services.FirebaseService
	appID           string
	adminEmails     []string
	webAPIKey       string // Necessario per chiedere il token a Firebase
}

// Aggiunto il parametro webAPIKey
func NewAuthHandler(firebaseService *services.FirebaseService, appID string, adminEmails []string, webAPIKey string) *AuthHandler {
	return &AuthHandler{
		firebaseService: firebaseService,
		appID:           appID,
		adminEmails:     adminEmails,
		webAPIKey:       webAPIKey,
	}
}

// Ora accettiamo email e password direttamente!
type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"` // Il JWT pronto da usare nelle chiamate successive
	UID   string `json:"uid"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Struttura per decodificare la risposta interna di Firebase
type firebaseVerifyPasswordResponse struct {
	IDToken string `json:"idToken"`
	LocalID string `json:"localId"`
	Email   string `json:"email"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Email e password sono obbligatorie"})
		return
	}

	if h.webAPIKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Server misconfigured: missing Firebase Web API Key"})
		return
	}

	// 1. Il Gateway scambia Email/Password con un JWT chiamando le API di Google
	authURL := fmt.Sprintf("https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=%s", h.webAPIKey)
	payload, _ := json.Marshal(map[string]interface{}{
		"email":             req.Email,
		"password":          req.Password,
		"returnSecureToken": true,
	})

	resp, err := http.Post(authURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "Errore di comunicazione con il provider di autenticazione"})
		return
	}
	defer resp.Body.Close()

	var fbResp firebaseVerifyPasswordResponse
	if err := json.NewDecoder(resp.Body).Decode(&fbResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Errore nella decodifica del token"})
		return
	}

	if fbResp.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Credenziali non valide", "details": fbResp.Error.Message})
		return
	}

	userID := fbResp.LocalID
	userEmail := fbResp.Email

	// 2. Determiniamo il ruolo (Admin, Viewer, ecc.)
	userRole, err := h.firebaseService.GetUserAppRole(c.Request.Context(), userID, h.appID)
	if err != nil || userRole == nil {
		userRole = h.checkAutoProvision(c.Request.Context(), userID, userEmail)
	}

	role := "viewer"
	if userRole != nil {
		role = userRole.Role
	}

	// 3. Restituiamo il Token pronto per l'uso ai servizi esterni o al frontend!
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": LoginResponse{
			Token: fbResp.IDToken,
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
