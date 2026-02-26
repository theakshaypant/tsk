package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/theakshaypant/tsk/internal/adapter/google"
	"github.com/theakshaypant/tsk/internal/adapter/outlook"
	"github.com/theakshaypant/tsk/internal/core"
	"github.com/theakshaypant/tsk/internal/util"
)

// CalendarAdapter extends core.Provider with login and calendar listing.
// Both Google and Outlook adapters implement this interface.
type CalendarAdapter interface {
	core.Provider
	Login(ctx context.Context) error
	Calendars() map[string]string
}

var (
	cfgFile string
	profile string
	adapter CalendarAdapter
)

var rootCmd = &cobra.Command{
	Use:   "tsk",
	Short: "A terminal calendar client for people who'd rather not deal with calendars",
	Long: `tsk — part "task", part the sound you make when yet another meeting invite lands in your inbox.

A TUI tool that pulls events from multiple calendar providers and shows them 
in your terminal. Because sometimes you just want to see what's eating your 
day without opening a browser.`,
	PersistentPreRunE: initAdapter,
	RunE:              listEvents,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags (inherited by all subcommands)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/tsk/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "config profile to use (e.g., work, personal)")

	// Filter flags (persistent - inherited by next, calendars, etc.)
	rootCmd.PersistentFlags().IntP("days", "d", 7, "Number of days to fetch (ignored if --from/--to specified)")
	rootCmd.PersistentFlags().String("from", "", "Start date (YYYY-MM-DD, 'today', 'tomorrow', 'monday', etc.)")
	rootCmd.PersistentFlags().String("to", "", "End date (YYYY-MM-DD, 'today', 'tomorrow', 'monday', etc.)")
	rootCmd.PersistentFlags().StringP("calendars", "c", "", "Comma-separated list of calendar names to filter")
	rootCmd.PersistentFlags().Bool("ooo", true, "Include out-of-office events")
	rootCmd.PersistentFlags().Bool("focus", false, "Include focus time events")
	rootCmd.PersistentFlags().Bool("workloc", false, "Include working location events")
	rootCmd.PersistentFlags().Bool("all-types", false, "Include all event types")
	rootCmd.PersistentFlags().Bool("accepted", true, "Only show accepted events")
	rootCmd.PersistentFlags().Bool("subscribed", true, "Include subscribed calendar events")
	rootCmd.PersistentFlags().Bool("smart-ooo", false, "Hide events on days you're OOO (based on your OOO events)")
	rootCmd.PersistentFlags().String("primary-calendar", "", "Primary calendar for smart OOO detection (default: auto-detect)")
	rootCmd.PersistentFlags().Bool("no-allday", false, "Exclude all-day events")

	// Bind persistent flags to viper
	viper.BindPFlag("days", rootCmd.PersistentFlags().Lookup("days"))
	viper.BindPFlag("from", rootCmd.PersistentFlags().Lookup("from"))
	viper.BindPFlag("to", rootCmd.PersistentFlags().Lookup("to"))
	viper.BindPFlag("calendars", rootCmd.PersistentFlags().Lookup("calendars"))
	viper.BindPFlag("ooo", rootCmd.PersistentFlags().Lookup("ooo"))
	viper.BindPFlag("focus", rootCmd.PersistentFlags().Lookup("focus"))
	viper.BindPFlag("workloc", rootCmd.PersistentFlags().Lookup("workloc"))
	viper.BindPFlag("all_types", rootCmd.PersistentFlags().Lookup("all-types"))
	viper.BindPFlag("accepted", rootCmd.PersistentFlags().Lookup("accepted"))
	viper.BindPFlag("subscribed", rootCmd.PersistentFlags().Lookup("subscribed"))
	viper.BindPFlag("smart_ooo", rootCmd.PersistentFlags().Lookup("smart-ooo"))
	viper.BindPFlag("primary_calendar", rootCmd.PersistentFlags().Lookup("primary-calendar"))
	viper.BindPFlag("no_allday", rootCmd.PersistentFlags().Lookup("no-allday"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		configDir := filepath.Join(home, ".config", "tsk")
		viper.AddConfigPath(configDir)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Environment variables
	viper.SetEnvPrefix("TSK")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("credentials_file", "credentials.json")
	viper.SetDefault("token_file", "token.json")
	viper.SetDefault("days", 7)
	viper.SetDefault("ooo", true)
	viper.SetDefault("accepted", true)
	viper.SetDefault("subscribed", true)

	// Read config file if it exists
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	// Apply profile settings if specified
	applyProfile()
}

// applyProfile merges profile-specific settings over defaults
func applyProfile() {
	// Check for profile from flag or env var
	activeProfile := profile
	if activeProfile == "" {
		activeProfile = viper.GetString("default_profile")
	}
	if activeProfile == "" {
		return
	}

	// Get profile settings
	profileKey := "profiles." + activeProfile
	if !viper.IsSet(profileKey) {
		fmt.Fprintf(os.Stderr, "Warning: profile '%s' not found in config\n", activeProfile)
		return
	}

	fmt.Fprintf(os.Stderr, "Using profile: %s\n", activeProfile)

	// List of settings that can be overridden by profile
	settings := []string{
		"provider",
		"credentials_file",
		"token_file",
		"client_id",
		"tenant_id",
		"days",
		"from",
		"to",
		"calendars",
		"primary_calendar",
		"ooo",
		"focus",
		"workloc",
		"all_types",
		"accepted",
		"subscribed",
		"smart_ooo",
		"no_allday",
	}

	// Display settings that can be overridden
	displaySettings := []string{
		"display.calendar",
		"display.time",
		"display.location",
		"display.meeting_link",
		"display.description",
		"display.status",
		"display.event_url",
		"display.attachments",
		"display.id",
		"display.in_progress",
	}

	// Override each setting if present in profile,
	// but only if the user hasn't explicitly set it via CLI flag.
	for _, key := range settings {
		profileSettingKey := profileKey + "." + key
		if viper.IsSet(profileSettingKey) && !isFlagExplicitlySet(key) {
			viper.Set(key, viper.Get(profileSettingKey))
		}
	}

	// Override display settings if present in profile
	for _, key := range displaySettings {
		profileSettingKey := profileKey + "." + key
		if viper.IsSet(profileSettingKey) {
			viper.Set(key, viper.Get(profileSettingKey))
		}
	}
}

func isFlagExplicitlySet(viperKey string) bool {
	flagName := strings.ReplaceAll(viperKey, "_", "-")
	f := rootCmd.PersistentFlags().Lookup(flagName)

	return f != nil && f.Changed
}

func initAdapter(cmd *cobra.Command, args []string) error {
	// Skip adapter init for commands that don't need it
	if cmd.Name() == "help" || cmd.Name() == "completion" || cmd.Name() == "profile" ||
		cmd.Parent() != nil && cmd.Parent().Name() == "profile" {
		return nil
	}

	provider := viper.GetString("provider")
	if provider == "" {
		provider = "google"
	}

	switch provider {
	case "google":
		return initGoogleAdapter(cmd)
	case "outlook":
		return initOutlookAdapter(cmd)
	default:
		return fmt.Errorf("unknown provider: %s (supported: google, outlook)", provider)
	}
}

func initGoogleAdapter(cmd *cobra.Command) error {
	credsFile := expandPath(viper.GetString("credentials_file"))
	tokenFile := expandPath(viper.GetString("token_file"))

	// Check if files exist
	if _, err := os.Stat(credsFile); os.IsNotExist(err) {
		return fmt.Errorf("credentials file not found: %s\n\nSetup guide: https://github.com/theakshaypant/tsk/tree/main/docs/google_setup.md", credsFile)
	}

	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		return fmt.Errorf("token file not found: %s\n\nRun 'tsk auth' to authenticate", tokenFile)
	}

	adapter = google.NewGoogleAdapter(
		"google",
		"Google Calendar",
		credsFile,
		tokenFile,
	)

	if err := adapter.Login(cmd.Context()); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	return nil
}

