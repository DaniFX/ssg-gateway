package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
)

type ServiceEndpointRepository struct {
	client     *firestore.Client
	collection string
}

func NewServiceEndpointRepository(client *firestore.Client) repository.ServiceEndpointRepository {
	return &ServiceEndpointRepository{
		client:     client,
		collection: "service_endpoints",
	}
}

func (r *ServiceEndpointRepository) Create(ctx context.Context, endpoint *models.ServiceEndpoint) error {
	_, err := r.client.Collection(r.collection).Doc(endpoint.ID).Set(ctx, endpoint)
	return err
}

func (r *ServiceEndpointRepository) GetByID(ctx context.Context, id string) (*models.ServiceEndpoint, error) {
	doc, err := r.client.Collection(r.collection).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	var endpoint models.ServiceEndpoint
	if err := doc.DataTo(&endpoint); err != nil {
		return nil, err
	}
	return &endpoint, nil
}

func (r *ServiceEndpointRepository) GetByServiceID(ctx context.Context, serviceID string) ([]models.ServiceEndpoint, error) {
	iter := r.client.Collection(r.collection).Where("serviceId", "==", serviceID).Documents(ctx)
	defer iter.Stop()

	var endpoints []models.ServiceEndpoint
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var endpoint models.ServiceEndpoint
		if err := doc.DataTo(&endpoint); err == nil {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints, nil
}

func (r *ServiceEndpointRepository) GetActiveByServiceID(ctx context.Context, serviceID string) ([]models.ServiceEndpoint, error) {
	iter := r.client.Collection(r.collection).
		Where("serviceId", "==", serviceID).
		Where("isActive", "==", true).
		Documents(ctx)
	defer iter.Stop()

	var endpoints []models.ServiceEndpoint
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var endpoint models.ServiceEndpoint
		if err := doc.DataTo(&endpoint); err == nil {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints, nil
}

func (r *ServiceEndpointRepository) Deactivate(ctx context.Context, id string) error {
	_, err := r.client.Collection(r.collection).Doc(id).Update(ctx, []firestore.Update{
		{Path: "isActive", Value: false},
	})
	return err
}

func (r *ServiceEndpointRepository) Delete(ctx context.Context, id string) error {
	_, err := r.client.Collection(r.collection).Doc(id).Delete(ctx)
	return err
}
