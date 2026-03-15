package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/ssg/ssg-db"
	"github.com/ssg/ssg-db/client/firestore"
	_ "github.com/ssg/ssg-db/migrations"
	"github.com/ssg/ssg-gateway/internal/config"
	"github.com/ssg/ssg-gateway/internal/handlers"
	"github.com/ssg/ssg-gateway/internal/middleware"
	"github.com/ssg/ssg-gateway/internal/services"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
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
	authMiddleware := middleware.NewFirebaseAuthMiddleware(firebaseService, userRepo, appID, cfg.AdminConfig.Emails)

	r := gin.Default()

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
	authHandler := handlers.NewAuthHandler(firebaseService, appID, cfg.AdminConfig.Emails)
	userHandler := handlers.NewUserHandler(userRepo, roleRepo, firebaseService, appID)
	roleHandler := handlers.NewRoleHandler(roleRepo)
	appHandler := handlers.NewAppHandler(appRepo)

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
			}
		}
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