func initOutlookAdapter(cmd *cobra.Command) error {
	clientID := viper.GetString("client_id")
	if clientID == "" {
		return fmt.Errorf("client_id not configured for Outlook provider\n\nAdd it to your profile config:\n  client_id: \"your-azure-app-client-id\"\n\nSetup guide: https://github.com/theakshaypant/tsk/tree/main/docs/outlook_setup.md")
	}

	tenantID := viper.GetString("tenant_id")
	tokenFile := expandPath(viper.GetString("token_file"))

	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		return fmt.Errorf("token file not found: %s\n\nRun 'tsk auth' to authenticate with Microsoft", tokenFile)
	}

	adapter = outlook.NewOutlookAdapter(
		"outlook",
		"Outlook Calendar",
		clientID,
		tenantID,
		tokenFile,
	)

	if err := adapter.Login(cmd.Context()); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	return nil
}

func listEvents(cmd *cobra.Command, args []string) error {
	now := time.Now()
	var start, end time.Time

	// Determine date range
	fromStr := viper.GetString("from")
	toStr := viper.GetString("to")

	if fromStr != "" || toStr != "" {
		// Use explicit date range
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
			// End of day
			end = end.Add(24*time.Hour - time.Second)
		} else {
			// Default to 7 days from start
			days := viper.GetInt("days")
			end = start.Add(time.Duration(days) * 24 * time.Hour)
		}
	} else {
		// Use days from now
		days := viper.GetInt("days")
		start = now
		end = now.Add(time.Duration(days) * 24 * time.Hour)
	}

	opts := core.FetchOptions{
		Start: start,
		End:   end,
	}

	// Calendar filter
	if calendars := viper.GetString("calendars"); calendars != "" {
		filterNames := strings.Split(calendars, ",")
		calendarIDs := resolveCalendarNames(filterNames, adapter.Calendars())
		if len(calendarIDs) == 0 {
			return fmt.Errorf("no matching calendars found for: %s\nUse 'tsk calendars' to see available calendars", calendars)
		}
		opts.CalendarIDs = calendarIDs
	}

	// Event type filter
	if viper.GetBool("all_types") {
		opts.IncludeTypes = []core.EventType{
			core.TypeDefault,
			core.TypeOutOfOffice,
			core.TypeFocusTime,
			core.TypeWorkLocation,
		}
	} else {
		opts.IncludeTypes = []core.EventType{core.TypeDefault}
		if viper.GetBool("ooo") {
			opts.IncludeTypes = append(opts.IncludeTypes, core.TypeOutOfOffice)
		}
		if viper.GetBool("focus") {
			opts.IncludeTypes = append(opts.IncludeTypes, core.TypeFocusTime)
		}
		if viper.GetBool("workloc") {
			opts.IncludeTypes = append(opts.IncludeTypes, core.TypeWorkLocation)
		}
	}

	// Status filter
	if viper.GetBool("accepted") {
		opts.IncludeStatuses = []core.EventStatus{core.StatusAccepted}
		if viper.GetBool("subscribed") {
			opts.IncludeStatuses = append(opts.IncludeStatuses, core.StatusNoResponse)
		}
	}

	// All-day events filter
	if viper.GetBool("no_allday") {
		opts.ExcludeAllDay = true
	}

	events, err := adapter.FetchEvents(cmd.Context(), opts)
	if err != nil {
		return fmt.Errorf("failed to fetch events: %w", err)
	}

	// Smart OOO filter: hide events on days you're OOO
	if viper.GetBool("smart_ooo") {
		primaryCal := detectPrimaryCalendar(viper.GetString("primary_calendar"))
		oooPeriods := getOOOPeriods(cmd.Context(), now, end, primaryCal)
		if len(oooPeriods) > 0 {
			events = filterEventsOutsideOOO(events, oooPeriods, primaryCal)
		}
	}

	fmt.Printf("📅 Events from %s to %s:\n", now.Format("Jan 2"), end.Format("Jan 2"))
	fmt.Println("─────────────────────────────────────────────────")

	if len(events) == 0 {
		fmt.Println("No upcoming events found.")
		return nil
	}

	for _, event := range events {
		printEvent(event)
	}

	fmt.Println("─────────────────────────────────────────────────")
	fmt.Printf("Total: %d events\n", len(events))

	return nil
}

