package firestore

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/ssg/ssg-db/models"
	"github.com/ssg/ssg-db/repository"
	"google.golang.org/api/iterator"
)

type UserRepository struct {
	client     *firestore.Client
	collection string
}

func NewUserRepository(client *firestore.Client) repository.UserRepository {
	return &UserRepository{
		client:     client,
		collection: "users",
	}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	_, err := r.client.Collection(r.collection).Doc(user.ID).Set(ctx, user)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	doc, err := r.client.Collection(r.collection).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	iter := r.client.Collection(r.collection).Where("email", "==", email).Limit(1).Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err != nil {
		if err == iterator.Done {
			return nil, nil
		}
		return nil, err
	}
	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()
	_, err := r.client.Collection(r.collection).Doc(user.ID).Set(ctx, user)
	return err
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	_, err := r.client.Collection(r.collection).Doc(id).Delete(ctx)
	return err
}

func (r *UserRepository) GetAll(ctx context.Context) ([]models.User, error) {
	iter := r.client.Collection(r.collection).Documents(ctx)
	defer iter.Stop()

	var users []models.User
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var user models.User
		if err := doc.DataTo(&user); err == nil {
			users = append(users, user)
		}
	}
	return users, nil
}

func (r *UserRepository) GetUserAppRole(ctx context.Context, userID, appID string) (*models.AppRole, error) {
	doc, err := r.client.Collection(r.collection).Doc(userID).Get(ctx)
	if err != nil {
		return nil, err
	}
	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, err
	}
	role, ok := user.Apps[appID]
	if !ok {
		return nil, nil
	}
	return &role, nil
}

func (r *UserRepository) SetUserAppRole(ctx context.Context, userID, appID, role string) error {
	fieldPath := "apps." + appID
	_, err := r.client.Collection(r.collection).Doc(userID).Update(ctx, []firestore.Update{
		{
			Path:  fieldPath,
			Value: models.AppRole{Role: role, AddedAt: time.Now()},
		},
	})
	return err
}

func (r *UserRepository) RemoveUserAppRole(ctx context.Context, userID, appID string) error {
	fieldPath := "apps." + appID
	_, err := r.client.Collection(r.collection).Doc(userID).Update(ctx, []firestore.Update{
		{
			Path:  fieldPath,
			Value: firestore.Delete,
		},
	})
	return err
}
