package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
)

type AppHandler struct {
	appRepo repository.AppRepository
}

func NewAppHandler(appRepo repository.AppRepository) *AppHandler {
	return &AppHandler{
		appRepo: appRepo,
	}
}

func (h *AppHandler) ListApps(c *gin.Context) {
	apps, err := h.appRepo.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to fetch apps",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apps,
	})
}

func (h *AppHandler) GetApp(c *gin.Context) {
	appID := c.Param("id")

	app, err := h.appRepo.GetByID(c.Request.Context(), appID)
	if err != nil || app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "NOT_FOUND",
				"message": "App not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    app,
	})
}

func (h *AppHandler) CreateApp(c *gin.Context) {
	var app models.App
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request body",
			},
		})
		return
	}

	if app.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "App ID is required",
			},
		})
		return
	}

	err := h.appRepo.Create(c.Request.Context(), &app)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to create app",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    app,
	})
}

func (h *AppHandler) UpdateApp(c *gin.Context) {
	appID := c.Param("id")

	var app models.App
	if err := c.ShouldBindJSON(&app); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": "Invalid request body",
			},
		})
		return
	}

	app.ID = appID

	err := h.appRepo.Update(c.Request.Context(), &app)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to update app",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    app,
	})
}

func (h *AppHandler) DeleteApp(c *gin.Context) {
	appID := c.Param("id")

	err := h.appRepo.Delete(c.Request.Context(), appID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to delete app",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"message": "App deleted"},
	})
}
