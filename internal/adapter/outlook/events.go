package outlook

import (
	"context"
	"fmt"
	"strings"
	"time"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/users"

	"github.com/theakshaypant/tsk/internal/core"
)

// FetchEvents retrieves events from the user's calendars matching the given options.
func (o *OutlookAdapter) FetchEvents(ctx context.Context, opts core.FetchOptions) ([]core.Event, error) {
	var results []core.Event

	calendarIDs := opts.CalendarIDs
	if len(calendarIDs) == 0 {
		for calID := range o.calendars {
			calendarIDs = append(calendarIDs, calID)
		}
	}

	for _, calID := range calendarIDs {
		if _, exists := o.calendars[calID]; !exists {
			continue
		}
		events, err := o.fetchEventsFromCalendar(ctx, calID, opts)
		if err != nil {
			continue // skip failed calendars
		}
		results = append(results, events...)
	}

	results = deduplicateEvents(results)
	sortEventsByStartTime(results)

	return results, nil
}

func (o *OutlookAdapter) fetchEventsFromCalendar(ctx context.Context, calendarID string, opts core.FetchOptions) ([]core.Event, error) {
	startStr := opts.Start.UTC().Format(time.RFC3339)
	endStr := opts.End.UTC().Format(time.RFC3339)
	selectFields := []string{
		"id", "iCalUId", "subject", "body", "start", "end", "location",
		"isAllDay", "showAs", "responseStatus", "onlineMeeting", "webLink",
		"isOrganizer", "isCancelled", "categories",
	}
	orderBy := []string{"start/dateTime"}
	top := int32(100)

	headers := abstractions.NewRequestHeaders()
	headers.Add("Prefer", `outlook.timezone="UTC"`)

	var result models.EventCollectionResponseable
	var err error

	if calendarID == "default" {
		config := &users.ItemCalendarViewRequestBuilderGetRequestConfiguration{
			QueryParameters: &users.ItemCalendarViewRequestBuilderGetQueryParameters{
				StartDateTime: &startStr,
				EndDateTime:   &endStr,
				Select:        selectFields,
				Orderby:       orderBy,
				Top:           &top,
			},
			Headers: headers,
		}
		result, err = o.client.Me().CalendarView().Get(ctx, config)
	} else {
		config := &users.ItemCalendarsItemCalendarViewRequestBuilderGetRequestConfiguration{
			QueryParameters: &users.ItemCalendarsItemCalendarViewRequestBuilderGetQueryParameters{
				StartDateTime: &startStr,
				EndDateTime:   &endStr,
				Select:        selectFields,
				Orderby:       orderBy,
				Top:           &top,
			},
			Headers: headers,
		}
		result, err = o.client.Me().Calendars().ByCalendarId(calendarID).CalendarView().Get(ctx, config)
	}

	if err != nil {
		return nil, fmt.Errorf("fetch calendar view: %w", err)
	}

	// Use PageIterator for automatic pagination
	calendarName := o.calendars[calendarID]
	var results []core.Event

	pageIterator, err := msgraphcore.NewPageIterator[models.Eventable](
		result,
		o.client.GetAdapter(),
		models.CreateEventCollectionResponseFromDiscriminatorValue,
	)
	if err != nil {
		return nil, fmt.Errorf("create page iterator: %w", err)
	}

	err = pageIterator.Iterate(ctx, func(item models.Eventable) bool {
		if derefBool(item.GetIsCancelled()) {
			return true // skip cancelled, continue
		}

		event := parseGraphEvent(o.ID(), item, calendarID, calendarName)

		// Treat timed events as all-day if they span the entire viewed day
		if !event.IsAllDay && !event.Start.After(opts.Start) && !event.End.Before(opts.End) {
			event.IsAllDay = true
		}

		if len(opts.IncludeTypes) > 0 && !containsType(opts.IncludeTypes, event.Type) {
			return true
		}
		if len(opts.IncludeStatuses) > 0 && !containsStatus(opts.IncludeStatuses, event.Status) {
			return true
		}
		if opts.ExcludeAllDay && event.IsAllDay {
			return true
		}

		results = append(results, event)
		return true
	})

	if err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return results, nil
}

