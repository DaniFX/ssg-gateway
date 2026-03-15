package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
)

type AppRepository struct {
	client     *firestore.Client
	collection string
}

func NewAppRepository(client *firestore.Client) repository.AppRepository {
	return &AppRepository{
		client:     client,
		collection: "apps",
	}
}

func (r *AppRepository) Create(ctx context.Context, app *models.App) error {
	_, err := r.client.Collection(r.collection).Doc(app.ID).Set(ctx, app)
	return err
}

func (r *AppRepository) GetByID(ctx context.Context, id string) (*models.App, error) {
	doc, err := r.client.Collection(r.collection).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	var app models.App
	if err := doc.DataTo(&app); err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *AppRepository) GetAll(ctx context.Context) ([]models.App, error) {
	iter := r.client.Collection(r.collection).Documents(ctx)
	defer iter.Stop()

	var apps []models.App
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var app models.App
		if err := doc.DataTo(&app); err == nil {
			apps = append(apps, app)
		}
	}
	return apps, nil
}

func (r *AppRepository) Update(ctx context.Context, app *models.App) error {
	_, err := r.client.Collection(r.collection).Doc(app.ID).Set(ctx, app)
	return err
}

func (r *AppRepository) Delete(ctx context.Context, id string) error {
	_, err := r.client.Collection(r.collection).Doc(id).Delete(ctx)
	return err
}
