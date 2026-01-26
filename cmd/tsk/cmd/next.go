package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/theakshaypant/tsk/internal/core"
)

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Show the next upcoming event",
	Long: `Show detailed information about the next upcoming event on your calendar.

Supports all the same filters as the main command.`,
	RunE: runNext,
}

func init() {
	rootCmd.AddCommand(nextCmd)
	// All filter flags are inherited from root as persistent flags
}

func runNext(cmd *cobra.Command, args []string) error {
	// Read from viper to get profile settings, with flag overrides
	calendars := viper.GetString("calendars")
	showOOO := viper.GetBool("ooo")
	showFocus := viper.GetBool("focus")
	showWorkLoc := viper.GetBool("workloc")
	allTypes := viper.GetBool("all_types")
	onlyAccepted := viper.GetBool("accepted")
	showSubscribed := viper.GetBool("subscribed")
	smartOOO := viper.GetBool("smart_ooo")
	primaryCalendar := viper.GetString("primary_calendar")
	noAllDay := viper.GetBool("no_allday")

	now := time.Now()
	var start, end time.Time

	// Determine date range
	fromStr := viper.GetString("from")
	toStr := viper.GetString("to")

	if fromStr != "" || toStr != "" {
		if fromStr != "" {
			var err error
			start, err = parseDate(fromStr, now)
			if err != nil {
				return err
			}
		} else {
			start = now
		}

		if toStr != "" {
			var err error
			end, err = parseDate(toStr, now)
			if err != nil {
				return err
			}
			end = end.Add(24*time.Hour - time.Second)
		} else {
			days := viper.GetInt("days")
			end = start.Add(time.Duration(days) * 24 * time.Hour)
		}
	} else {
		days := viper.GetInt("days")
		start = now
		end = now.Add(time.Duration(days) * 24 * time.Hour)
	}

	opts := core.FetchOptions{
		Start: start,
		End:   end,
	}

	// Calendar filter
	if calendars != "" {
		filterNames := strings.Split(calendars, ",")
		calendarIDs := resolveCalendarNames(filterNames, adapter.Calendars())
		if len(calendarIDs) == 0 {
			return fmt.Errorf("no matching calendars found for: %s\nUse 'tsk calendars' to see available calendars", calendars)
		}
		opts.CalendarIDs = calendarIDs
	}

	// Event type filter
	if allTypes {
		opts.IncludeTypes = []core.EventType{
			core.TypeDefault,
			core.TypeOutOfOffice,
			core.TypeFocusTime,
			core.TypeWorkLocation,
		}
	} else {
		opts.IncludeTypes = []core.EventType{core.TypeDefault}
		if showOOO {
			opts.IncludeTypes = append(opts.IncludeTypes, core.TypeOutOfOffice)
		}
		if showFocus {
			opts.IncludeTypes = append(opts.IncludeTypes, core.TypeFocusTime)
		}
		if showWorkLoc {
			opts.IncludeTypes = append(opts.IncludeTypes, core.TypeWorkLocation)
		}
	}

	// Status filter
	if onlyAccepted {
		opts.IncludeStatuses = []core.EventStatus{core.StatusAccepted}
		if showSubscribed {
			opts.IncludeStatuses = append(opts.IncludeStatuses, core.StatusNoResponse)
		}
	}

	events, err := adapter.FetchEvents(cmd.Context(), opts)
	if err != nil {
		return fmt.Errorf("failed to fetch events: %w", err)
	}

	// Apply smart OOO filter
	if smartOOO {
		primaryID := detectPrimaryCalendar(primaryCalendar)
		oooPeriods := getOOOPeriods(cmd.Context(), now, end, primaryID)
		if len(oooPeriods) > 0 {
			events = filterEventsOutsideOOO(events, oooPeriods, primaryID)
		}
	}

	// Find eligible events (filter out all-day if needed, skip past events)
	var eligible []core.Event
	for _, e := range events {
		// Skip all-day events if --no-allday is set
		if noAllDay && e.IsAllDay {
			continue
		}

		// Include events that start after now, or are in progress (and not all-day)
		if e.Start.After(now) || (e.InProgress(now) && !e.IsAllDay) {
			eligible = append(eligible, e)
		}
	}

	if len(eligible) == 0 {
		fmt.Println("No upcoming events found.")
		return nil
	}

	// Find all events that start at the same time as the first eligible event
	nextStart := eligible[0].Start
	var concurrent []core.Event
	for _, e := range eligible {
		if e.Start.Equal(nextStart) {
			concurrent = append(concurrent, e)
		} else {
			break // Events are sorted by start time
		}
	}

	// Show conflict warning if multiple events at the same time
	if len(concurrent) > 1 {
		printConcurrentEvents(concurrent, now)
	} else {
		printNextEvent(concurrent[0], now)
	}

	return nil
}

func printConcurrentEvents(events []core.Event, now time.Time) {
	first := events[0]

	fmt.Println("─────────────────────────────────────────────────")
	fmt.Printf("  ⚠️  CONFLICT: %d EVENTS AT THE SAME TIME\n", len(events))
	fmt.Println("─────────────────────────────────────────────────")

	// Countdown display
	fmt.Println()
	if first.InProgress(now) {
		remaining := first.End.Sub(now)
		fmt.Printf("  🟢 IN PROGRESS - %s remaining\n", formatDurationCompact(remaining))
	} else {
		until := first.Start.Sub(now)
		fmt.Printf("  ⏳ STARTS IN: %s\n", formatCountdown(until))
	}

	opts := DisplayOptionsFromConfig(false)
	opts.ShowInProgress = false // Already shown in header
	opts.ShowDesc = false       // Keep conflict view compact

	for i, event := range events {
		fmt.Printf("\n  EVENT %d of %d\n", i+1, len(events))
		fmt.Println("  ─────────────────────────────────────────────")
		DisplayEvent(event, opts)
	}

	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────")
}

func formatCountdown(d time.Duration) string {
	if d < 0 {
		return "NOW"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	plural := func(n int, unit string) string {
		if n == 1 {
			return fmt.Sprintf("%d %s", n, unit)
		}
		return fmt.Sprintf("%d %ss", n, unit)
	}

	var parts []string
	if days > 0 {
		parts = append(parts, plural(days, "day"))
	}
	if hours > 0 {
		parts = append(parts, plural(hours, "hour"))
	}
	if minutes > 0 {
		parts = append(parts, plural(minutes, "minute"))
	}

	if len(parts) == 0 {
		return "less than a minute"
	}
	return strings.Join(parts, ", ")
}

func printNextEvent(event core.Event, now time.Time) {
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Println("  NEXT EVENT")
	fmt.Println("─────────────────────────────────────────────────")

	// Countdown display
	fmt.Println()
	if event.InProgress(now) {
		remaining := event.End.Sub(now)
		fmt.Printf("  🟢 IN PROGRESS - %s remaining\n", formatDurationCompact(remaining))
	} else {
		until := event.Start.Sub(now)
		fmt.Printf("  ⏳ STARTS IN: %s\n", formatCountdown(until))
	}
	fmt.Println()

	opts := DisplayOptionsFromConfig(true)
	opts.ShowInProgress = false // Already shown in header
	DisplayEvent(event, opts)

	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────")
}
