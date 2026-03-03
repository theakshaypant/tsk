package core

import (
	"time"
)

// EventStatus represents the user's response to an event invitation.
type EventStatus int

const (
	StatusAccepted EventStatus = iota
	// User declined
	StatusRejected
	// User marked as tentative
	StatusTentative
	// Awaiting user's response
	StatusAwaiting
	// No response needed (subscribed calendars, self-created events)
	StatusNoResponse
)

// EventType represents the kind of calendar entry.
type EventType int

const (
	TypeDefault      EventType = iota // Regular meeting/event
	TypeOutOfOffice                   // Out of office block
	TypeFocusTime                     // Focus time block
	TypeWorkLocation                  // Working location (home/office)
)

// Calendar represents the calendar an event belongs to.
type Calendar struct {
	// Calendar ID (e.g., "primary", "user@example.com", subscription ID)
	ID string
	// Human-readable name (e.g., "Work", "Holidays in India")
	Name string
}

// CalendarResponse tracks a calendar and the user's response status in it.
// Used when the same event appears in multiple shared calendars.
type CalendarResponse struct {
	Calendar Calendar
	Status   EventStatus
	URL      string // Calendar-specific event page URL
}

// Attachment represents a file linked to an event.
// Only store the link, we do not download the content.
type Attachment struct {
	ID string
	// For example, "Quarterly_Report.pdf", "Agenda.doc"
	Name string
	// The web link (Google Drive link, OneDrive link, etc.)
	URL string
	// For example, "application/pdf" (useful for showing icons like 📄 or 🖼️)
	MimeType string
}

// All adapters (Google, Outlook, etc.) must convert their data to this format.
type Event struct {
	// Unique ID (provided by the source)
	ID string
	// Universal identifier for deduplication across calendars (e.g., ICalUID)
	DedupeKey string
	// The ID of the provider source (e.g., "personal_google")
	ProviderID string
	// Which calendar this event belongs to (primary — first seen)
	Calendar Calendar
	// All calendars this event appears in, with per-calendar status and URL.
	// Populated after deduplication. Empty means single-calendar event.
	Calendars []CalendarResponse
	// Type of calendar entry
	Type EventType
	// Details
	Title       string
	Description string
	Location    string
	Status      EventStatus
	// Calendar event page URL
	URL string
	// Video conferencing link (Google Meet, Zoom, Teams, etc.)
	MeetingLink string
	Attachments []Attachment
	// Timing
	Start    time.Time
	End      time.Time
	IsAllDay bool
	// Recurring event information
	// RecurringEventID is the ID of the master recurring event (empty for non-recurring events)
	RecurringEventID string
	// Metadata
	Metadata map[string]string
}

// Duration returns the length of the event.
func (e Event) Duration() time.Duration {
	return e.End.Sub(e.Start)
}

// InProgress checks if the event is happening right now.
func (e Event) InProgress(now time.Time) bool {
	return now.After(e.Start) && now.Before(e.End)
}

// NeedsResponse checks if this event is awaiting the user's response.
// Returns true for events with StatusAwaiting.
func (e Event) NeedsResponse() bool {
	return e.Status == StatusAwaiting
}

// IsRecurring checks if this event is part of a recurring series.
func (e Event) IsRecurring() bool {
	return e.RecurringEventID != ""
}

// GetProposedTime extracts a proposed time from event metadata if present.
// Returns nil if no proposal is found. Looks for properties set by tsk or
// other compatible clients using the "tsk:proposedStart" and "tsk:proposedEnd" keys.
func (e Event) GetProposedTime() *TimeProposal {
	if e.Metadata == nil {
		return nil
	}

	startStr, hasStart := e.Metadata["tsk:proposedStart"]
	endStr, hasEnd := e.Metadata["tsk:proposedEnd"]

	if !hasStart || !hasEnd {
		return nil
	}

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return nil
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return nil
	}

	return &TimeProposal{
		Start: start,
		End:   end,
	}
}

// ResponseType represents the kind of response to an event invitation
type ResponseType int

const (
	ResponseAccept ResponseType = iota
	ResponseDecline
	ResponseTentative
)

// String returns a human-readable representation of the response type.
func (r ResponseType) String() string {
	switch r {
	case ResponseAccept:
		return "Accept"
	case ResponseDecline:
		return "Decline"
	case ResponseTentative:
		return "Tentative"
	default:
		return "Unknown"
	}
}

// RecurringScope determines which instances of a recurring event to respond to
type RecurringScope int

const (
	// RecurringScopeThisInstance responds only to this single instance
	RecurringScopeThisInstance RecurringScope = iota
	// RecurringScopeAllInstances responds to all instances (past and future)
	RecurringScopeAllInstances
)

// String returns a human-readable representation of the recurring scope.
func (r RecurringScope) String() string {
	switch r {
	case RecurringScopeThisInstance:
		return "This event only"
	case RecurringScopeAllInstances:
		return "All events in the series"
	default:
		return "Unknown"
	}
}

// TimeProposal represents a proposed new time for an event
type TimeProposal struct {
	Start time.Time
	End   time.Time
}

// RespondOptions configures how to respond to an event invitation
type RespondOptions struct {
	Response       ResponseType
	Comment        string
	ProposedTime   *TimeProposal
	RecurringScope RecurringScope // Only used for recurring events
}