// parseGraphEvent converts a Graph SDK event into our unified core.Event.
func parseGraphEvent(providerID string, item models.Eventable, calendarID, calendarName string) core.Event {
	// Event type — map from Outlook's showAs + categories
	eventType := core.TypeDefault
	if showAs := item.GetShowAs(); showAs != nil {
		switch *showAs {
		case models.OOF_FREEBUSYSTATUS:
			eventType = core.TypeOutOfOffice
		case models.WORKINGELSEWHERE_FREEBUSYSTATUS:
			eventType = core.TypeWorkLocation
		}
	}
	for _, cat := range item.GetCategories() {
		lower := strings.ToLower(cat)
		if lower == "focus time" || lower == "focustime" {
			eventType = core.TypeFocusTime
		}
	}

	// Parse times (we request UTC via Prefer header)
	startTime := parseSDKDateTime(item.GetStart())
	endTime := parseSDKDateTime(item.GetEnd())

	// Response status
	status := parseSDKEventStatus(item)

	// Meeting link (Teams, Zoom, etc.)
	meetingLink := ""
	if om := item.GetOnlineMeeting(); om != nil {
		if joinURL := om.GetJoinUrl(); joinURL != nil {
			meetingLink = *joinURL
		}
	}

	// Description — body.content may be HTML or text
	description := ""
	if body := item.GetBody(); body != nil {
		if content := body.GetContent(); content != nil {
			description = *content
		}
	}

	// Location
	location := ""
	if loc := item.GetLocation(); loc != nil {
		if dn := loc.GetDisplayName(); dn != nil {
			location = *dn
		}
	}

	return core.Event{
		ID:         derefStr(item.GetId()),
		DedupeKey:  derefStr(item.GetICalUId()),
		ProviderID: providerID,
		Calendar: core.Calendar{
			ID:   calendarID,
			Name: calendarName,
		},
		Type:        eventType,
		Title:       derefStr(item.GetSubject()),
		Description: description,
		Location:    location,
		Status:      status,
		URL:         derefStr(item.GetWebLink()),
		MeetingLink: meetingLink,
		Start:       startTime,
		End:         endTime,
		IsAllDay:    derefBool(item.GetIsAllDay()),
	}
}

// parseSDKDateTime converts a Graph SDK DateTimeTimeZone to time.Time.
// Times are in UTC because we set the Prefer: outlook.timezone="UTC" header.
func parseSDKDateTime(dt models.DateTimeTimeZoneable) time.Time {
	if dt == nil {
		return time.Time{}
	}
	dateTimeStr := dt.GetDateTime()
	if dateTimeStr == nil {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02T15:04:05.0000000",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, *dateTimeStr); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// parseSDKEventStatus maps Outlook response status to our unified EventStatus.
func parseSDKEventStatus(item models.Eventable) core.EventStatus {
	rs := item.GetResponseStatus()
	if rs == nil {
		return core.StatusNoResponse
	}
	resp := rs.GetResponse()
	if resp == nil {
		return core.StatusNoResponse
	}
	switch *resp {
	case models.ACCEPTED_RESPONSETYPE:
		return core.StatusAccepted
	case models.ORGANIZER_RESPONSETYPE:
		return core.StatusAccepted
	case models.DECLINED_RESPONSETYPE:
		return core.StatusRejected
	case models.TENTATIVELYACCEPTED_RESPONSETYPE:
		return core.StatusTentative
	case models.NOTRESPONDED_RESPONSETYPE:
		return core.StatusAwaiting
	case models.NONE_RESPONSETYPE:
		return core.StatusNoResponse
	default:
		return core.StatusNoResponse
	}
}
