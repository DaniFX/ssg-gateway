package migrations

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
	"github.com/ssg/ssg-db/models"
)

var migrationFuncs []func(ctx context.Context, client *firestore.Client) error

func init() {
	migrationFuncs = append(migrationFuncs, init001)
}

func GetMigrations() []func(ctx context.Context, client *firestore.Client) error {
	return migrationFuncs
}

func init001(ctx context.Context, client *firestore.Client) error {
	log.Println("Running migration 001: Create initial collections")

	roles := []models.Role{
		{
			ID:          "admin",
			Name:        "Admin",
			Description: "Full access to all resources",
			Permissions: []string{"*"},
		},
		{
			ID:          "editor",
			Name:        "Editor",
			Description: "Can edit content",
			Permissions: []string{"read", "write"},
		},
		{
			ID:          "viewer",
			Name:        "Viewer",
			Description: "Read-only access",
			Permissions: []string{"read"},
		},
	}

	for _, role := range roles {
		_, err := client.Collection("roles").Doc(role.ID).Set(ctx, role)
		if err != nil {
			return err
		}
	}

	apps := []models.App{
		{
			ID:          "ssg-admin",
			Name:        "SSG Admin",
			Description: "Admin dashboard for SSG",
		},
	}

	for _, app := range apps {
		_, err := client.Collection("apps").Doc(app.ID).Set(ctx, app)
		if err != nil {
			return err
		}
	}

	log.Println("Migration 001 completed: Created roles and apps collections")
	return nil
}
