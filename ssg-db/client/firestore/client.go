package firestore

import (
	"context"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"

	"github.com/ssg/ssg-db/repository"
)

type Client struct {
	client *firestore.Client
}

func NewClient(ctx context.Context, projectID string, credentialsPath string) (*Client, error) {
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

	return &Client{client: client}, nil
}

func NewClientWithClient(client *firestore.Client) *Client {
	return &Client{client: client}
}

func (c *Client) GetClient() *firestore.Client {
	return c.client
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) User() repository.UserRepository {
	return NewUserRepository(c.client)
}

func (c *Client) Role() repository.RoleRepository {
	return NewRoleRepository(c.client)
}

func (c *Client) App() repository.AppRepository {
	return NewAppRepository(c.client)
}
