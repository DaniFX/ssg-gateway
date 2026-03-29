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
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
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

	// Caching per i generatori di Token OIDC (evita di bloccare ogni singola richiesta HTTP)
	tokenSources map[string]oauth2.TokenSource
	tsMu         sync.RWMutex
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
		tokenSources: make(map[string]oauth2.TokenSource),
	}
}

// getTokenSource recupera o crea un generatore di token OIDC per un URL specifico
func (r *RouteConfigurator) getTokenSource(ctx context.Context, audience string) (oauth2.TokenSource, error) {
	// Lettura rapida dalla cache
	r.tsMu.RLock()
	ts, exists := r.tokenSources[audience]
	r.tsMu.RUnlock()
	if exists {
		return ts, nil
	}

	// Lock in scrittura se non esiste
	r.tsMu.Lock()
	defer r.tsMu.Unlock()

	// Doppio controllo (pattern standard) per sicurezza in concorrenza
	if ts, exists := r.tokenSources[audience]; exists {
		return ts, nil
	}

	// Crea il nuovo token source per il microservizio target
	ts, err := idtoken.NewTokenSource(ctx, audience)
	if err != nil {
		return nil, err
	}

	r.tokenSources[audience] = ts
	return ts, nil
}

// Start begins initial route configuration
func (r *RouteConfigurator) Start(ctx context.Context) error {
	if err := r.setupRoutes(ctx); err != nil {
		return fmt.Errorf("initial route setup failed: %w", err)
	}
	return nil
}

// Stop halts the route configuration process
func (r *RouteConfigurator) Stop() {
	close(r.stopChan)
}

// RefreshRoutes viene chiamato esternamente (es. dal discovery service)
func (r *RouteConfigurator) RefreshRoutes() error {
	fmt.Println("🔄 Refreshing routes from database due to new service registration...")
	return r.setupRoutes(context.Background())
}

// setupRoutes creates routes for all currently active services and endpoints
func (r *RouteConfigurator) setupRoutes(ctx context.Context) error {
	services, err := r.serviceRepo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active services: %w", err)
	}

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
	if service.URL == "" {
		return fmt.Errorf("service %s has no URL configured", service.Name)
	}

	routeKey := fmt.Sprintf("%s:%s:%s", service.ID, endpoint.ID, endpoint.Method)

	r.mu.Lock()
	if _, exists := r.activeRoutes[routeKey]; exists {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	servicePath := strings.TrimRight(service.Name, "/")
	if servicePath == "" {
		servicePath = "/"
	} else if !strings.HasPrefix(servicePath, "/") {
		servicePath = "/" + servicePath
	}

	endpointPath := endpoint.Path
	if !strings.HasPrefix(endpointPath, "/") {
		endpointPath = "/" + endpointPath
	}

	fullPath := servicePath + endpointPath
	handler := r.createProxyHandler(service, endpoint)

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
func (r *RouteConfigurator) unregisterRoute(serviceID, endpointID, method string) {
	routeKey := fmt.Sprintf("%s:%s:%s", serviceID, endpointID, method)

	r.mu.Lock()
	delete(r.activeRoutes, routeKey)
	r.mu.Unlock()
}

// createProxyHandler creates a handler function that proxies requests to the service endpoint
func (r *RouteConfigurator) createProxyHandler(service models.Service, endpoint models.ServiceEndpoint) gin.HandlerFunc {
	// Calcoliamo l'audience una volta sola: è l'URL di base del Cloud Run target (es: https://servizio.run.app)
	audience := strings.TrimRight(service.URL, "/")

	return func(c *gin.Context) {
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

		servicePath := strings.TrimRight(service.Name, "/")
		if servicePath == "" {
			servicePath = "/"
		} else if !strings.HasPrefix(servicePath, "/") {
			servicePath = "/" + servicePath
		}

		requestPath := c.Request.URL.Path
		if strings.HasPrefix(requestPath, servicePath) {
			requestPath = strings.TrimPrefix(requestPath, servicePath)
		}
		if strings.HasPrefix(requestPath, "//") {
			requestPath = strings.TrimPrefix(requestPath, "/")
		}

		targetURL := service.URL + requestPath
		if c.Request.URL.RawQuery != "" {
			targetURL += "?" + c.Request.URL.RawQuery
		}

		proxyReq, err := http.NewRequestWithContext(c.Request.Context(), endpoint.Method, targetURL, c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to create proxy request"})
			return
		}

		// Copia gli header originali
		copyHeaders(proxyReq.Header, c.Request.Header)

		// Rimuove gli header "hop-by-hop" che non devono essere proxyati
		hopByHopHeaders := []string{
			"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
			"Te", "Trailer", "Transfer-Encoding", "Upgrade",
		}
		for _, h := range hopByHopHeaders {
			proxyReq.Header.Del(h)
		}

		// =========================================================================
		// INIEZIONE IAM (OIDC TOKEN) E IDENTITÀ UTENTE
		// =========================================================================

		// 1. Inseriamo il Token OIDC per superare la barriera IAM del microservizio Cloud Run
		ts, err := r.getTokenSource(c.Request.Context(), audience)
		if err == nil {
			token, err := ts.Token()
			if err == nil {
				proxyReq.Header.Set("Authorization", "Bearer "+token.AccessToken)
			} else {
				fmt.Printf("⚠️ Errore nel generare OIDC token per %s: %v\n", audience, err)
			}
		} else {
			// In locale potrebbe dare errore se non hai GOOGLE_APPLICATION_CREDENTIALS settate,
			// ma funzionerà in modo nativo e automatico su Cloud Run.
			fmt.Printf("⚠️ Impossibile inizializzare TokenSource per %s: %v\n", audience, err)
		}

		// 2. Inoltriamo l'identità dell'utente al Microservizio tramite Header Custom
		// (Altrimenti il microservizio non saprebbe chi ha fatto la richiesta originaria)
		if userID, exists := c.Get("userID"); exists {
			proxyReq.Header.Set("X-User-Id", fmt.Sprint(userID))
		}
		if userEmail, exists := c.Get("userEmail"); exists {
			proxyReq.Header.Set("X-User-Email", fmt.Sprint(userEmail))
		}
		if userRole, exists := c.Get("userRole"); exists {
			proxyReq.Header.Set("X-User-Role", fmt.Sprint(userRole))
		}
		// =========================================================================

		// Esegue la richiesta
		resp, err := r.httpClient.Do(proxyReq)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "Failed to connect to backend service"})
			return
		}
		defer resp.Body.Close()

		copyHeaders(c.Writer.Header(), resp.Header)
		for _, h := range hopByHopHeaders {
			c.Writer.Header().Del(h)
		}

		c.Writer.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(c.Writer, resp.Body)
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
