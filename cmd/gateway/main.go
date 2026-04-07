package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	db "github.com/ssg/ssg-db"
	"github.com/ssg/ssg-db/client/firestore"
	_ "github.com/ssg/ssg-db/migrations"
	"github.com/ssg/ssg-gateway/internal/config"
	"github.com/ssg/ssg-gateway/internal/handlers"
	"github.com/ssg/ssg-gateway/internal/middleware"
	"github.com/ssg/ssg-gateway/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Warn("No .env file found, using environment variables")
	}

	cfg := config.Load()

	firebaseService, err := services.NewFirebaseService(
		cfg.FirebaseConfig.CredentialsPath,
		cfg.FirestoreConfig.ProjectID,
	)
	if err != nil {
		log.Fatalf("Failed to initialize Firebase: %v", err)
	}

	migrator, err := db.NewMigrator(cfg.FirestoreConfig.ProjectID, cfg.FirebaseConfig.CredentialsPath)
	if err != nil {
		log.Fatalf("Failed to initialize migrator: %v", err)
	}
	defer migrator.Close()

	if err := migrator.Migrate(context.Background()); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	dbClient := firestore.NewClientWithClient(migrator.GetClient())
	defer dbClient.Close()

	userRepo := dbClient.User()
	roleRepo := dbClient.Role()
	appRepo := dbClient.App()

	appID := "ssg-admin"

	// Qui inizializzi il middleware di Firebase per i JWT
	authMiddleware := middleware.NewFirebaseAuthMiddleware(firebaseService, userRepo, appID, cfg.AdminConfig.Emails)

	communicatorClient, err := services.NewCommunicatorClient(cfg)
	if err != nil {
		log.Printf("Warning: Failed to initialize communicator client: %v", err)
	} else {
		defer communicatorClient.Close()
	}

	loggingService, err := services.NewLoggingService(context.Background(), cfg.FirestoreConfig.ProjectID, cfg.FirebaseConfig.CredentialsPath)
	if err != nil {
		slog.Warn("Failed to initialize GCP Logging Service. Logs endpoint may not work.", "error", err)
	}
	logsHandler := handlers.NewLogsHandler(loggingService)

	// 1. PREDICHIARAZIONE del RouteConfigurator
	// Serve affinché il DiscoveryService possa chiamare il metodo RefreshRoutes()
	var routeConfigurator *services.RouteConfigurator

	// 2. Inizializza il Discovery Service passando la callback di aggiornamento
	discoveryService := services.NewDiscoveryService(dbClient, cfg, func() error {
		if routeConfigurator != nil {
			// Aggiorna le rotte in memoria non appena un servizio si registra
			routeConfigurator.RefreshRoutes()
		}
		return nil
	})

	r := gin.Default()
	r.Use(middleware.LoggerContext())

	// 3. Inizializza e avvia il Route Configurator
	// MODIFICA QUI: Passiamo authMiddleware.Authenticate() al costruttore!
	routeConfigurator = services.NewRouteConfigurator(dbClient, cfg, r, authMiddleware.Authenticate())
	go routeConfigurator.Start(context.Background())

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5173",
			"http://localhost:3000",
			"https://ssg-api-99ac0.web.app",
			"https://ssg-api-99ac0.firebaseapp.com",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	rateLimiter := middleware.NewRateLimiter(10, 20)
	go middleware.CleanupRateLimiters(rateLimiter, 5*time.Minute)

	r.Use(rateLimiter.Middleware())

	healthHandler := handlers.NewHealthHandler()
	webApiKey := os.Getenv("FIREBASE_WEB_API_KEY")
	authHandler := handlers.NewAuthHandler(firebaseService, appID, cfg.AdminConfig.Emails, webApiKey)
	userHandler := handlers.NewUserHandler(userRepo, roleRepo, firebaseService, appID)
	roleHandler := handlers.NewRoleHandler(roleRepo)
	appHandler := handlers.NewAppHandler(appRepo)
	communicatorHandler := handlers.NewCommunicatorHandler(communicatorClient)

	// =========================================================================
	// HANDSHAKE E REGISTRAZIONE MICROSERVIZI (PUSH)
	// =========================================================================
	internalAuth := func(c *gin.Context) {
		secret := c.GetHeader("X-Internal-Secret")
		if secret == "" || secret != os.Getenv("INTERNAL_SECRET") {
			c.AbortWithStatusJSON(401, gin.H{"error": "Unauthorized internal access"})
			return
		}
		c.Next()
	}

	r.POST("/internal/register", internalAuth, func(c *gin.Context) {
		var reg services.ServiceDiscoveryResponse
		if err := c.ShouldBindJSON(&reg); err != nil {
			c.JSON(400, gin.H{"error": "Invalid registration payload", "details": err.Error()})
			return
		}

		serviceURL := c.GetHeader("X-Service-Url")
		if serviceURL == "" && reg.Metadata != nil {
			serviceURL = reg.Metadata["targetUrl"]
		}

		if serviceURL == "" {
			c.JSON(400, gin.H{"error": "Service URL is required. Send it via 'X-Service-Url' header or in metadata as 'targetUrl'"})
			return
		}

		err := discoveryService.RegisterService(c.Request.Context(), reg, serviceURL)
		if err != nil {
			c.JSON(500, gin.H{"success": false, "error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"success": true,
			"message": fmt.Sprintf("Service %s registered successfully", reg.ServiceName),
		})
	})
	// =========================================================================

	r.GET("/health", healthHandler.Health)
	r.GET("/ready", healthHandler.Ready)
	r.GET("/live", healthHandler.Live)

	r.POST("/auth/login", authHandler.Login)

	api := r.Group("/api/v1")
	{
		api.GET("/public", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"success": true,
				"data":    gin.H{"message": "Public endpoint"},
			})
		})

		protected := api.Group("")
		protected.Use(authMiddleware.Authenticate())
		{
			protected.GET("/me", func(c *gin.Context) {
				userID, _ := c.Get("userID")
				email, _ := c.Get("userEmail")
				role, _ := c.Get("userRole")

				c.JSON(200, gin.H{
					"success": true,
					"data": gin.H{
						"userId": userID,
						"email":  email,
						"role":   role,
					},
				})
			})

			admin := protected.Group("/admin")
			admin.Use(authMiddleware.RequireRole("admin"))
			{
				admin.GET("/stats", func(c *gin.Context) {
					c.JSON(200, gin.H{
						"success": true,
						"data": gin.H{
							"totalUsers":  100,
							"totalOrders": 50,
						},
					})
				})

				admin.GET("/users", userHandler.ListUsers)
				admin.GET("/users/:id", userHandler.GetUser)
				admin.POST("/users", userHandler.CreateUser)
				admin.PUT("/users/:id/role", userHandler.UpdateUserRole)
				admin.DELETE("/users/:id", userHandler.DeleteUser)

				admin.GET("/roles", roleHandler.ListRoles)
				admin.GET("/roles/:id", roleHandler.GetRole)
				admin.POST("/roles", roleHandler.CreateRole)
				admin.PUT("/roles/:id", roleHandler.UpdateRole)
				admin.DELETE("/roles/:id", roleHandler.DeleteRole)

				admin.GET("/apps", appHandler.ListApps)
				admin.GET("/apps/:id", appHandler.GetApp)
				admin.POST("/apps", appHandler.CreateApp)
				admin.PUT("/apps/:id", appHandler.UpdateApp)
				admin.DELETE("/apps/:id", appHandler.DeleteApp)

				admin.POST("/send-email", communicatorHandler.SendEmail)

				admin.GET("/logs", logsHandler.GetLogs)
			}
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
