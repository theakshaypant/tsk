package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/theakshaypant/tsk/internal/core"
	"github.com/theakshaypant/tsk/internal/tui"
)

var tuiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch the interactive TUI",
	Long:  `Launch an interactive terminal user interface for browsing calendar events.`,
	RunE:  runTUI,
}

func init() {
	tuiCmd.Flags().String("split", "side", "Panel split direction: side (side-by-side) or stack (top/bottom)")
	tuiCmd.Flags().Int("list-percent", 0, "List panel size as percentage (10-90, 0 = auto)")
	viper.BindPFlag("ui.split", tuiCmd.Flags().Lookup("split"))
	viper.BindPFlag("ui.list_percent", tuiCmd.Flags().Lookup("list-percent"))
	rootCmd.AddCommand(tuiCmd)
}

func parseUIOptions() tui.UIOptions {
	split := strings.ToLower(viper.GetString("ui.split"))
	var dir tui.SplitDirection
	switch split {
	case "stack":
		dir = tui.SplitStack
	default:
		dir = tui.SplitSide
	}

	return tui.UIOptions{
		Split:       dir,
		ListPercent: viper.GetInt("ui.list_percent"),
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Build fetch options from config/flags
	opts := buildFetchOptions()

	// Create the TUI model
	uiOpts := parseUIOptions()
	m := tui.NewModel(adapter, opts, uiOpts)

	// Set up the program with mouse support and alt screen
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Run the TUI
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

func buildFetchOptions() core.FetchOptions {
	opts := core.FetchOptions{}

	// Calendar filter
	if calendars := viper.GetString("calendars"); calendars != "" {
		filterNames := strings.Split(calendars, ",")
		calendarIDs := resolveCalendarNames(filterNames, adapter.Calendars())
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

	return opts
}

// OpenBrowser opens a URL in the default browser
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
