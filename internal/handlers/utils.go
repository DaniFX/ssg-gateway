package handlers

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// HandleError logs the real error to the server logs and returns a safe JSON error response to the client.
func HandleError(c *gin.Context, status int, code, logMessage, userMessage string, err error) {
	if err != nil {
		slog.Error(logMessage,
			"code", code,
			"error", err.Error(),
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
		)
	} else {
		slog.Error(logMessage,
			"code", code,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
		)
	}

	c.JSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": userMessage,
		},
	})
}
