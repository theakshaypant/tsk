package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/theakshaypant/tsk/internal/core"
)

var (
	respondAccept      bool
	respondDecline     bool
	respondTentative   bool
	respondMessage     string
	respondPropose     string
	respondAllInstances bool
)

var respondCmd = &cobra.Command{
	Use:   "respond <calendar:event-id>",
	Short: "Respond to a calendar event invitation",
	Long: `Respond to a calendar event invitation with accept, decline, or tentative.

Optionally include a message to the organizer and propose a new time.

Event reference format: calendarID:eventID
  Example: primary:abc123xyz

To find event IDs, enable display.id in your config and run 'tsk' or 'tsk next'.

Examples:
  # Accept an event
  tsk respond primary:abc123 --accept

  # Decline with a message
  tsk respond primary:abc123 --decline --message "Sorry, I have a conflict"

  # Tentatively accept with proposed new time (simplest - just times)
  tsk respond primary:abc123 --tentative --propose "14:00/15:00"

  # Propose time with date (uses local timezone)
  tsk respond primary:abc123 --tentative --propose "2026-03-04T14:00/2026-03-04T15:00"

  # Propose time in UTC (Z suffix)
  tsk respond primary:abc123 --tentative --propose "2026-03-04T14:00Z/2026-03-04T15:00Z"

  # Propose time in a specific timezone (PST/PDT)
  tsk respond primary:abc123 --tentative --propose "2026-03-04T14:00-08:00/2026-03-04T15:00-08:00"

  # Accept with message
  tsk respond primary:abc123 --accept -m "Looking forward to it!"

  # Decline all instances of a recurring event
  tsk respond primary:abc123 --decline --all-instances

Recurring events:
  By default, responds to a single instance. Use --all-instances to respond to
  all occurrences of a recurring event series.

Note: Currently only supported for Google Calendar. Outlook support coming soon.`,
	Args: cobra.ExactArgs(1),
	RunE: runRespond,
}

func init() {
	rootCmd.AddCommand(respondCmd)
	respondCmd.Flags().BoolVar(&respondAccept, "accept", false, "Accept the event")
	respondCmd.Flags().BoolVar(&respondDecline, "decline", false, "Decline the event")
	respondCmd.Flags().BoolVar(&respondTentative, "tentative", false, "Mark as tentative")
	respondCmd.Flags().StringVarP(&respondMessage, "message", "m", "", "Optional message to organizer")
	respondCmd.Flags().StringVar(&respondPropose, "propose", "", "Propose new time (format: start/end in RFC3339)")
	respondCmd.Flags().BoolVar(&respondAllInstances, "all-instances", false, "Respond to all instances of a recurring event")
}

