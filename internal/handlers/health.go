package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ssg/ssg-gateway/internal/models"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data: models.HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now().UTC(),
			Services: map[string]string{
				"gateway":  "ok",
				"firebase": "ok",
			},
		},
	})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}
