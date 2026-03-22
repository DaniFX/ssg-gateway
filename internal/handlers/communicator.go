package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ssg/ssg-gateway/internal/services"
)

type CommunicatorHandler struct {
	communicatorClient *services.CommunicatorClient
}

func NewCommunicatorHandler(communicatorClient *services.CommunicatorClient) *CommunicatorHandler {
	return &CommunicatorHandler{
		communicatorClient: communicatorClient,
	}
}

type SendEmailRequest struct {
	To      string `json:"to" binding:"required"`
	Subject string `json:"subject" binding:"required"`
	Body    string `json:"body" binding:"required"`
	IsHTML  bool   `json:"is_html"`
}

func (h *CommunicatorHandler) SendEmail(c *gin.Context) {
	var req SendEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	messageID, err := h.communicatorClient.SendEmail(c.Request.Context(), req.To, req.Subject, req.Body, req.IsHTML)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Email sent successfully",
		"message_id": messageID,
	})
}