func runRespond(cmd *cobra.Command, args []string) error {
	// Parse event reference
	eventRef := args[0]
	calendarID, eventID, err := parseEventReference(eventRef)
	if err != nil {
		return err
	}

	// Validate exactly one response type is selected
	responseCount := 0
	var responseType core.ResponseType
	if respondAccept {
		responseCount++
		responseType = core.ResponseAccept
	}
	if respondDecline {
		responseCount++
		responseType = core.ResponseDecline
	}
	if respondTentative {
		responseCount++
		responseType = core.ResponseTentative
	}

	if responseCount == 0 {
		return fmt.Errorf("must specify exactly one of --accept, --decline, or --tentative")
	}
	if responseCount > 1 {
		return fmt.Errorf("cannot specify multiple response types (choose one: --accept, --decline, or --tentative)")
	}

	// Build response options
	recurringScope := core.RecurringScopeThisInstance
	if respondAllInstances {
		recurringScope = core.RecurringScopeAllInstances
	}

	opts := core.RespondOptions{
		Response:       responseType,
		Comment:        respondMessage,
		RecurringScope: recurringScope,
	}

	// Parse proposed time if provided
	// Need to fetch the event first to get its date for time-only proposals
	if respondPropose != "" {
		// Fetch the event to get its date
		events, err := adapter.FetchEvents(cmd.Context(), core.FetchOptions{
			Start:        time.Now().Add(-24 * time.Hour),
			End:          time.Now().Add(365 * 24 * time.Hour),
			CalendarIDs:  []string{calendarID},
			IncludeTypes: []core.EventType{core.TypeDefault, core.TypeOutOfOffice, core.TypeFocusTime, core.TypeWorkLocation},
		})
		if err != nil {
			return fmt.Errorf("failed to fetch event for date reference: %w", err)
		}

		var eventDate time.Time
		for _, e := range events {
			if e.ID == eventID || e.Calendar.ID == calendarID {
				eventDate = e.Start
				break
			}
		}

		if eventDate.IsZero() {
			// Fallback to today if event not found
			eventDate = time.Now()
		}

		proposal, err := parseProposedTime(respondPropose, eventDate)
		if err != nil {
			return fmt.Errorf("invalid proposed time: %w", err)
		}
		opts.ProposedTime = proposal
	}

	// Call the adapter to respond
	err = adapter.RespondToEvent(cmd.Context(), calendarID, eventID, opts)
	if err != nil {
		return formatRespondError(err)
	}

	// Show success message
	printRespondSuccess(responseType, opts)

	return nil
}

// parseEventReference splits "calendarID:eventID" into separate components.
func parseEventReference(ref string) (calendarID, eventID string, err error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid event reference format: %s\n\nExpected format: calendarID:eventID\nExample: primary:abc123xyz\n\nTip: Enable display.id in config to see event IDs", ref)
	}

	calendarID = strings.TrimSpace(parts[0])
	eventID = strings.TrimSpace(parts[1])

	if calendarID == "" || eventID == "" {
		return "", "", fmt.Errorf("calendar ID and event ID cannot be empty")
	}

	return calendarID, eventID, nil
}

// parseProposedTime parses a proposed time in the format "start/end".
// Accepts multiple formats:
//   - 14:00/15:00 (uses event date, local timezone)
//   - 2026-03-04T14:00/2026-03-04T15:00 (local timezone)
//   - 2026-03-04T14:00:00Z/2026-03-04T15:00:00Z (UTC)
//   - 2026-03-04T14:00:00-08:00/2026-03-04T15:00:00-08:00 (Pacific Time)
func parseProposedTime(proposal string, eventDate time.Time) (*core.TimeProposal, error) {
	parts := strings.SplitN(proposal, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected format: start/end\nExamples:\n  14:00/15:00 (uses event date)\n  2026-03-04T14:00/15:00\n  2026-03-04T14:00:00Z (UTC)")
	}

	start, err := parseTimeWithEventDate(strings.TrimSpace(parts[0]), eventDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}

	end, err := parseTimeWithEventDate(strings.TrimSpace(parts[1]), eventDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}

	if !end.After(start) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	return &core.TimeProposal{
		Start: start,
		End:   end,
	}, nil
}

