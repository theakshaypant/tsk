package google

import (
	"context"
	"fmt"
	"time"

	"github.com/theakshaypant/tsk/internal/core"

	"google.golang.org/api/calendar/v3"
)

func (g *GoogleAdapter) FetchEvents(ctx context.Context, opts core.FetchOptions) ([]core.Event, error) {
	var results []core.Event

	// Determine which calendars to fetch from
	calendarIDs := opts.CalendarIDs
	if len(calendarIDs) == 0 {
		// No filter specified - fetch from all calendars
		for calID := range g.calendars {
			calendarIDs = append(calendarIDs, calID)
		}
	}

	// Fetch from selected calendars
	for _, calID := range calendarIDs {
		// Skip if calendar doesn't exist
		if _, exists := g.calendars[calID]; !exists {
			continue
		}
		events, err := g.fetchEventsFromCalendar(ctx, calID, opts)
		if err != nil {
			// Log warning but continue with other calendars
			continue
		}
		results = append(results, events...)
	}

	results = deduplicateEvents(results)

	// Sort by start time
	sortEventsByStartTime(results)

	return results, nil
}

func (g *GoogleAdapter) fetchEventsFromCalendar(ctx context.Context, calendarID string, opts core.FetchOptions) ([]core.Event, error) {
	// Google API requires RFC3339 format
	tMin := opts.Start.Format(time.RFC3339)
	tMax := opts.End.Format(time.RFC3339)

	var results []core.Event
	pageToken := ""

	calendarName := g.calendars[calendarID]

	for {
		req := g.service.Events.List(calendarID).
			ShowDeleted(false).
			SingleEvents(true).
			TimeMin(tMin).
			TimeMax(tMax).
			OrderBy("startTime").
			Context(ctx)

		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		eventsResult, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("api call failed for calendar %s: %w", calendarID, err)
		}

		for _, item := range eventsResult.Items {
			event := g.parseEvent(item, calendarID, calendarName)

			// Treat timed events as all-day if they span the entire viewed day.
			// These are multi-day spanning events that Google sends with
			// DateTime instead of Date.
			if !event.IsAllDay && !event.Start.After(opts.Start) && !event.End.Before(opts.End) {
				event.IsAllDay = true
			}

			// Filter by event type
			if len(opts.IncludeTypes) > 0 && !containsType(opts.IncludeTypes, event.Type) {
				continue
			}

			// Filter by status
			if len(opts.IncludeStatuses) > 0 && !containsStatus(opts.IncludeStatuses, event.Status) {
				continue
			}

			// Filter out all-day events
			if opts.ExcludeAllDay && event.IsAllDay {
				continue
			}

			results = append(results, event)
		}

		pageToken = eventsResult.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return results, nil
}

// parseEvent converts a Google Calendar event to our unified Event type.
func (g *GoogleAdapter) parseEvent(item *calendar.Event, calendarID, calendarName string) core.Event {
	// Event type
	eventType := core.TypeDefault
	switch item.EventType {
	case "outOfOffice":
		eventType = core.TypeOutOfOffice
	case "focusTime":
		eventType = core.TypeFocusTime
	case "workingLocation":
		eventType = core.TypeWorkLocation
	}

	// Timing (all day vs time specific)
	var startTime, endTime time.Time
	isAllDay := false

	if item.Start.DateTime != "" {
		startTime, _ = time.Parse(time.RFC3339, item.Start.DateTime)
		endTime, _ = time.Parse(time.RFC3339, item.End.DateTime)
	} else {
		// All day event (YYYY-MM-DD)
		startTime, _ = time.Parse("2006-01-02", item.Start.Date)
		// Google end dates for all-day events are exclusive (next day),
		// but we keep it raw for now or adjust based on preference.
		endTime, _ = time.Parse("2006-01-02", item.End.Date)
		isAllDay = true
	}

	// Attachments
	var attachments []core.Attachment
	for _, att := range item.Attachments {
		attachments = append(attachments, core.Attachment{
			ID:       att.FileId,
			Name:     att.Title,
			URL:      att.FileUrl,
			MimeType: att.MimeType,
		})
	}

	// Check the user's response from attendees list
	status := g.parseEventStatus(item)

	// Extract meeting link from conference data
	meetingLink := extractMeetingLink(item)

	// Build unified Event
	return core.Event{
		ID:         item.Id,
		DedupeKey:  item.ICalUID,
		ProviderID: g.ID(),
		Calendar: core.Calendar{
			ID:   calendarID,
			Name: calendarName,
		},
		Type:        eventType,
		Title:       item.Summary,
		Description: item.Description,
		Location:    item.Location,
		Status:      status,
		URL:         item.HtmlLink,
		MeetingLink: meetingLink,
		Start:       startTime,
		End:         endTime,
		IsAllDay:    isAllDay,
		Attachments: attachments,
	}
}

// extractMeetingLink gets the video conferencing link from Google Calendar event.
func extractMeetingLink(item *calendar.Event) string {
	// First check ConferenceData (Google Meet, Zoom, etc.)
	if item.ConferenceData != nil {
		for _, entry := range item.ConferenceData.EntryPoints {
			if entry.EntryPointType == "video" {
				return entry.Uri
			}
		}
	}

	// Fallback to legacy HangoutLink
	if item.HangoutLink != "" {
		return item.HangoutLink
	}

	return ""
}

// parseEventStatus determines the user's response status for an event.
func (g *GoogleAdapter) parseEventStatus(item *calendar.Event) core.EventStatus {
	// Check if user is in attendees list
	for _, attendee := range item.Attendees {
		if attendee.Self {
			switch attendee.ResponseStatus {
			case "declined":
				return core.StatusRejected
			case "tentative":
				return core.StatusTentative
			case "needsAction":
				return core.StatusAwaiting
			case "accepted":
				return core.StatusAccepted
			}
		}
	}

	// No attendees or user not in list - could be:
	// 1. Event created by user (no response needed)
	// 2. Subscribed calendar (holidays, etc.)
	// 3. Imported event
	if len(item.Attendees) == 0 {
		if item.Status == "cancelled" {
			return core.StatusRejected
		}
		// No response needed for self-created or subscribed calendar events
		return core.StatusNoResponse
	}

	// User not found in attendees but attendees exist - likely a subscribed calendar
	return core.StatusNoResponse
}
