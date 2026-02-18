package core

import (
	"context"
	"time"
)

// FetchOptions configures which events to retrieve.
type FetchOptions struct {
	Start time.Time
	End   time.Time

	// Filter by calendar ID. Empty means all calendars.
	// Use specific IDs to fetch from only certain calendars.
	CalendarIDs []string

	// Filter by event type. Empty means only default events.
	// To include OOO, focus time, etc., add them explicitly.
	IncludeTypes []EventType

	// Filter by response status. Empty means all statuses.
	// To show only accepted events, set to []EventStatus{StatusAccepted}.
	IncludeStatuses []EventStatus

	// ExcludeAllDay filters out all-day events when true.
	ExcludeAllDay bool
}

// DefaultFetchOptions returns sensible defaults (regular events, all statuses).
func DefaultFetchOptions(start, end time.Time) FetchOptions {
	return FetchOptions{
		Start:           start,
		End:             end,
		IncludeTypes:    []EventType{TypeDefault},
		IncludeStatuses: nil, // all statuses
	}
}

// Provider represents a calendar source (Google, iCloud, Local .ics, etc).
type Provider interface {
	// ID returns the unique identifier from the config (e.g. "work_calendar")
	ID() string
	// Name returns a human-readable label (e.g. "Work Account")
	Name() string
	// FetchEvents retrieves events matching the given options.
	// This should block until done or context is cancelled.
	FetchEvents(ctx context.Context, opts FetchOptions) ([]Event, error)
}
