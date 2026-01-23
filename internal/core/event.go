package core

import (
	"time"
)

// EventStatus represents the user's response to an event invitation.
type EventStatus int

const (
	StatusAccepted EventStatus = iota
	StatusRejected
	StatusTentative
	StatusAwaiting
)

// EventType represents the kind of calendar entry.
type EventType int

const (
	TypeDefault      EventType = iota // Regular meeting/event
	TypeOutOfOffice                   // Out of office block
	TypeFocusTime                     // Focus time block
	TypeWorkLocation                  // Working location (home/office)
)

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
	// The ID of the provider source (e.g., "personal_google")
	ProviderID string
	// Type of calendar entry
	Type EventType
	// Details
	Title       string
	Description string
	Location    string
	Status      EventStatus
	// Video call link or event page
	URL         string
	Attachments []Attachment
	// Timing
	Start    time.Time
	End      time.Time
	IsAllDay bool
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
