package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ssg/ssg-db/client/firestore"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
	"github.com/ssg/ssg-gateway/internal/services"
)

// ServiceHandler handles HTTP requests for service management
type ServiceHandler struct {
	serviceService *services.DiscoveryService
	serviceRepo    repository.ServiceRepository
	endpointRepo   repository.ServiceEndpointRepository
}

// NewServiceHandler creates a new service handler
func NewServiceHandler(discoveryService *services.DiscoveryService, dbClient *firestore.Client) *ServiceHandler {
	return &ServiceHandler{
		serviceService: discoveryService,
		serviceRepo:    dbClient.Service(),
		endpointRepo:   dbClient.ServiceEndpoint(),
	}
}

// GetServices returns all registered services
func (h *ServiceHandler) GetServices(c *gin.Context) {
	services, err := h.serviceRepo.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    services,
	})
}

// GetService returns a specific service by ID
func (h *ServiceHandler) GetService(c *gin.Context) {
	id := c.Param("id")
	service, err := h.serviceRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Service not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    service,
	})
}

// CreateService registers a new service
func (h *ServiceHandler) CreateService(c *gin.Context) {
	var service models.Service
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err := h.serviceRepo.Create(c.Request.Context(), &service); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    service,
	})
}

// UpdateService updates an existing service
func (h *ServiceHandler) UpdateService(c *gin.Context) {
	id := c.Param("id")
	var service models.Service
	if err := c.ShouldBindJSON(&service); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	service.ID = id
	if err := h.serviceRepo.Update(c.Request.Context(), &service); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    service,
	})
}

// DeleteService removes a service
func (h *ServiceHandler) DeleteService(c *gin.Context) {
	id := c.Param("id")
	if err := h.serviceRepo.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Service deleted successfully",
	})
}

// GetServiceEndpoints returns all endpoints for a service
func (h *ServiceHandler) GetServiceEndpoints(c *gin.Context) {
	serviceID := c.Param("serviceID")
	endpoints, err := h.endpointRepo.GetByServiceID(c.Request.Context(), serviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    endpoints,
	})
}

// TriggerDiscovery manually triggers service discovery
func (h *ServiceHandler) TriggerDiscovery(c *gin.Context) {
	if err := h.serviceService.DiscoverAllServices(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Service discovery triggered successfully",
	})
}
