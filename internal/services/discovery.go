package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	firestore "github.com/ssg/ssg-db/client/firestore"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
	"github.com/ssg/ssg-gateway/internal/config"
)

// DiscoveryService handles discovering services and their endpoints
type DiscoveryService struct {
	dbClient          *firestore.Client
	cfg               *config.Config
	httpClient        *http.Client
	discoveryInterval time.Duration
	stopChan          chan struct{}
	serviceRepo       repository.ServiceRepository
	endpointRepo      repository.ServiceEndpointRepository
	updateCallback    func() error
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(dbClient *firestore.Client, cfg *config.Config, updateCallback func() error) *DiscoveryService {
	fc := dbClient.GetClient()
	return &DiscoveryService{
		dbClient:          dbClient,
		cfg:               cfg,
		httpClient:        &http.Client{Timeout: 10 * time.Second},
		discoveryInterval: 30 * time.Second, // Discover every 30 seconds
		stopChan:          make(chan struct{}),
		serviceRepo:       firestore.NewServiceRepository(fc),
		endpointRepo:      firestore.NewServiceEndpointRepository(fc),
		updateCallback:    updateCallback,
	}
}

// Start begins the discovery process
func (s *DiscoveryService) Start(ctx context.Context) error {
	// Initial discovery
	if err := s.DiscoverAllServices(ctx); err != nil {
		return fmt.Errorf("initial service discovery failed: %w", err)
	}

	// Start periodic discovery
	ticker := time.NewTicker(s.discoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopChan:
			return nil
		case <-ticker.C:
			if err := s.DiscoverAllServices(ctx); err != nil {
				// Log error but continue discovery
				// In a real implementation, you'd want proper logging
				fmt.Printf("Service discovery error: %v\n", err)
			}
		}
	}
}

// Stop halts the discovery process
func (s *DiscoveryService) Stop() {
	close(s.stopChan)
}

// DiscoverAllServices discovers all registered services
func (s *DiscoveryService) DiscoverAllServices(ctx context.Context) error {
	// Get all active services from Firestore
	services, err := s.serviceRepo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active services: %w", err)
	}

	// For each service, discover its endpoints
	for _, service := range services {
		if err := s.discoverServiceEndpoints(ctx, service); err != nil {
			// Log error but continue with other services
			fmt.Printf("Failed to discover endpoints for service %s: %v\n", service.Name, err)
		}
	}

	return nil
}

// discoverServiceEndpoints discovers endpoints for a specific service
func (s *DiscoveryService) discoverServiceEndpoints(ctx context.Context, service models.Service) error {
	// Skip if service URL is not set
	if service.URL == "" {
		return fmt.Errorf("service %s has no URL configured", service.Name)
	}

	// Call the service's discovery endpoint
	discoveryURL := fmt.Sprintf("%s/_discover", service.URL)
	resp, err := s.httpClient.Get(discoveryURL)
	if err != nil {
		return fmt.Errorf("failed to call discovery endpoint %s: %w", discoveryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discovery endpoint returned status %d", resp.StatusCode)
	}

	// Parse the discovery response
	var discoveryResp ServiceDiscoveryResponse
	if err := json.NewDecoder(resp.Body).Decode(&discoveryResp); err != nil {
		return fmt.Errorf("failed to decode discovery response: %w", err)
	}

	// Validate the response
	if discoveryResp.ServiceName != service.Name {
		return fmt.Errorf("service name mismatch: expected %s, got %s", service.Name, discoveryResp.ServiceName)
	}

	// Update service metadata and version
	service.Description = discoveryResp.Description
	service.Version = discoveryResp.Version
	service.Metadata = discoveryResp.Metadata
	if err := s.serviceRepo.Update(ctx, &service); err != nil {
		return fmt.Errorf("failed to update service: %w", err)
	}

	// Deactivate all existing endpoints for this service
	existingEndpoints, err := s.endpointRepo.GetByServiceID(ctx, service.ID)
	if err != nil {
		return fmt.Errorf("failed to get existing endpoints: %w", err)
	}
	for _, endpoint := range existingEndpoints {
		if err := s.endpointRepo.Deactivate(ctx, endpoint.ID); err != nil {
			fmt.Printf("Failed to deactivate endpoint %s: %v\n", endpoint.ID, err)
			// Continue with other endpoints
		}
	}

	// Create new endpoints from discovery response
	for _, endpointSpec := range discoveryResp.Endpoints {
		endpoint := models.ServiceEndpoint{
			ID:                         generateEndpointID(service.ID, endpointSpec.Path, endpointSpec.Method),
			ServiceID:                  service.ID,
			Path:                       endpointSpec.Path,
			Method:                     endpointSpec.Method,
			Summary:                    endpointSpec.Summary,
			InputSchema:                endpointSpec.InputSchema,
			OutputSchema:               endpointSpec.OutputSchema,
			AuthRequired:               endpointSpec.AuthRequired,
			RateLimitRequestsPerMinute: endpointSpec.RateLimit.RequestsPerMinute,
			RateLimitBurst:             endpointSpec.RateLimit.Burst,
			IsActive:                   true,
		}

		if err := s.endpointRepo.Create(ctx, &endpoint); err != nil {
			return fmt.Errorf("failed to create endpoint: %w", err)
		}
	}

	// If we have an update callback, call it to update routes
	if s.updateCallback != nil {
		if err := s.updateCallback(); err != nil {
			// Log error but continue
			fmt.Printf("Failed to update routes after service discovery: %v\n", err)
		}
	}

	return nil
}

// generateEndpointID creates a unique ID for an endpoint
func generateEndpointID(serviceID, path, method string) string {
	// Simple hash-based ID generation
	// In production, you might want to use a proper UUID or hash
	return fmt.Sprintf("%s-%s-%s", serviceID, method, path)
}

// ServiceDiscoveryResponse represents the response from a service's discovery endpoint
type ServiceDiscoveryResponse struct {
	ServiceName string            `json:"serviceName"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Endpoints   []EndpointSpec    `json:"endpoints"`
}

// EndpointSpec represents a single endpoint specification
type EndpointSpec struct {
	Path         string      `json:"path"`
	Method       string      `json:"method"` // GET, POST, PUT, DELETE, PATCH
	Summary      string      `json:"summary,omitempty"`
	InputSchema  interface{} `json:"inputSchema,omitempty"`  // JSON Schema
	OutputSchema interface{} `json:"outputSchema,omitempty"` // JSON Schema
	AuthRequired bool        `json:"authRequired,omitempty"`
	RateLimit    struct {
		RequestsPerMinute int `json:"requestsPerMinute"`
		Burst             int `json:"burst"`
	} `json:"rateLimit,omitempty"`
}
