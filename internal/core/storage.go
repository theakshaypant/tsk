package core

import (
	"context"
	"time"
)

// Storage handles the persistence of events.
type Storage interface {
	// Sync saves a batch of events.
	// Smart enough to update existing ones and insert new ones.
	SyncEvents(ctx context.Context, events []Event) error
	// List returns events sorted by Start time.
	ListEvents(ctx context.Context, filter EventFilter) ([]Event, error)
	// Purge removes events associated with a specific provider ID.
	// Useful when re-syncing a calendar from scratch.
	PurgeProvider(ctx context.Context, providerID string) error
}

// EventFilter defines criteria for querying the database.
type EventFilter struct {
	Start time.Time
	End   time.Time
	// If empty, return all providers
	ProviderIDs []string
}
