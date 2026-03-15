package config

import (
	"os"
	"strings"
)

type Config struct {
	Port            string
	Environment     string
	FirebaseConfig  FirebaseConfig
	FirestoreConfig FirestoreConfig
	AdminConfig     AdminConfig
}

type FirebaseConfig struct {
	CredentialsPath string
	ProjectID       string
	PrivateKey      string
	ClientEmail     string
}

type FirestoreConfig struct {
	ProjectID string
}

type AdminConfig struct {
	Emails []string
}

func Load() *Config {
	adminEmails := getEnv("ADMIN_EMAILS", "info@danieleoppezzo.com")

	return &Config{
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		FirebaseConfig: FirebaseConfig{
			CredentialsPath: getEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
			ProjectID:       getEnv("FIREBASE_PROJECT_ID", ""),
			PrivateKey:      getEnv("FIREBASE_PRIVATE_KEY", ""),
			ClientEmail:     getEnv("FIREBASE_CLIENT_EMAIL", ""),
		},
		FirestoreConfig: FirestoreConfig{
			ProjectID: getEnv("FIRESTORE_PROJECT_ID", ""),
		},
		AdminConfig: AdminConfig{
			Emails: parseEmails(adminEmails),
		},
	}
}

func parseEmails(emails string) []string {
	var result []string
	for _, e := range strings.Split(emails, ",") {
		trimmed := strings.TrimSpace(e)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
