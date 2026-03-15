package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"

	"github.com/ssg/ssg-db/migrations"
)

type Migration struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	AppliedAt time.Time `json:"appliedAt"`
}

type MigrationFunc func(ctx context.Context, client *firestore.Client) error

type Migrator struct {
	client    *firestore.Client
	projectID string
}

func NewMigrator(projectID string, credentialsPath string) (*Migrator, error) {
	ctx := context.Background()

	var client *firestore.Client
	var err error

	if credentialsPath != "" {
		client, err = firestore.NewClient(ctx, projectID, option.WithCredentialsFile(credentialsPath))
	} else {
		client, err = firestore.NewClient(ctx, projectID)
	}

	if err != nil {
		return nil, err
	}

	return &Migrator{
		client:    client,
		projectID: projectID,
	}, nil
}

func (m *Migrator) Migrate(ctx context.Context) error {
	collection := m.client.Collection("_migrations")

	docs, err := collection.Documents(ctx).GetAll()
	if err != nil {
		return err
	}

	currentVersion := 0
	for _, doc := range docs {
		var migration Migration
		if err := doc.DataTo(&migration); err == nil {
			if migration.Version > currentVersion {
				currentVersion = migration.Version
			}
		}
	}

	log.Printf("Current DB version: %d", currentVersion)
	log.Printf("Total migrations available: %d", len(migrations.GetMigrations()))

	migrationList := migrations.GetMigrations()
	for i := currentVersion; i < len(migrationList); i++ {
		log.Printf("Applying migration %d...", i+1)

		if err := migrationList[i](ctx, m.client); err != nil {
			return err
		}

		docID := fmt.Sprintf("migration_%d", i+1)
		_, err := collection.Doc(docID).Set(ctx, Migration{
			Version:   i + 1,
			Name:      "001_initial",
			AppliedAt: time.Now(),
		})
		if err != nil {
			return err
		}

		log.Printf("Migration %d applied successfully", i+1)
	}

	log.Println("All migrations completed")
	return nil
}

func (m *Migrator) GetClient() *firestore.Client {
	return m.client
}

func (m *Migrator) Close() error {
	return m.client.Close()
}
