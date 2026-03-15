package models

import "time"

type User struct {
	ID        string             `json:"id" firestore:"id"`
	Email     string             `json:"email" firestore:"email"`
	CreatedAt time.Time          `json:"createdAt" firestore:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt" firestore:"updatedAt"`
	Apps      map[string]AppRole `json:"apps" firestore:"apps"`
}

type AppRole struct {
	Role    string    `json:"role" firestore:"role"`
	AddedAt time.Time `json:"addedAt" firestore:"addedAt"`
}

type Role struct {
	ID          string   `json:"id" firestore:"id"`
	Name        string   `json:"name" firestore:"name"`
	Description string   `json:"description" firestore:"description"`
	Permissions []string `json:"permissions" firestore:"permissions"`
}

type App struct {
	ID          string    `json:"id" firestore:"id"`
	Name        string    `json:"name" firestore:"name"`
	Description string    `json:"description" firestore:"description"`
	CreatedAt   time.Time `json:"createdAt" firestore:"createdAt"`
}
