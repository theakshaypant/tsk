package google

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/theakshaypant/tsk/internal/core"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type GoogleAdapter struct {
	id        string
	name      string
	client    *http.Client
	service   *calendar.Service
	config    *oauth2.Config
	credsFile string
	tokenFile string
	calendars map[string]string
}

func NewGoogleAdapter(id, name, credsFile, tokenFile string) *GoogleAdapter {
	return &GoogleAdapter{
		id:        id,
		name:      name,
		credsFile: credsFile,
		tokenFile: tokenFile,
		calendars: make(map[string]string),
	}
}

func (g *GoogleAdapter) ID() string   { return g.id }
func (g *GoogleAdapter) Name() string { return g.name }

// Login loads credentials and token, then initializes the Calendar service.
// Run `go run ./cmd/auth/main.go credentials.json` first to generate token.json.
func (g *GoogleAdapter) Login(ctx context.Context) error {
	b, err := os.ReadFile(g.credsFile)
	if err != nil {
		return fmt.Errorf("read credentials file: %w", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return fmt.Errorf("parse credentials: %w", err)
	}
	g.config = config

	tok, err := tokenFromFile(g.tokenFile)
	if err != nil {
		return fmt.Errorf("read token file (run cmd/auth first): %w", err)
	}

	g.client = g.config.Client(ctx, tok)
	g.service, err = calendar.NewService(ctx, option.WithHTTPClient(g.client))
	if err != nil {
		return err
	}

	// Fetch calendar list to get names for all calendars
	if err := g.loadCalendarList(ctx); err != nil {
		return fmt.Errorf("load calendar list: %w", err)
	}

	return nil
}

// loadCalendarList fetches all calendars the user has access to.
func (g *GoogleAdapter) loadCalendarList(ctx context.Context) error {
	calList, err := g.service.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return err
	}

	for _, cal := range calList.Items {
		g.calendars[cal.Id] = cal.Summary
	}
	return nil
}

// Calendars returns a list of available calendars (ID -> Name).
func (g *GoogleAdapter) Calendars() map[string]string {
	return g.calendars
}

// tokenFromFile reads an OAuth token from a JSON file.
func tokenFromFile(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

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

// deduplicateEvents merges events that share the same DedupeKey (ICalUID).
// The first occurrence becomes the primary; subsequent occurrences add their
// calendar and status to the Calendars slice.
func deduplicateEvents(events []core.Event) []core.Event {
	seen := make(map[string]int) // DedupeKey -> index in result
	var result []core.Event

	for _, event := range events {
		if event.DedupeKey == "" {
			// No dedup key — keep as-is
			result = append(result, event)
			continue
		}

		if idx, exists := seen[event.DedupeKey]; exists {
			// Duplicate — merge calendar info into the existing event
			result[idx].Calendars = append(result[idx].Calendars, core.CalendarResponse{
				Calendar: event.Calendar,
				Status:   event.Status,
				URL:      event.URL,
			})
		} else {
			// First occurrence — initialize Calendars with this event's own info
			event.Calendars = []core.CalendarResponse{
				{
					Calendar: event.Calendar,
					Status:   event.Status,
					URL:      event.URL,
				},
			}
			seen[event.DedupeKey] = len(result)
			result = append(result, event)
		}
	}

	return result
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

func sortEventsByStartTime(events []core.Event) {
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].Start.Before(events[i].Start) {
				events[i], events[j] = events[j], events[i]
			}
		}
	}
}

func containsType(types []core.EventType, t core.EventType) bool {
	for _, v := range types {
		if v == t {
			return true
		}
	}
	return false
}

func containsStatus(statuses []core.EventStatus, s core.EventStatus) bool {
	for _, v := range statuses {
		if v == s {
			return true
		}
	}
	return false
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
