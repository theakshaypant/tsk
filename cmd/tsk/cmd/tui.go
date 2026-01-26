package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

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
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Build fetch options from config/flags
	opts := buildFetchOptions()

	// Create the TUI model
	m := tui.NewModel(adapter, opts)

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
