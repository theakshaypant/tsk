package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage configuration profiles",
	Long: `Manage configuration profiles for different accounts and filter presets.

Profiles allow you to quickly switch between different Google accounts
and filter configurations.`,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all profiles",
	RunE:  runProfileList,
}

var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show profile settings",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runProfileShow,
}

var profileAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileAdd,
}

var profileSetDefaultCmd = &cobra.Command{
	Use:   "default <name>",
	Short: "Set the default profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runProfileSetDefault,
}

var profileEditCmd = &cobra.Command{
	Use:   "edit <name>",
	Short: "Edit a profile's settings",
	Long: `Edit a profile's settings using flags.

Example:
  tsk profile edit work --days=14 --smart-ooo=true
  tsk profile edit meetings --ooo=false --no-allday=true`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileEdit,
}

func init() {
	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileAddCmd)
	profileCmd.AddCommand(profileSetDefaultCmd)
	profileCmd.AddCommand(profileEditCmd)

	// Flags for add command - filters
	profileAddCmd.Flags().String("credentials-file", "", "Path to credentials file")
	profileAddCmd.Flags().String("token-file", "", "Path to token file")
	profileAddCmd.Flags().String("primary-calendar", "", "Primary calendar ID")
	profileAddCmd.Flags().Int("days", 7, "Number of days to fetch")
	profileAddCmd.Flags().String("calendars", "", "Calendar filter")
	profileAddCmd.Flags().Bool("ooo", true, "Include OOO events")
	profileAddCmd.Flags().Bool("focus", false, "Include focus time")
	profileAddCmd.Flags().Bool("workloc", false, "Include working location")
	profileAddCmd.Flags().Bool("all-types", false, "Include all event types")
	profileAddCmd.Flags().Bool("accepted", true, "Only accepted events")
	profileAddCmd.Flags().Bool("subscribed", true, "Include subscribed calendars")
	profileAddCmd.Flags().Bool("smart-ooo", false, "Smart OOO filtering")
	profileAddCmd.Flags().Bool("no-allday", false, "Exclude all-day events")
	// Flags for add command - display
	profileAddCmd.Flags().Bool("show-calendar", true, "Show calendar name")
	profileAddCmd.Flags().Bool("show-time", true, "Show time/duration")
	profileAddCmd.Flags().Bool("show-location", true, "Show location")
	profileAddCmd.Flags().Bool("show-meeting-link", true, "Show meeting link")
	profileAddCmd.Flags().Bool("show-description", true, "Show description")
	profileAddCmd.Flags().Bool("show-status", true, "Show response status")
	profileAddCmd.Flags().Bool("show-event-url", true, "Show event URL")
	profileAddCmd.Flags().Bool("show-attachments", false, "Show attachments")
	profileAddCmd.Flags().Bool("show-id", false, "Show event ID")
	profileAddCmd.Flags().Bool("show-in-progress", true, "Show in-progress status")

	// Same flags for edit command - filters
	profileEditCmd.Flags().String("credentials-file", "", "Path to credentials file")
	profileEditCmd.Flags().String("token-file", "", "Path to token file")
	profileEditCmd.Flags().String("primary-calendar", "", "Primary calendar ID")
	profileEditCmd.Flags().Int("days", 0, "Number of days to fetch")
	profileEditCmd.Flags().String("calendars", "", "Calendar filter")
	profileEditCmd.Flags().Bool("ooo", false, "Include OOO events")
	profileEditCmd.Flags().Bool("focus", false, "Include focus time")
	profileEditCmd.Flags().Bool("workloc", false, "Include working location")
	profileEditCmd.Flags().Bool("all-types", false, "Include all event types")
	profileEditCmd.Flags().Bool("accepted", false, "Only accepted events")
	profileEditCmd.Flags().Bool("subscribed", false, "Include subscribed calendars")
	profileEditCmd.Flags().Bool("smart-ooo", false, "Smart OOO filtering")
	profileEditCmd.Flags().Bool("no-allday", false, "Exclude all-day events")
	// Same flags for edit command - display
	profileEditCmd.Flags().Bool("show-calendar", false, "Show calendar name")
	profileEditCmd.Flags().Bool("show-time", false, "Show time/duration")
	profileEditCmd.Flags().Bool("show-location", false, "Show location")
	profileEditCmd.Flags().Bool("show-meeting-link", false, "Show meeting link")
	profileEditCmd.Flags().Bool("show-description", false, "Show description")
	profileEditCmd.Flags().Bool("show-status", false, "Show response status")
	profileEditCmd.Flags().Bool("show-event-url", false, "Show event URL")
	profileEditCmd.Flags().Bool("show-attachments", false, "Show attachments")
	profileEditCmd.Flags().Bool("show-id", false, "Show event ID")
	profileEditCmd.Flags().Bool("show-in-progress", false, "Show in-progress status")
}