// DisplayOptions controls how events are displayed
type DisplayOptions struct {
	Compact        bool   // Compact mode for list views
	ShowCalendar   bool   // Show calendar name
	ShowTime       bool   // Show when/duration
	ShowLocation   bool   // Show location
	ShowMeetLink   bool   // Show meeting link
	ShowDesc       bool   // Show description
	ShowStatus     bool   // Show response status
	ShowEventURL   bool   // Show calendar event URL
	ShowAttach     bool   // Show attachments
	ShowID         bool   // Show event ID
	ShowInProgress bool   // Show in-progress status
	Indent         string // Indentation prefix
}

// DefaultDisplayOptions returns options for list view
func DefaultDisplayOptions() DisplayOptions {
	return DisplayOptions{
		Compact:        true,
		ShowCalendar:   true,
		ShowTime:       true,
		ShowLocation:   true,
		ShowMeetLink:   true,
		ShowDesc:       true,
		ShowStatus:     true,
		ShowEventURL:   true,
		ShowAttach:     false,
		ShowID:         false,
		ShowInProgress: true,
		Indent:         "  ",
	}
}

// DetailedDisplayOptions returns options for detailed view (no in-progress since shown in header)
func DetailedDisplayOptions() DisplayOptions {
	return DisplayOptions{
		Compact:        false,
		ShowCalendar:   true,
		ShowTime:       true,
		ShowLocation:   true,
		ShowMeetLink:   true,
		ShowDesc:       true,
		ShowStatus:     true,
		ShowEventURL:   true,
		ShowAttach:     true,
		ShowID:         true,
		ShowInProgress: false,
		Indent:         "  ",
	}
}