// parseTimeWithEventDate parses a time string, with smart defaults:
// - Uses event date if only time is provided (14:00)
// - Uses local timezone if no timezone is specified
// - Seconds are optional
func parseTimeWithEventDate(timeStr string, eventDate time.Time) (time.Time, error) {
	// First try parsing as full RFC3339 (with timezone)
	t, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t, nil
	}

	// Try parsing without timezone: 2006-01-02T15:04:05
	t, err = time.ParseInLocation("2006-01-02T15:04:05", timeStr, time.Local)
	if err == nil {
		return t, nil
	}

	// Try parsing without timezone or seconds: 2006-01-02T15:04
	t, err = time.ParseInLocation("2006-01-02T15:04", timeStr, time.Local)
	if err == nil {
		return t, nil
	}

	// Try parsing as time-only with seconds: 15:04:05
	t, err = time.ParseInLocation("15:04:05", timeStr, time.Local)
	if err == nil {
		// Use event's date with this time
		return time.Date(eventDate.Year(), eventDate.Month(), eventDate.Day(),
			t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
	}

	// Try parsing as time-only without seconds: 15:04
	t, err = time.ParseInLocation("15:04", timeStr, time.Local)
	if err == nil {
		// Use event's date with this time
		return time.Date(eventDate.Year(), eventDate.Month(), eventDate.Day(),
			t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}

	return time.Time{}, fmt.Errorf("invalid time format\nSupported formats:\n  14:00 (time only)\n  2026-03-04T14:00 (date + time)\n  2026-03-04T14:00:00Z (with timezone)")
}

// formatRespondError converts core errors to user-friendly messages.
func formatRespondError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, core.ErrInsufficientScope):
		return fmt.Errorf("insufficient permissions to respond to events\n\nPlease re-authenticate with updated permissions:\n  tsk auth\n\nThis grants permission to respond to events (but not delete or modify calendars)")
	case errors.Is(err, core.ErrNotImplemented):
		return fmt.Errorf("event response is not yet supported for this provider\n\nCurrently supported:\n  ✅ Google Calendar\n  🚧 Outlook (coming soon)")
	case errors.Is(err, core.ErrNotAttendee):
		return fmt.Errorf("you are not an attendee of this event\n\nYou can only respond to events where you are invited as an attendee")
	case errors.Is(err, core.ErrIsOrganizer):
		return fmt.Errorf("you cannot respond to your own event\n\nYou are the organizer of this event")
	default:
		return fmt.Errorf("failed to respond to event: %w", err)
	}
}

// printRespondSuccess shows a confirmation message after successful response.
func printRespondSuccess(responseType core.ResponseType, opts core.RespondOptions) {
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Println("  ✅ RESPONSE SENT")
	fmt.Println("─────────────────────────────────────────────────")
	fmt.Println()

	var responseText string
	switch responseType {
	case core.ResponseAccept:
		responseText = "Accepted"
	case core.ResponseDecline:
		responseText = "Declined"
	case core.ResponseTentative:
		responseText = "Tentatively accepted"
	}
	fmt.Printf("  Response: %s\n", responseText)

	// Show scope for recurring events
	if opts.RecurringScope == core.RecurringScopeAllInstances {
		fmt.Printf("  Scope:    %s\n", opts.RecurringScope.String())
	}

	if opts.Comment != "" {
		// Don't show the auto-generated proposal text in the message
		message := opts.Comment
		if opts.ProposedTime != nil {
			// Remove the auto-appended proposal text
			proposalText := fmt.Sprintf("Proposed time: %s - %s",
				opts.ProposedTime.Start.Format(time.RFC3339),
				opts.ProposedTime.End.Format(time.RFC3339))
			message = strings.TrimSuffix(message, "\n\n"+proposalText)
			message = strings.TrimSuffix(message, proposalText)
		}
		if message != "" {
			fmt.Printf("  Message: %s\n", message)
		}
	}

	if opts.ProposedTime != nil {
		fmt.Println()
		fmt.Println("  Proposed new time:")
		fmt.Printf("    Start: %s\n", opts.ProposedTime.Start.Format("Mon Jan 2, 2006 at 3:04 PM MST"))
		fmt.Printf("    End:   %s\n", opts.ProposedTime.End.Format("3:04 PM MST"))
		fmt.Println()
		fmt.Println("  Sent to organizer:")
		fmt.Println("    • Human-readable format converted to event's timezone")
		fmt.Println("    • RFC3339 format with your timezone preserved")
		fmt.Println("    • Both formats included in comment for clarity")
		fmt.Println()
		fmt.Println("  Stored locally (your calendar only):")
		fmt.Println("    • Structured data: tsk:proposedStart, tsk:proposedEnd")
	}

	fmt.Println()
	fmt.Println("─────────────────────────────────────────────────")
}