func runProfileList(cmd *cobra.Command, args []string) error {
	profiles := viper.GetStringMap("profiles")
	defaultProfile := viper.GetString("default_profile")

	if len(profiles) == 0 {
		fmt.Println("No profiles configured.")
		fmt.Println("\nAdd one with: tsk profile add <name> --credentials-file=<path>")
		return nil
	}

	fmt.Println("Available profiles:")
	fmt.Println("─────────────────────────────────────────────────")

	for name := range profiles {
		marker := "  "
		if name == defaultProfile {
			marker = "* "
		}
		fmt.Printf("%s%s\n", marker, name)
	}

	fmt.Println("─────────────────────────────────────────────────")
	if defaultProfile != "" {
		fmt.Printf("Default: %s\n", defaultProfile)
	}
	fmt.Println("\nUse 'tsk profile show <name>' for details")

	return nil
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	var profileName string
	if len(args) > 0 {
		profileName = args[0]
	} else {
		profileName = viper.GetString("default_profile")
		if profileName == "" {
			return fmt.Errorf("no profile specified and no default profile set")
		}
	}

	profileKey := "profiles." + profileName
	if !viper.IsSet(profileKey) {
		return fmt.Errorf("profile '%s' not found", profileName)
	}

	settings := viper.GetStringMap(profileKey)

	fmt.Printf("Profile: %s\n", profileName)
	if profileName == viper.GetString("default_profile") {
		fmt.Println("(default)")
	}
	fmt.Println("─────────────────────────────────────────────────")

	// Display in organized sections
	fmt.Println("\n📁 Authentication:")
	printSetting(settings, "credentials_file", "credentials-file")
	printSetting(settings, "token_file", "token-file")
	printSetting(settings, "primary-calendar", "primary-calendar")

	fmt.Println("\n📅 Time Range:")
	printSetting(settings, "days", "days")
	printSetting(settings, "calendars", "calendars")

	fmt.Println("\n🏷️  Event Types:")
	printSetting(settings, "ooo", "ooo")
	printSetting(settings, "focus", "focus")
	printSetting(settings, "workloc", "workloc")
	printSetting(settings, "all-types", "all-types")

	fmt.Println("\n🔍 Filters:")
	printSetting(settings, "accepted", "accepted")
	printSetting(settings, "subscribed", "subscribed")
	printSetting(settings, "smart-ooo", "smart-ooo")
	printSetting(settings, "no-allday", "no-allday")

	// Display settings
	if display, ok := settings["display"].(map[string]interface{}); ok && len(display) > 0 {
		fmt.Println("\n👁️  Display:")
		printSetting(display, "calendar", "show_calendar")
		printSetting(display, "time", "show_time")
		printSetting(display, "location", "show_location")
		printSetting(display, "meeting_link", "show_meeting_link")
		printSetting(display, "description", "show_description")
		printSetting(display, "status", "show_status")
		printSetting(display, "event_url", "show_event_url")
		printSetting(display, "attachments", "show_attachments")
		printSetting(display, "id", "show_id")
		printSetting(display, "in_progress", "show_in_progress")
	}

	fmt.Println()
	return nil
}

func printSetting(settings map[string]interface{}, key, displayKey string) {
	if val, ok := settings[key]; ok {
		fmt.Printf("  %s: %v\n", displayKey, val)
	}
}

