package outlook

import (
	"encoding/json"
	"os"

	"github.com/theakshaypant/tsk/internal/core"

	"golang.org/x/oauth2"
)

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
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

// deduplicateEvents merges events that share the same DedupeKey (ICalUID).
func deduplicateEvents(events []core.Event) []core.Event {
	seen := make(map[string]int)
	var result []core.Event

	for _, event := range events {
		if event.DedupeKey == "" {
			result = append(result, event)
			continue
		}

		if idx, exists := seen[event.DedupeKey]; exists {
			result[idx].Calendars = append(result[idx].Calendars, core.CalendarResponse{
				Calendar: event.Calendar,
				Status:   event.Status,
				URL:      event.URL,
			})
		} else {
			event.Calendars = []core.CalendarResponse{
				{Calendar: event.Calendar, Status: event.Status, URL: event.URL},
			}
			seen[event.DedupeKey] = len(result)
			result = append(result, event)
		}
	}

	return result
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
