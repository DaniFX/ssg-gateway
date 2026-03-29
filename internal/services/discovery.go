package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	ssgfirestore "github.com/ssg/ssg-db/client/firestore"
	models "github.com/ssg/ssg-db/models"
	repository "github.com/ssg/ssg-db/repository"
	"github.com/ssg/ssg-gateway/internal/config"
)

// DiscoveryService handles discovering services and their endpoints
type DiscoveryService struct {
	dbClient       *ssgfirestore.Client
	cfg            *config.Config
	httpClient     *http.Client
	serviceRepo    repository.ServiceRepository
	endpointRepo   repository.ServiceEndpointRepository
	updateCallback func() error
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(dbClient *ssgfirestore.Client, cfg *config.Config, updateCallback func() error) *DiscoveryService {
	return &DiscoveryService{
		dbClient:       dbClient,
		cfg:            cfg,
		httpClient:     &http.Client{Timeout: 10 * time.Second},
		serviceRepo:    dbClient.Service(),
		endpointRepo:   dbClient.ServiceEndpoint(),
		updateCallback: updateCallback,
	}
}

// RegisterService viene chiamato quando un microservizio fa l'handshake (Push).
func (s *DiscoveryService) RegisterService(ctx context.Context, registration ServiceDiscoveryResponse, serviceURL string) error {
	// 1. Dobbiamo recuperare il servizio. Dato che non hai GetByName, cerchiamo tutti i servizi attivi
	// e li filtriamo per nome. In Firestore, l'ID del documento spesso COINCIDE con il nome del servizio.
	// Per sicurezza, iteriamo o proviamo a usare GetByID con il ServiceName (se l'ID è il nome).

	var targetService *models.Service

	// Tentativo 1: Recupera tutti e cerca per nome
	services, err := s.serviceRepo.GetAll(ctx)
	if err == nil {
		for _, srv := range services {
			if srv.Name == registration.ServiceName {
				targetService = &srv
				break
			}
		}
	}

	// 2. Se non esiste, lo creiamo
	if targetService == nil {
		// Crea un nuovo servizio usando un ID univoco. Qui usiamo il nome come ID per semplicità,
		// oppure lasciamo che il repository generi l'ID (se ID è vuoto).
		newSrv := models.Service{
			ID:          registration.ServiceName, // Usa il nome come ID document su Firestore se appropriato
			Name:        registration.ServiceName,
			URL:         serviceURL,
			Description: registration.Description,
			Version:     registration.Version,
			Metadata:    registration.Metadata,
			IsActive:    true,
		}

		if err := s.serviceRepo.Create(ctx, &newSrv); err != nil {
			return fmt.Errorf("failed to create new service %s: %w", registration.ServiceName, err)
		}
		targetService = &newSrv
	} else {
		// 3. Se esiste, aggiorniamo i dati
		targetService.URL = serviceURL
		targetService.Version = registration.Version
		targetService.Description = registration.Description
		targetService.Metadata = registration.Metadata
		targetService.IsActive = true

		if err := s.serviceRepo.Update(ctx, targetService); err != nil {
			return fmt.Errorf("failed to update service %s: %w", targetService.Name, err)
		}
	}

	// 4. Deactivate all existing endpoints for this service prima di inserirli nuovi
	existingEndpoints, err := s.endpointRepo.GetByServiceID(ctx, targetService.ID)
	if err == nil {
		for _, endpoint := range existingEndpoints {
			if err := s.endpointRepo.Deactivate(ctx, endpoint.ID); err != nil {
				fmt.Printf("Failed to deactivate endpoint %s: %v\n", endpoint.ID, err)
			}
		}
	}

	// 5. Create new endpoints from the registration payload
	for _, endpointSpec := range registration.Endpoints {
		endpoint := models.ServiceEndpoint{
			ID:                         generateEndpointID(targetService.ID, endpointSpec.Path, endpointSpec.Method),
			ServiceID:                  targetService.ID,
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
			fmt.Printf("Failed to create endpoint %s for service %s: %v\n", endpoint.Path, targetService.Name, err)
			// Non blocchiamo tutto se fallisce un endpoint, proseguiamo col prossimo
		}
	}

	// 6. Trigger route update callback per il Gateway
	if s.updateCallback != nil {
		if err := s.updateCallback(); err != nil {
			fmt.Printf("Failed to update routes after service discovery: %v\n", err)
		}
	}

	return nil
}

// generateEndpointID creates a unique ID for an endpoint
func generateEndpointID(serviceID, path, method string) string {
	return fmt.Sprintf("%s-%s-%s", serviceID, method, path)
}

// ServiceDiscoveryResponse represents the response from a service's discovery endpoint (Push Payload)
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