func runProfileAdd(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	// Check if profile already exists
	profileKey := "profiles." + profileName
	if viper.IsSet(profileKey) {
		return fmt.Errorf("profile '%s' already exists. Use 'tsk profile edit %s' to modify it", profileName, profileName)
	}

	// Build profile from flags
	profile := make(map[string]interface{})

	if val, _ := cmd.Flags().GetString("credentials-file"); val != "" {
		profile["credentials_file"] = val
	}
	if val, _ := cmd.Flags().GetString("token-file"); val != "" {
		profile["token_file"] = val
	}
	if val, _ := cmd.Flags().GetString("primary-calendar"); val != "" {
		profile["primary-calendar"] = val
	}
	if val, _ := cmd.Flags().GetInt("days"); cmd.Flags().Changed("days") {
		profile["days"] = val
	}
	if val, _ := cmd.Flags().GetString("calendars"); val != "" {
		profile["calendars"] = val
	}
	if cmd.Flags().Changed("ooo") {
		val, _ := cmd.Flags().GetBool("ooo")
		profile["ooo"] = val
	}
	if cmd.Flags().Changed("focus") {
		val, _ := cmd.Flags().GetBool("focus")
		profile["focus"] = val
	}
	if cmd.Flags().Changed("workloc") {
		val, _ := cmd.Flags().GetBool("workloc")
		profile["workloc"] = val
	}
	if cmd.Flags().Changed("all-types") {
		val, _ := cmd.Flags().GetBool("all-types")
		profile["all-types"] = val
	}
	if cmd.Flags().Changed("accepted") {
		val, _ := cmd.Flags().GetBool("accepted")
		profile["accepted"] = val
	}
	if cmd.Flags().Changed("subscribed") {
		val, _ := cmd.Flags().GetBool("subscribed")
		profile["subscribed"] = val
	}
	if cmd.Flags().Changed("smart-ooo") {
		val, _ := cmd.Flags().GetBool("smart-ooo")
		profile["smart-ooo"] = val
	}
	if cmd.Flags().Changed("no-allday") {
		val, _ := cmd.Flags().GetBool("no-allday")
		profile["no-allday"] = val
	}

	// Display settings
	display := make(map[string]interface{})
	displayChanged := false

	displayFlags := map[string]string{
		"show-calendar":     "calendar",
		"show-time":         "time",
		"show-location":     "location",
		"show-meeting-link": "meeting_link",
		"show-description":  "description",
		"show-status":       "status",
		"show-event-url":    "event_url",
		"show-attachments":  "attachments",
		"show-id":           "id",
		"show-in-progress":  "in_progress",
	}

	for flag, key := range displayFlags {
		if cmd.Flags().Changed(flag) {
			val, _ := cmd.Flags().GetBool(flag)
			display[key] = val
			displayChanged = true
		}
	}

	if displayChanged {
		profile["display"] = display
	}

	// Save to config
	if err := saveProfileToConfig(profileName, profile); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	fmt.Printf("✓ Profile '%s' created\n", profileName)
	fmt.Printf("\nUse it with: tsk -p %s\n", profileName)
	fmt.Printf("Set as default: tsk profile default %s\n", profileName)

	return nil
}

func runProfileSetDefault(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	// Check if profile exists
	profileKey := "profiles." + profileName
	if !viper.IsSet(profileKey) {
		return fmt.Errorf("profile '%s' not found", profileName)
	}

	// Update config file
	if err := setDefaultProfileInConfig(profileName); err != nil {
		return fmt.Errorf("failed to set default profile: %w", err)
	}

	fmt.Printf("✓ Default profile set to '%s'\n", profileName)
	return nil
}

