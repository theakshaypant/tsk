package google

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/theakshaypant/tsk/internal/core"

	"google.golang.org/api/calendar/v3"
)

func (g *GoogleAdapter) RespondToEvent(ctx context.Context, calendarID, eventID string, opts core.RespondOptions) error {
	// Fetch the event to determine if it's recurring and validate access
	event, err := g.service.Events.Get(calendarID, eventID).Context(ctx).Do()
	if err != nil {
		if isInsufficientScopeError(err) {
			return core.ErrInsufficientScope
		}
		return fmt.Errorf("failed to fetch event: %w", err)
	}

	if event.Status == "cancelled" {
		return fmt.Errorf("cannot respond to a cancelled event")
	}

	// Determine which event ID to update based on recurring scope
	// Otherwise use the instance ID (default: RecurringScopeThisInstance)
	targetEventID := eventID
	if opts.RecurringScope == core.RecurringScopeAllInstances && event.RecurringEventId != "" {
		// Respond to all instances by updating the master recurring event
		targetEventID = event.RecurringEventId
		// Re-fetch the master event
		event, err = g.service.Events.Get(calendarID, targetEventID).Context(ctx).Do()
		if err != nil {
			if isInsufficientScopeError(err) {
				return core.ErrInsufficientScope
			}
			return fmt.Errorf("failed to fetch recurring event: %w", err)
		}
	}

	// Find the attendee representing the authenticated user
	var userAttendee *calendar.EventAttendee
	for i, attendee := range event.Attendees {
		if attendee.Self {
			userAttendee = event.Attendees[i]
			break
		}
	}

	// Validate user is an attendee
	if userAttendee == nil {
		return core.ErrNotAttendee
	}

	// Validate user is not the organizer
	if userAttendee.Organizer {
		return core.ErrIsOrganizer
	}

	// Map ResponseType to Google's response status
	var responseStatus string
	switch opts.Response {
	case core.ResponseAccept:
		responseStatus = "accepted"
	case core.ResponseDecline:
		responseStatus = "declined"
	case core.ResponseTentative:
		responseStatus = "tentative"
	default:
		return fmt.Errorf("invalid response type")
	}

	// Update the attendee's response
	userAttendee.ResponseStatus = responseStatus

	// Add comment if provided
	comment := opts.Comment
	if opts.ProposedTime != nil {
		// Store proposed time in extendedProperties.private for local reference
		// (Note: Extended properties set by attendees do NOT propagate to the organizer.
		// Only the comment and responseStatus fields propagate. We store in private
		// properties so the attendee can see their proposal when viewing the event.)
		if event.ExtendedProperties == nil {
			event.ExtendedProperties = &calendar.EventExtendedProperties{}
		}
		if event.ExtendedProperties.Private == nil {
			event.ExtendedProperties.Private = make(map[string]string)
		}

		// Store structured data in private properties (attendee's copy only)
		event.ExtendedProperties.Private["tsk:proposedStart"] = opts.ProposedTime.Start.Format(time.RFC3339)
		event.ExtendedProperties.Private["tsk:proposedEnd"] = opts.ProposedTime.End.Format(time.RFC3339)
		event.ExtendedProperties.Private["tsk:proposedBy"] = "tsk-cli"

		// Add to comment - this IS visible to the organizer
		// Format in a human-friendly way with timezone information
		proposedStart := opts.ProposedTime.Start
		proposedEnd := opts.ProposedTime.End

		// If the event has a timezone, convert proposed times to that timezone for clarity
		if event.Start != nil && event.Start.TimeZone != "" {
			if loc, err := time.LoadLocation(event.Start.TimeZone); err == nil {
				proposedStart = proposedStart.In(loc)
				proposedEnd = proposedEnd.In(loc)
			}
		}

		// Format with both human-readable and RFC3339 for clarity
		proposalText := fmt.Sprintf("Proposed new time:\n  %s to %s\n  (%s to %s)",
			proposedStart.Format("Mon Jan 2, 2006 at 3:04 PM MST"),
			proposedEnd.Format("3:04 PM MST"),
			opts.ProposedTime.Start.Format(time.RFC3339),
			opts.ProposedTime.End.Format(time.RFC3339))
		if comment != "" {
			comment = comment + "\n\n" + proposalText
		} else {
			comment = proposalText
		}
	}
	if comment != "" {
		userAttendee.Comment = comment
	}

	// Update the event (use targetEventID which may be the master recurring event)
	_, err = g.service.Events.Update(calendarID, targetEventID, event).
		SendUpdates("all").
		Context(ctx).
		Do()
	if err != nil {
		if isInsufficientScopeError(err) {
			return core.ErrInsufficientScope
		}
		return fmt.Errorf("failed to update event: %w", err)
	}

	return nil
}

// isInsufficientScopeError checks if the error is due to insufficient OAuth scope.
func isInsufficientScopeError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "insufficient") &&
		(strings.Contains(errStr, "scope") || strings.Contains(errStr, "permission"))
}
