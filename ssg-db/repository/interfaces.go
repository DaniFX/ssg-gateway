package repository

import (
	"context"

	"github.com/ssg/ssg-db/models"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetAll(ctx context.Context) ([]models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
	GetUserAppRole(ctx context.Context, userID, appID string) (*models.AppRole, error)
	SetUserAppRole(ctx context.Context, userID, appID, role string) error
	RemoveUserAppRole(ctx context.Context, userID, appID string) error
}

type RoleRepository interface {
	Create(ctx context.Context, role *models.Role) error
	GetByID(ctx context.Context, id string) (*models.Role, error)
	GetAll(ctx context.Context) ([]models.Role, error)
	Update(ctx context.Context, role *models.Role) error
	Delete(ctx context.Context, id string) error
}

type AppRepository interface {
	Create(ctx context.Context, app *models.App) error
	GetByID(ctx context.Context, id string) (*models.App, error)
	GetAll(ctx context.Context) ([]models.App, error)
	Update(ctx context.Context, app *models.App) error
	Delete(ctx context.Context, id string) error
}