func runProfileEdit(cmd *cobra.Command, args []string) error {
	profileName := args[0]

	// Check if profile exists
	profileKey := "profiles." + profileName
	if !viper.IsSet(profileKey) {
		return fmt.Errorf("profile '%s' not found. Use 'tsk profile add %s' to create it", profileName, profileName)
	}

	// Get existing profile
	existingProfile := viper.GetStringMap(profileKey)
	profile := make(map[string]interface{})
	for k, v := range existingProfile {
		profile[k] = v
	}

	// Update with changed flags
	changed := false
	if val, _ := cmd.Flags().GetString("credentials-file"); cmd.Flags().Changed("credentials-file") {
		profile["credentials_file"] = val
		changed = true
	}
	if val, _ := cmd.Flags().GetString("token-file"); cmd.Flags().Changed("token-file") {
		profile["token_file"] = val
		changed = true
	}
	if val, _ := cmd.Flags().GetString("primary-calendar"); cmd.Flags().Changed("primary-calendar") {
		profile["primary-calendar"] = val
		changed = true
	}
	if val, _ := cmd.Flags().GetInt("days"); cmd.Flags().Changed("days") {
		profile["days"] = val
		changed = true
	}
	if val, _ := cmd.Flags().GetString("calendars"); cmd.Flags().Changed("calendars") {
		profile["calendars"] = val
		changed = true
	}
	if cmd.Flags().Changed("ooo") {
		val, _ := cmd.Flags().GetBool("ooo")
		profile["ooo"] = val
		changed = true
	}
	if cmd.Flags().Changed("focus") {
		val, _ := cmd.Flags().GetBool("focus")
		profile["focus"] = val
		changed = true
	}
	if cmd.Flags().Changed("workloc") {
		val, _ := cmd.Flags().GetBool("workloc")
		profile["workloc"] = val
		changed = true
	}
	if cmd.Flags().Changed("all-types") {
		val, _ := cmd.Flags().GetBool("all-types")
		profile["all-types"] = val
		changed = true
	}
	if cmd.Flags().Changed("accepted") {
		val, _ := cmd.Flags().GetBool("accepted")
		profile["accepted"] = val
		changed = true
	}
	if cmd.Flags().Changed("subscribed") {
		val, _ := cmd.Flags().GetBool("subscribed")
		profile["subscribed"] = val
		changed = true
	}
	if cmd.Flags().Changed("smart-ooo") {
		val, _ := cmd.Flags().GetBool("smart-ooo")
		profile["smart-ooo"] = val
		changed = true
	}
	if cmd.Flags().Changed("no-allday") {
		val, _ := cmd.Flags().GetBool("no-allday")
		profile["no-allday"] = val
		changed = true
	}

	// Display settings
	displayFlags := map[string]string{
		"show-calendar":     "calendar",
		"show-time":         "time",
		"show-location":     "location",
		"show-meeting-link": "meeting_link",
		"show-description":  "description",
		"show-status":       "status",
		"show-event-url":    "event_url",
		"show-attachments":  "attachments",
		"show-id":           "id",
		"show-in-progress":  "in_progress",
	}

	// Get existing display settings or create new
	var display map[string]interface{}
	if existing, ok := profile["display"].(map[string]interface{}); ok {
		display = existing
	} else {
		display = make(map[string]interface{})
	}

	for flag, key := range displayFlags {
		if cmd.Flags().Changed(flag) {
			val, _ := cmd.Flags().GetBool(flag)
			display[key] = val
			changed = true
		}
	}

	if len(display) > 0 {
		profile["display"] = display
	}

	if !changed {
		fmt.Println("No changes specified. Use flags to update settings:")
		fmt.Println("  tsk profile edit", profileName, "--days=14 --smart-ooo=true")
		return nil
	}

	// Save to config
	if err := saveProfileToConfig(profileName, profile); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	fmt.Printf("✓ Profile '%s' updated\n", profileName)
	return nil
}

// Config file manipulation functions

func getConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "tsk", "config.yaml")
}

func readConfigFile() (map[string]interface{}, error) {
	configPath := getConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config == nil {
		config = make(map[string]interface{})
	}

	return config, nil
}

func writeConfigFile(config map[string]interface{}) error {
	configPath := getConfigPath()

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func saveProfileToConfig(name string, profile map[string]interface{}) error {
	config, err := readConfigFile()
	if err != nil {
		return err
	}

	profiles, ok := config["profiles"].(map[string]interface{})
	if !ok {
		profiles = make(map[string]interface{})
	}

	profiles[name] = profile
	config["profiles"] = profiles

	return writeConfigFile(config)
}

func setDefaultProfileInConfig(name string) error {
	config, err := readConfigFile()
	if err != nil {
		return err
	}

	config["default_profile"] = name

	return writeConfigFile(config)
}
