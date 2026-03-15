package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
)

type RoleRepository struct {
	client     *firestore.Client
	collection string
}

func NewRoleRepository(client *firestore.Client) repository.RoleRepository {
	return &RoleRepository{
		client:     client,
		collection: "roles",
	}
}

func (r *RoleRepository) Create(ctx context.Context, role *models.Role) error {
	_, err := r.client.Collection(r.collection).Doc(role.ID).Set(ctx, role)
	return err
}

func (r *RoleRepository) GetByID(ctx context.Context, id string) (*models.Role, error) {
	doc, err := r.client.Collection(r.collection).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	var role models.Role
	if err := doc.DataTo(&role); err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) GetAll(ctx context.Context) ([]models.Role, error) {
	iter := r.client.Collection(r.collection).Documents(ctx)
	defer iter.Stop()

	var roles []models.Role
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var role models.Role
		if err := doc.DataTo(&role); err == nil {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

func (r *RoleRepository) Update(ctx context.Context, role *models.Role) error {
	_, err := r.client.Collection(r.collection).Doc(role.ID).Set(ctx, role)
	return err
}

func (r *RoleRepository) Delete(ctx context.Context, id string) error {
	_, err := r.client.Collection(r.collection).Doc(id).Delete(ctx)
	return err
}
