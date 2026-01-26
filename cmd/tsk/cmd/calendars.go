package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var calendarsCmd = &cobra.Command{
	Use:     "calendars",
	Aliases: []string{"cal", "cals"},
	Short:   "List available calendars",
	Long:    `List all calendars you have access to, including primary, shared, and subscribed calendars.`,
	RunE:    runCalendars,
}

func init() {
	rootCmd.AddCommand(calendarsCmd)
}

func runCalendars(cmd *cobra.Command, args []string) error {
	calendars := adapter.Calendars()

	fmt.Println("📅 Available calendars:")
	fmt.Println("─────────────────────────────────────────────────")

	for id, name := range calendars {
		fmt.Printf("\n  • %s\n", name)
		fmt.Printf("    ID: %s\n", id)
	}

	fmt.Println()
	fmt.Printf("Total: %d calendars\n", len(calendars))
	fmt.Println("\nTip: Use 'tsk -c \"calendar name\"' to filter events by calendar")

	return nil
}
