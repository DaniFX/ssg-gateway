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

type Service struct {
	ID          string            `json:"id" firestore:"id"`
	Name        string            `json:"name" firestore:"name"`
	URL         string            `json:"url" firestore:"url"`
	Description string            `json:"description" firestore:"description"`
	Version     string            `json:"version" firestore:"version"`
	IsActive    bool              `json:"isActive" firestore:"isActive"`
	Metadata    map[string]string `json:"metadata" firestore:"metadata"`
	CreatedAt   time.Time         `json:"createdAt" firestore:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt" firestore:"updatedAt"`
}

type ServiceEndpoint struct {
	ID                         string      `json:"id" firestore:"id"`
	ServiceID                  string      `json:"serviceId" firestore:"serviceId"`
	Path                       string      `json:"path" firestore:"path"`
	Method                     string      `json:"method" firestore:"method"`
	Summary                    string      `json:"summary" firestore:"summary"`
	InputSchema                interface{} `json:"inputSchema" firestore:"inputSchema"`
	OutputSchema               interface{} `json:"outputSchema" firestore:"outputSchema"`
	AuthRequired               bool        `json:"authRequired" firestore:"authRequired"`
	RateLimitRequestsPerMinute int         `json:"rateLimitRequestsPerMinute" firestore:"rateLimitRequestsPerMinute"`
	RateLimitBurst             int         `json:"rateLimitBurst" firestore:"rateLimitBurst"`
	IsActive                   bool        `json:"isActive" firestore:"isActive"`
	CreatedAt                  time.Time   `json:"createdAt" firestore:"createdAt"`
	UpdatedAt                  time.Time   `json:"updatedAt" firestore:"updatedAt"`
}
