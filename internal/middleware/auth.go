package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
	"github.com/ssg/ssg-gateway/internal/services"
)

type FirebaseAuthMiddleware struct {
	firebaseService *services.FirebaseService
	userRepo        repository.UserRepository
	appID           string
	adminEmails     []string
}

func NewFirebaseAuthMiddleware(firebaseService *services.FirebaseService, userRepo repository.UserRepository, appID string, adminEmails []string) *FirebaseAuthMiddleware {
	return &FirebaseAuthMiddleware{
		firebaseService: firebaseService,
		userRepo:        userRepo,
		appID:           appID,
		adminEmails:     adminEmails,
	}
}

func (m *FirebaseAuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Missing authorization header",
				},
			})
			c.Abort()
			return
		}

		idToken := strings.TrimPrefix(authHeader, "Bearer ")
		if idToken == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid authorization format",
				},
			})
			c.Abort()
			return
		}

		token, err := m.firebaseService.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "UNAUTHORIZED",
					"message": "Invalid or expired token",
				},
			})
			c.Abort()
			return
		}

		userEmail := token.Claims["email"].(string)
		userID := token.UID

		c.Set("userID", userID)
		c.Set("userEmail", userEmail)

		userRole, err := m.userRepo.GetUserAppRole(context.Background(), userID, m.appID)
		if err != nil || userRole == nil {
			userRole = m.checkAutoProvision(userID, userEmail)
		}

		if userRole != nil {
			c.Set("userRole", userRole.Role)
		} else {
			c.Set("userRole", "viewer")
		}

		// =========================================================================
        // PROPAGAZIONE NEXUS (Nuovo blocco per i microservizi)
        // =========================================================================
        // Inseriamo i claim validati negli header nativi della HTTP Request.
        // Il reverse proxy li inoltrerà automaticamente ai microservizi downstream.
        c.Request.Header.Set("X-Nexus-User-ID", userID)
        c.Request.Header.Set("X-Nexus-Role", c.GetString("userRole"))

        // Gestione del Trace ID per il logging distribuito (opzionale ma consigliato)
        traceID := c.GetHeader("X-Cloud-Trace-Context") // Header standard di GCP
        if traceID == "" {
            // Se non c'è, ne creiamo uno semplice (in prod meglio usare UUID)
            traceID = fmt.Sprintf("ssg-trace-%d", time.Now().UnixNano())
        }
        c.Request.Header.Set("X-Nexus-Trace-ID", traceID)
        // =========================================================================

		c.Next()
	}
}

func (m *FirebaseAuthMiddleware) checkAutoProvision(userID, userEmail string) *models.AppRole {
	emailLower := strings.ToLower(userEmail)

	for _, adminEmail := range m.adminEmails {
		if strings.ToLower(adminEmail) == emailLower {
			role := "admin"
			existingUser, err := m.userRepo.GetByID(context.Background(), userID)
			if err != nil || existingUser == nil {
				user := &models.User{
					ID:    userID,
					Email: userEmail,
					Apps: map[string]models.AppRole{
						m.appID: {Role: role, AddedAt: time.Now()},
					},
				}
				if createErr := m.userRepo.Create(context.Background(), user); createErr != nil {
					return nil
				}
				return &models.AppRole{Role: role}
			}
			setErr := m.userRepo.SetUserAppRole(context.Background(), userID, m.appID, role)
			if setErr == nil {
				return &models.AppRole{Role: role}
			}
		}
	}

	return nil
}

func (m *FirebaseAuthMiddleware) RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("userRole")
		if !exists {
			userRole = "viewer"
		}

		roleStr, ok := userRole.(string)
		if !ok {
			roleStr = "viewer"
		}

		for _, role := range allowedRoles {
			if role == roleStr {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "FORBIDDEN",
				"message": "Insufficient permissions",
			},
		})
		c.Abort()
	}
}
