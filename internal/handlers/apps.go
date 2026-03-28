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
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch apps from repository", "Failed to fetch apps", err)
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
		HandleError(c, http.StatusNotFound, "NOT_FOUND", "App not found in repository", "App not found", err)
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
		HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to bind app JSON", "Invalid request body", err)
		return
	}

	if app.ID == "" {
		HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "App ID is missing in create request", "App ID is required", nil)
		return
	}

	err := h.appRepo.Create(c.Request.Context(), &app)
	if err != nil {
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create app in repository", "Failed to create app", err)
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
		HandleError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to bind app JSON for update", "Invalid request body", err)
		return
	}

	app.ID = appID

	err := h.appRepo.Update(c.Request.Context(), &app)
	if err != nil {
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update app in repository", "Failed to update app", err)
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
		HandleError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete app from repository", "Failed to delete app", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"message": "App deleted"},
	})
}
