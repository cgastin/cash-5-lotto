package store

import (
	"context"
	"time"
)

// DrawRepository defines persistence operations for lottery draws.
type DrawRepository interface {
	// UpsertDraw stores a draw; no-ops if already exists with matching checksum.
	UpsertDraw(ctx context.Context, draw Draw) error

	// BatchUpsertDraws stores multiple draws atomically where possible.
	BatchUpsertDraws(ctx context.Context, draws []Draw) error

	// GetDraw retrieves a draw by date.
	GetDraw(ctx context.Context, date time.Time) (*Draw, error)

	// ListDraws returns draws in [from, to] inclusive, sorted ascending.
	ListDraws(ctx context.Context, from, to time.Time) ([]Draw, error)

	// GetLatestDraw returns the most recent draw.
	GetLatestDraw(ctx context.Context) (*Draw, error)

	// GetAllDrawDates returns all stored draw dates sorted ascending.
	GetAllDrawDates(ctx context.Context) ([]time.Time, error)

	// DrawExists checks if a draw for the given date is stored.
	DrawExists(ctx context.Context, date time.Time) (bool, error)

	// GetDrawCount returns total draws stored.
	GetDrawCount(ctx context.Context) (int, error)
}

// SyncStateRepository manages the ingestion sync state record.
type SyncStateRepository interface {
	GetSyncState(ctx context.Context) (*SyncState, error)
	UpdateSyncState(ctx context.Context, state SyncState) error
}

// UserRepository manages user records.
type UserRepository interface {
	GetUser(ctx context.Context, userID string) (*User, error)
	UpsertUser(ctx context.Context, user User) error
}

// SubscriptionRepository manages subscription records.
type SubscriptionRepository interface {
	GetSubscription(ctx context.Context, userID string) (*Subscription, error)
	UpsertSubscription(ctx context.Context, sub Subscription) error
}

// PredictionRepository manages generated predictions.
type PredictionRepository interface {
	StorePrediction(ctx context.Context, pred Prediction) error
	GetPrediction(ctx context.Context, date time.Time) (*Prediction, error)
	ListPredictions(ctx context.Context, from, to time.Time) ([]Prediction, error)
}

// AuditRepository manages audit log entries.
type AuditRepository interface {
	LogEvent(ctx context.Context, event AuditEvent) error
	ListEventsByActor(ctx context.Context, actorID string, limit int) ([]AuditEvent, error)
}
