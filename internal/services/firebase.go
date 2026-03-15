package services

import (
	"context"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type FirebaseService struct {
	app        *firebase.App
	authClient *auth.Client
	firestore  *firestore.Client
	projectID  string
}

func NewFirebaseService(credentialsPath, firestoreProjectID string) (*FirebaseService, error) {
	opt := option.WithCredentialsFile(credentialsPath)

	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, err
	}

	authClient, err := app.Auth(context.Background())
	if err != nil {
		return nil, err
	}

	firestoreClient, err := firestore.NewClient(context.Background(), firestoreProjectID)
	if err != nil {
		return nil, err
	}

	return &FirebaseService{
		app:        app,
		authClient: authClient,
		firestore:  firestoreClient,
		projectID:  firestoreProjectID,
	}, nil
}

func (s *FirebaseService) GetAuthClient() *auth.Client {
	return s.authClient
}

func (s *FirebaseService) GetFirestore() *firestore.Client {
	return s.firestore
}

func (s *FirebaseService) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	return s.authClient.VerifyIDToken(ctx, idToken)
}

type UserAppRole struct {
	Role    string `json:"role"`
	AddedAt int64  `json:"addedAt"`
}

type UserData struct {
	Email     string                 `json:"email"`
	CreatedAt int64                  `json:"createdAt"`
	Apps      map[string]UserAppRole `json:"apps"`
}

func (s *FirebaseService) GetUserAppRole(ctx context.Context, userID, appID string) (*UserAppRole, error) {
	doc, err := s.firestore.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		return nil, err
	}

	var userData UserData
	if err := doc.DataTo(&userData); err != nil {
		return nil, err
	}

	role, ok := userData.Apps[appID]
	if !ok {
		return nil, nil
	}

	return &role, nil
}

func (s *FirebaseService) SetUserAppRole(ctx context.Context, userID, appID, role string) error {
	_, err := s.firestore.Collection("users").Doc(userID).Set(ctx, map[string]interface{}{
		"apps": map[string]interface{}{
			appID: UserAppRole{
				Role:    role,
				AddedAt: 0,
			},
		},
	}, firestore.MergeAll)
	return err
}

func (s *FirebaseService) GetUserEmail(ctx context.Context, userID string) (string, error) {
	user, err := s.authClient.GetUser(ctx, userID)
	if err != nil {
		return "", err
	}
	return user.Email, nil
}

func (s *FirebaseService) CreateUser(ctx context.Context, email, password string) (*auth.UserRecord, error) {
	params := (&auth.UserToCreate{}).
		Email(email).
		Password(password)
	return s.authClient.CreateUser(ctx, params)
}

func (s *FirebaseService) DeleteUser(ctx context.Context, userID string) error {
	return s.authClient.DeleteUser(ctx, userID)
}
