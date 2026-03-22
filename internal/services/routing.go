package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	ssgfirestore "github.com/ssg/ssg-db/client/firestore"
	models "github.com/ssg/ssg-db/models"
	repository "github.com/ssg/ssg-db/repository"
	"github.com/ssg/ssg-gateway/internal/config"

	"github.com/gin-gonic/gin"
)

// RouteConfigurator handles dynamic route configuration based on discovered services
type RouteConfigurator struct {
	dbClient     *ssgfirestore.Client
	cfg          *config.Config
	router       *gin.Engine
	serviceRepo  repository.ServiceRepository
	endpointRepo repository.ServiceEndpointRepository
	httpClient   *http.Client
	activeRoutes map[string]*routeInfo
	mu           sync.RWMutex
	stopChan     chan struct{}
}

// routeInfo tracks information about an active route
type routeInfo struct {
	serviceID    string
	endpointID   string
	path         string
	method       string
	registeredAt time.Time
}

// NewRouteConfigurator creates a new route configurator
func NewRouteConfigurator(dbClient *ssgfirestore.Client, cfg *config.Config, router *gin.Engine) *RouteConfigurator {
	return &RouteConfigurator{
		dbClient:     dbClient,
		cfg:          cfg,
		router:       router,
		serviceRepo:  dbClient.Service(),
		endpointRepo: dbClient.ServiceEndpoint(),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		activeRoutes: make(map[string]*routeInfo),
		stopChan:     make(chan struct{}),
	}
}

// Start begins monitoring for service/endpoint changes and updating routes
func (r *RouteConfigurator) Start(ctx context.Context) error {
	// Perform initial route setup
	if err := r.setupRoutes(ctx); err != nil {
		return fmt.Errorf("initial route setup failed: %w", err)
	}

	// Start watching for changes in services and endpoints
	go r.watchServices(ctx)
	go r.watchEndpoints(ctx)

	return nil
}

// Stop halts the route configuration process
func (r *RouteConfigurator) Stop() {
	close(r.stopChan)
}

// setupRoutes creates routes for all currently active services and endpoints
func (r *RouteConfigurator) setupRoutes(ctx context.Context) error {
	// Get all active services
	services, err := r.serviceRepo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active services: %w", err)
	}

	// For each service, get its active endpoints and create routes
	for _, service := range services {
		endpoints, err := r.endpointRepo.GetActiveByServiceID(ctx, service.ID)
		if err != nil {
			return fmt.Errorf("failed to get endpoints for service %s: %w", service.Name, err)
		}

		for _, endpoint := range endpoints {
			if err := r.registerRoute(ctx, service, endpoint); err != nil {
				return fmt.Errorf("failed to register route for service %s endpoint %s: %w",
					service.Name, endpoint.Path, err)
			}
		}
	}

	return nil
}

// registerRoute creates a single route in the Gin router
func (r *RouteConfigurator) registerRoute(ctx context.Context, service models.Service, endpoint models.ServiceEndpoint) error {
	// Skip if service URL is not set
	if service.URL == "" {
		return fmt.Errorf("service %s has no URL configured", service.Name)
	}

	// Generate a unique key for this route
	routeKey := fmt.Sprintf("%s:%s:%s", service.ID, endpoint.ID, endpoint.Method)

	// Check if route already exists
	r.mu.Lock()
	if _, exists := r.activeRoutes[routeKey]; exists {
		r.mu.Unlock()
		return nil // Route already exists
	}
	r.mu.Unlock()

	// Construct the full path for the gateway route
	// Format: /{service-name}{endpoint-path}
	// Ensure service name starts with / and doesn't have trailing slash
	servicePath := strings.TrimRight(service.Name, "/")
	if servicePath == "" {
		servicePath = "/"
	} else if !strings.HasPrefix(servicePath, "/") {
		servicePath = "/" + servicePath
	}

	// Ensure endpoint path starts with /
	endpointPath := endpoint.Path
	if !strings.HasPrefix(endpointPath, "/") {
		endpointPath = "/" + endpointPath
	}

	fullPath := servicePath + endpointPath

	// Create the proxy handler for this route
	handler := r.createProxyHandler(service, endpoint)

	// Register the route with Gin
	switch endpoint.Method {
	case http.MethodGet:
		r.router.GET(fullPath, handler)
	case http.MethodPost:
		r.router.POST(fullPath, handler)
	case http.MethodPut:
		r.router.PUT(fullPath, handler)
	case http.MethodDelete:
		r.router.DELETE(fullPath, handler)
	case http.MethodPatch:
		r.router.PATCH(fullPath, handler)
	case http.MethodHead:
		r.router.HEAD(fullPath, handler)
	case http.MethodOptions:
		r.router.OPTIONS(fullPath, handler)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", endpoint.Method)
	}

	// Track the active route
	r.mu.Lock()
	r.activeRoutes[routeKey] = &routeInfo{
		serviceID:    service.ID,
		endpointID:   endpoint.ID,
		path:         fullPath,
		method:       endpoint.Method,
		registeredAt: time.Now(),
	}
	r.mu.Unlock()

	return nil
}

