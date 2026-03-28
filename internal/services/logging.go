package services

import (
	"context"

	"cloud.google.com/go/logging/logadmin"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type LoggingService struct {
	projectID   string
	adminClient *logadmin.Client
}

type LogEntry struct {
	InsertID  string      `json:"insertId"`
	LogName   string      `json:"logName"`
	Payload   interface{} `json:"payload"`
	Severity  string      `json:"severity"`
	Timestamp string      `json:"timestamp"`
}

// NewLoggingService initializes the GCP logadmin Client
func NewLoggingService(ctx context.Context, projectID string, credentialsPath string) (*LoggingService, error) {
	var opts []option.ClientOption
	if credentialsPath != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsPath))
	}

	adminClient, err := logadmin.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, err
	}

	return &LoggingService{
		projectID:   projectID,
		adminClient: adminClient,
	}, nil
}

// GetLogs queries Cloud Logging using standard filters
func (s *LoggingService) GetLogs(ctx context.Context, filter string, limit int) ([]LogEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100 // default safe limit
	}

	it := s.adminClient.Entries(ctx, logadmin.Filter(filter))
	var entries []LogEntry

	for len(entries) < limit {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		entries = append(entries, LogEntry{
			InsertID:  entry.InsertID,
			LogName:   entry.LogName,
			Payload:   entry.Payload,
			Severity:  entry.Severity.String(),
			Timestamp: entry.Timestamp.Format("2006-01-02T15:04:05.999Z"),
		})
	}

	// Always return an empty slice rather than nil for JSON consistency
	if entries == nil {
		entries = []LogEntry{}
	}

	return entries, nil
}
