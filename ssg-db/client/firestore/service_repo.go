package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
)

type ServiceRepository struct {
	client     *firestore.Client
	collection string
}

func NewServiceRepository(client *firestore.Client) repository.ServiceRepository {
	return &ServiceRepository{
		client:     client,
		collection: "services",
	}
}

func (r *ServiceRepository) Create(ctx context.Context, service *models.Service) error {
	_, err := r.client.Collection(r.collection).Doc(service.ID).Set(ctx, service)
	return err
}

func (r *ServiceRepository) GetByID(ctx context.Context, id string) (*models.Service, error) {
	doc, err := r.client.Collection(r.collection).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	var service models.Service
	if err := doc.DataTo(&service); err != nil {
		return nil, err
	}
	return &service, nil
}

func (r *ServiceRepository) GetAll(ctx context.Context) ([]models.Service, error) {
	iter := r.client.Collection(r.collection).Documents(ctx)
	defer iter.Stop()

	var services []models.Service
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var service models.Service
		if err := doc.DataTo(&service); err == nil {
			services = append(services, service)
		}
	}
	return services, nil
}

func (r *ServiceRepository) GetActive(ctx context.Context) ([]models.Service, error) {
	iter := r.client.Collection(r.collection).Where("isActive", "==", true).Documents(ctx)
	defer iter.Stop()

	var services []models.Service
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var service models.Service
		if err := doc.DataTo(&service); err == nil {
			services = append(services, service)
		}
	}
	return services, nil
}

func (r *ServiceRepository) Update(ctx context.Context, service *models.Service) error {
	_, err := r.client.Collection(r.collection).Doc(service.ID).Set(ctx, service)
	return err
}

func (r *ServiceRepository) Delete(ctx context.Context, id string) error {
	_, err := r.client.Collection(r.collection).Doc(id).Delete(ctx)
	return err
}