// unregisterRoute removes a route from the Gin router
// Note: Gin doesn't support unregistering routes directly, so we mark them as inactive
// and handle 404s in the proxy handler instead of actually removing from router
func (r *RouteConfigurator) unregisterRoute(serviceID, endpointID, method string) {
	routeKey := fmt.Sprintf("%s:%s:%s", serviceID, endpointID, method)

	r.mu.Lock()
	delete(r.activeRoutes, routeKey)
	r.mu.Unlock()
}

// createProxyHandler creates a handler function that proxies requests to the service endpoint
func (r *RouteConfigurator) createProxyHandler(service models.Service, endpoint models.ServiceEndpoint) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this route is still active (service/endpoint not deleted/deactivated)
		routeKey := fmt.Sprintf("%s:%s:%s", service.ID, endpoint.ID, endpoint.Method)
		r.mu.RLock()
		if _, active := r.activeRoutes[routeKey]; !active {
			r.mu.RUnlock()
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "Service endpoint not found or no longer active",
			})
			return
		}
		r.mu.RUnlock()

		// Construct the target URL: service.URL + endpoint.Path + original request path/query
		// We need to strip the service prefix from the request path to avoid duplication
		// Example: request to /hello-world-go/api/hello should go to service.URL + /api/hello
		servicePath := strings.TrimRight(service.Name, "/")
		if servicePath == "" {
			servicePath = "/"
		} else if !strings.HasPrefix(servicePath, "/") {
			servicePath = "/" + servicePath
		}

		requestPath := c.Request.URL.Path
		// Remove the service prefix from the request path
		if strings.HasPrefix(requestPath, servicePath) {
			requestPath = strings.TrimPrefix(requestPath, servicePath)
		}
		// Ensure we don't have double slashes
		if strings.HasPrefix(requestPath, "//") {
			requestPath = strings.TrimPrefix(requestPath, "/")
		}

		targetURL := service.URL + requestPath
		if c.Request.URL.RawQuery != "" {
			targetURL += "?" + c.Request.URL.RawQuery
		}

		// Create the proxy request
		proxyReq, err := http.NewRequestWithContext(c.Request.Context(),
			endpoint.Method, targetURL, c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to create proxy request: " + err.Error(),
			})
			return
		}

		// Copy headers from the original request (excluding hop-by-hop headers)
		copyHeaders(proxyReq.Header, c.Request.Header)

		// Remove hop-by-hop headers that shouldn't be forwarded
		hopByHopHeaders := []string{
			"Connection",
			"Keep-Alive",
			"Proxy-Authenticate",
			"Proxy-Authorization",
			"Te",      // canonicalized version of "TE"
			"Trailer", // canonicalized version of "Trailer"
			"Transfer-Encoding",
			"Upgrade",
		}
		for _, h := range hopByHopHeaders {
			proxyReq.Header.Del(h)
		}

		// Execute the proxy request
		resp, err := r.httpClient.Do(proxyReq)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"message": "Failed to connect to service: " + err.Error(),
			})
			return
		}
		defer resp.Body.Close()

		// Copy response headers (excluding hop-by-hop headers)
		copyHeaders(c.Writer.Header(), resp.Header)
		// Remove hop-by-hop headers from response
		for _, h := range hopByHopHeaders {
			c.Writer.Header().Del(h)
		}

		// Set the status code
		c.Writer.WriteHeader(resp.StatusCode)

		// Copy the response body
		_, err = io.Copy(c.Writer, resp.Body)
		if err != nil {
			// Log error but don't return as we've already started writing the response
			fmt.Printf("Error copying response body: %v\n", err)
		}
	}
}

// copyHeaders copies headers from source to destination map
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// watchServices monitors the services collection for changes
func (r *RouteConfigurator) watchServices(ctx context.Context) {
	// In a real implementation, we would use Firestore snapshots to watch for changes
	// For simplicity, we'll use polling with a reasonable interval
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		case <-ticker.C:
			if err := r.setupRoutes(ctx); err != nil {
				fmt.Printf("Error updating routes from services watch: %v\n", err)
			}
		}
	}
}

// watchEndpoints monitors the endpoints collection for changes
func (r *RouteConfigurator) watchEndpoints(ctx context.Context) {
	// In a real implementation, we would use Firestore snapshots to watch for changes
	// For simplicity, we'll use polling with a reasonable interval
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		case <-ticker.C:
			if err := r.setupRoutes(ctx); err != nil {
				fmt.Printf("Error updating routes from endpoints watch: %v\n", err)
			}
		}
	}
}