// DisplayOptionsFromConfig builds display options from viper config
func DisplayOptionsFromConfig(detailed bool) DisplayOptions {
	opts := DefaultDisplayOptions()
	if detailed {
		opts = DetailedDisplayOptions()
	}

	// Override with config values if set
	if viper.IsSet("display.calendar") {
		opts.ShowCalendar = viper.GetBool("display.calendar")
	}
	if viper.IsSet("display.time") {
		opts.ShowTime = viper.GetBool("display.time")
	}
	if viper.IsSet("display.location") {
		opts.ShowLocation = viper.GetBool("display.location")
	}
	if viper.IsSet("display.meeting_link") {
		opts.ShowMeetLink = viper.GetBool("display.meeting_link")
	}
	if viper.IsSet("display.description") {
		opts.ShowDesc = viper.GetBool("display.description")
	}
	if viper.IsSet("display.status") {
		opts.ShowStatus = viper.GetBool("display.status")
	}
	if viper.IsSet("display.event_url") {
		opts.ShowEventURL = viper.GetBool("display.event_url")
	}
	if viper.IsSet("display.attachments") {
		opts.ShowAttach = viper.GetBool("display.attachments")
	}
	if viper.IsSet("display.id") {
		opts.ShowID = viper.GetBool("display.id")
	}
	if viper.IsSet("display.in_progress") {
		opts.ShowInProgress = viper.GetBool("display.in_progress")
	}

	return opts
}

