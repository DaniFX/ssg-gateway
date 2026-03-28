package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ssg/ssg-gateway/internal/services"
)

type LogsHandler struct {
	loggingService *services.LoggingService
}

func NewLogsHandler(loggingService *services.LoggingService) *LogsHandler {
	return &LogsHandler{
		loggingService: loggingService,
	}
}

// GetLogs returns GCP logs matching an advanced filter query
func (h *LogsHandler) GetLogs(c *gin.Context) {
	if h.loggingService == nil {
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Logging service is not initialized", "Logging feature is unavailable", errors.New("logging_service_nil"))
		return
	}

	filter := c.Query("filter")
	limitStr := c.Query("limit")
	limit := 100 // default

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	logs, err := h.loggingService.GetLogs(c.Request.Context(), filter, limit)
	if err != nil {
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to query logs from GCP", "Failed to retrieve logs", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
	})
}