// DisplayEvent prints an event with the given options
func DisplayEvent(event core.Event, opts DisplayOptions) {
	indent := opts.Indent

	// Title with type label (always shown)
	typeLabel := formatEventType(event.Type)
	if typeLabel != "" {
		fmt.Printf("%s[%s] %s\n", indent, typeLabel, event.Title)
	} else {
		fmt.Printf("%s%s\n", indent, event.Title)
	}

	if opts.ShowCalendar {
		if len(event.Calendars) > 1 {
			var calNames []string
			for _, cr := range event.Calendars {
				calNames = append(calNames, cr.Calendar.Name)
			}
			fmt.Printf("%s📅 Calendars:   %s\n", indent, strings.Join(calNames, ", "))
		} else {
			fmt.Printf("%s📅 Calendar:    %s\n", indent, event.Calendar.Name)
		}
	}

	if opts.ShowTime {
		fmt.Printf("%s🕐 When:        %s\n", indent, formatEventTime(event.Start, event.End, event.IsAllDay))
		fmt.Printf("%s⏱️  Duration:    %s\n", indent, formatDurationCompact(event.Duration()))
	}

	if opts.ShowLocation && event.Location != "" {
		fmt.Printf("%s📍 Location:    %s\n", indent, event.Location)
	}

	if opts.ShowMeetLink && event.MeetingLink != "" {
		linkText := util.MakeHyperlink(event.MeetingLink, event.MeetingLink)
		fmt.Printf("%s📹 Join:        %s\n", indent, linkText)
	}

	if opts.ShowDesc && event.Description != "" {
		if opts.Compact {
			fmt.Printf("%s📝 Description: %s\n", indent, truncate(util.HTMLToText(event.Description, 80), 80))
		} else {
			fmt.Printf("%s📝 Description:\n", indent)
			desc := util.HTMLToText(event.Description, 60)
			for _, line := range wrapText(desc, 60) {
				fmt.Printf("%s   %s\n", indent, line)
			}
		}
	}

	if opts.ShowStatus {
		if len(event.Calendars) > 1 {
			fmt.Printf("%s📊 Responses:\n", indent)
			for _, cr := range event.Calendars {
				fmt.Printf("%s   %s: %s\n", indent, cr.Calendar.Name, formatStatus(cr.Status))
			}
		} else {
			fmt.Printf("%s📊 Response:    %s\n", indent, formatStatus(event.Status))
		}
	}

	if opts.ShowEventURL && event.URL != "" {
		linkText := util.MakeHyperlink(event.URL, event.URL)
		fmt.Printf("%s🔗 Event:       %s\n", indent, linkText)
	}

	if opts.ShowAttach && len(event.Attachments) > 0 {
		fmt.Printf("%s📎 Attachments:\n", indent)
		for _, att := range event.Attachments {
			if att.URL != "" {
				// Use hyperlink with attachment name as display text
				linkText := util.MakeHyperlink(att.URL, att.Name)
				fmt.Printf("%s   • %s\n", indent, linkText)
			} else {
				fmt.Printf("%s   • %s\n", indent, att.Name)
			}
		}
	}

	if opts.ShowInProgress && event.InProgress(time.Now()) {
		remaining := time.Until(event.End)
		fmt.Printf("%s🟢 IN PROGRESS (%s remaining)\n", indent, formatDurationCompact(remaining))
	}

	if opts.ShowID {
		fmt.Printf("%s🆔 ID:          %s\n", indent, event.ID)
	}
}

// printEvent is a convenience wrapper for list display
func printEvent(event core.Event) {
	fmt.Println()
	DisplayEvent(event, DisplayOptionsFromConfig(false))
}

// wrapText wraps text to the given width
func wrapText(s string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(s, "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}

		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}

		line := words[0]
		for _, word := range words[1:] {
			if len(line)+1+len(word) > width {
				lines = append(lines, line)
				line = word
			} else {
				line += " " + word
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// formatDurationCompact formats a duration in a compact way
func formatDurationCompact(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("%dd %dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}

func formatEventTime(start, end time.Time, isAllDay bool) string {
	// Convert to local timezone for display
	localStart := start.Local()
	localEnd := end.Local()

	if isAllDay {
		if localStart.Day() == localEnd.Day() || end.Sub(start) <= 24*time.Hour {
			return localStart.Format("Mon, Jan 2") + " (all day)"
		}
		return fmt.Sprintf("%s - %s (all day)", localStart.Format("Mon, Jan 2"), localEnd.Add(-24*time.Hour).Format("Mon, Jan 2"))
	}

	if localStart.Day() == localEnd.Day() {
		return fmt.Sprintf("%s, %s - %s", localStart.Format("Mon, Jan 2"), localStart.Format("3:04 PM"), localEnd.Format("3:04 PM"))
	}
	return fmt.Sprintf("%s - %s", localStart.Format("Mon, Jan 2 3:04 PM"), localEnd.Format("Mon, Jan 2 3:04 PM"))
}

func formatStatus(status core.EventStatus) string {
	switch status {
	case core.StatusAccepted:
		return "Accepted ✓"
	case core.StatusRejected:
		return "Declined ✗"
	case core.StatusTentative:
		return "Tentative ?"
	case core.StatusAwaiting:
		return "Awaiting response"
	case core.StatusNoResponse:
		return "No response needed"
	default:
		return "Unknown"
	}
}

func formatEventType(t core.EventType) string {
	switch t {
	case core.TypeOutOfOffice:
		return "🏖️ OOO"
	case core.TypeFocusTime:
		return "🎯 Focus"
	case core.TypeWorkLocation:
		return "🏠 Location"
	default:
		return ""
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// parseDate parses a date string in various formats
// Supports: YYYY-MM-DD, "today", "tomorrow", "yesterday", weekday names
func parseDate(s string, defaultTime time.Time) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch s {
	case "today":
		return today, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	}

	// Check for weekday names (e.g., "monday", "next tuesday")
	weekdays := map[string]time.Weekday{
		"sunday": time.Sunday, "sun": time.Sunday,
		"monday": time.Monday, "mon": time.Monday,
		"tuesday": time.Tuesday, "tue": time.Tuesday,
		"wednesday": time.Wednesday, "wed": time.Wednesday,
		"thursday": time.Thursday, "thu": time.Thursday,
		"friday": time.Friday, "fri": time.Friday,
		"saturday": time.Saturday, "sat": time.Saturday,
	}

	// Handle "next <weekday>"
	dayName := strings.TrimPrefix(s, "next ")
	if wd, ok := weekdays[dayName]; ok {
		daysUntil := int(wd - today.Weekday())
		if daysUntil <= 0 {
			daysUntil += 7
		}
		return today.AddDate(0, 0, daysUntil), nil
	}

	// Try parsing as YYYY-MM-DD
	if t, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return t, nil
	}

	// Try parsing as MM-DD (current year)
	if t, err := time.ParseInLocation("01-02", s, now.Location()); err == nil {
		t = t.AddDate(now.Year(), 0, 0)
		return t, nil
	}

	// Try parsing as MM/DD
	if t, err := time.ParseInLocation("01/02", s, now.Location()); err == nil {
		t = t.AddDate(now.Year(), 0, 0)
		return t, nil
	}

	// Try parsing as MM/DD/YYYY
	if t, err := time.ParseInLocation("01/02/2006", s, now.Location()); err == nil {
		return t, nil
	}

	return defaultTime, fmt.Errorf("unable to parse date: %s (use YYYY-MM-DD, 'today', 'tomorrow', or weekday names)", s)
}

// expandPath expands ~ to the user's home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func resolveCalendarNames(names []string, calendars map[string]string) []string {
	var ids []string

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		nameLower := strings.ToLower(name)

		if _, exists := calendars[name]; exists {
			ids = append(ids, name)
			continue
		}

		for id, calName := range calendars {
			if strings.Contains(strings.ToLower(calName), nameLower) {
				ids = append(ids, id)
				break
			}
		}
	}

	return ids
}

// OOOPeriod represents a time range when the user is out of office
type OOOPeriod struct {
	Start time.Time
	End   time.Time
}

// detectPrimaryCalendar finds the user's primary calendar
// Uses the provided value, or auto-detects based on calendar ID
func detectPrimaryCalendar(configured string) string {
	if configured != "" {
		// Try to resolve the name to an ID
		ids := resolveCalendarNames([]string{configured}, adapter.Calendars())
		if len(ids) > 0 {
			return ids[0]
		}
		return configured
	}

	calendars := adapter.Calendars()

	// Auto-detect: look for "primary" first (Google uses this key)
	if _, exists := calendars["primary"]; exists {
		return "primary"
	}

	// Look for a calendar named "Calendar" (Outlook default)
	for id, name := range calendars {
		if name == "Calendar" {
			return id
		}
	}

	// Then look for a personal email calendar (not group calendars)
	// Google group calendars have IDs like "c_xxx@group.calendar.google.com"
	for id := range calendars {
		if strings.Contains(id, "@") && !strings.Contains(id, "@group.calendar.google.com") {
			return id
		}
	}

	// Fall back to first calendar
	for id := range calendars {
		return id
	}

	return ""
}

// getOOOPeriods fetches OOO events from the primary calendar and returns time ranges
func getOOOPeriods(ctx context.Context, start, end time.Time, primaryCalendar string) []OOOPeriod {
	if primaryCalendar == "" {
		return nil
	}

	// Fetch only OOO events from primary calendar
	// Include all statuses since OOO events might be NoResponse
	opts := core.FetchOptions{
		Start:       start,
		End:         end,
		CalendarIDs: []string{primaryCalendar},
		IncludeTypes: []core.EventType{
			core.TypeOutOfOffice,
		},
		IncludeStatuses: []core.EventStatus{
			core.StatusAccepted,
			core.StatusNoResponse,
			core.StatusTentative,
			core.StatusAwaiting,
		},
	}

	events, err := adapter.FetchEvents(ctx, opts)
	if err != nil {
		return nil
	}

	var periods []OOOPeriod
	for _, e := range events {
		periods = append(periods, OOOPeriod{
			Start: e.Start,
			End:   e.End,
		})
	}

	return periods
}

// filterEventsOutsideOOO removes events that occur during OOO periods
// Only events from the primary calendar are shown during OOO days
func filterEventsOutsideOOO(events []core.Event, oooPeriods []OOOPeriod, primaryCalendar string) []core.Event {
	var filtered []core.Event

	for _, event := range events {
		// Check if event falls on an OOO day
		if isEventOnOOODay(event, oooPeriods) {
			// On OOO days, only show the OOO event itself from primary calendar
			if event.Calendar.ID == primaryCalendar && event.Type == core.TypeOutOfOffice {
				filtered = append(filtered, event)
			}
			// Skip ALL other events on OOO days (including other primary calendar events)
			continue
		}

		// Not an OOO day - show the event
		filtered = append(filtered, event)
	}

	return filtered
}

// isEventOnOOODay checks if an event overlaps with any OOO day
func isEventOnOOODay(event core.Event, oooPeriods []OOOPeriod) bool {
	for _, ooo := range oooPeriods {
		// Check each day of the OOO period
		current := ooo.Start.Local()
		end := ooo.End.Local()

		for current.Before(end) {
			oooYear, oooMonth, oooDay := current.Date()

			// Check if event spans this OOO day
			// An event spans a day if it starts before the day ends AND ends after the day starts
			dayStart := time.Date(oooYear, oooMonth, oooDay, 0, 0, 0, 0, current.Location())
			dayEnd := dayStart.Add(24 * time.Hour)

			if event.Start.Before(dayEnd) && event.End.After(dayStart) {
				return true
			}

			// Move to next day
			current = current.Add(24 * time.Hour)
		}
	}
	return false
}
