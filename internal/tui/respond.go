package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/theakshaypant/tsk/internal/core"
)

// RespondModal is a form for responding to calendar events
type RespondModal struct {
	event               core.Event
	calendarID          string
	selectedCalendarIdx int // Index into event.Calendars for multi-calendar events
	width               int
	height              int
	focusIndex          int
	responseType        core.ResponseType
	messageInput        textinput.Model
	proposalInput       textinput.Model
	recurringScope      core.RecurringScope
	submitted           bool
	cancelled           bool
}

// NewRespondModal creates a new respond modal for the given event
func NewRespondModal(event core.Event, calendarID string) RespondModal {
	messageInput := textinput.New()
	messageInput.Placeholder = "Optional message to organizer..."
	messageInput.CharLimit = 500
	messageInput.Width = 50

	proposalInput := textinput.New()
	proposalInput.Placeholder = "14:00/15:00 or 2026-03-04T14:00/15:00"
	proposalInput.CharLimit = 100
	proposalInput.Width = 50

	// Find which calendar index to use for multi-calendar events
	selectedCalendarIdx := 0
	if len(event.Calendars) > 1 {
		// Find the calendar that matches the provided calendarID
		for i, cal := range event.Calendars {
			if cal.Calendar.ID == calendarID {
				selectedCalendarIdx = i
				break
			}
		}
	}

	// Pre-populate response type based on current status
	// For multi-calendar events, use the status from the selected calendar
	responseType := core.ResponseAccept
	status := event.Status
	if len(event.Calendars) > 0 && selectedCalendarIdx < len(event.Calendars) {
		status = event.Calendars[selectedCalendarIdx].Status
	}

	switch status {
	case core.StatusAccepted:
		responseType = core.ResponseAccept
	case core.StatusRejected:
		responseType = core.ResponseDecline
	case core.StatusTentative:
		responseType = core.ResponseTentative
	case core.StatusAwaiting:
		responseType = core.ResponseAccept // Default for pending invitations
	}

	// Pre-populate proposed time if it exists
	if proposal := event.GetProposedTime(); proposal != nil {
		proposalStr := proposal.Start.Format("15:04") + "/" + proposal.End.Format("15:04")
		proposalInput.SetValue(proposalStr)
	}

	return RespondModal{
		event:               event,
		calendarID:          calendarID,
		selectedCalendarIdx: selectedCalendarIdx,
		responseType:        responseType,
		messageInput:        messageInput,
		proposalInput:       proposalInput,
		recurringScope:      core.RecurringScopeThisInstance,
	}
}

// Init initializes the modal
func (m RespondModal) Init() tea.Cmd {
	return textinput.Blink
}

// Helper methods to get dynamic focus indices
func (m RespondModal) calendarFocusIdx() int {
	if len(m.event.Calendars) > 1 {
		return 1
	}
	return -1 // Not shown
}

func (m RespondModal) recurringScopeFocusIdx() int {
	offset := 1
	if len(m.event.Calendars) > 1 {
		offset++
	}
	if m.event.IsRecurring() {
		return offset
	}
	return -1 // Not shown
}

func (m RespondModal) messageFocusIdx() int {
	offset := 1
	if len(m.event.Calendars) > 1 {
		offset++
	}
	if m.event.IsRecurring() {
		offset++
	}
	return offset
}

func (m RespondModal) proposalFocusIdx() int {
	return m.messageFocusIdx() + 1
}

func (m RespondModal) maxFocusIdx() int {
	return m.proposalFocusIdx()
}

// Update handles messages for the respond modal
func (m RespondModal) Update(msg tea.Msg) (RespondModal, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.cancelled = true
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c"))):
			m.cancelled = true
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			// Submit the form
			// Since text inputs are single-line, Enter should submit
			m.submitted = true
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("tab", "down"))):
			m.focusIndex++
			if m.focusIndex > m.maxFocusIdx() {
				m.focusIndex = 0
			}
			m.updateFocus()
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab", "up"))):
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = m.maxFocusIdx()
			}
			m.updateFocus()
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("1"))):
			if m.focusIndex == 0 {
				m.responseType = core.ResponseAccept
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("2"))):
			if m.focusIndex == 0 {
				m.responseType = core.ResponseDecline
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("3"))):
			if m.focusIndex == 0 {
				m.responseType = core.ResponseTentative
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("y", "n"))):
			// Handle recurring scope toggle
			if m.event.IsRecurring() && m.focusIndex == m.recurringScopeFocusIdx() {
				if msg.String() == "y" {
					m.recurringScope = core.RecurringScopeAllInstances
				} else {
					m.recurringScope = core.RecurringScopeThisInstance
				}
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "right"))):
			// Handle calendar selection (only when on calendar picker)
			if len(m.event.Calendars) > 1 && m.focusIndex == m.calendarFocusIdx() {
				if msg.String() == "right" {
					m.selectedCalendarIdx++
					if m.selectedCalendarIdx >= len(m.event.Calendars) {
						m.selectedCalendarIdx = 0
					}
				} else {
					m.selectedCalendarIdx--
					if m.selectedCalendarIdx < 0 {
						m.selectedCalendarIdx = len(m.event.Calendars) - 1
					}
				}
				// Update calendar ID and response type based on selected calendar
				m.calendarID = m.event.Calendars[m.selectedCalendarIdx].Calendar.ID
				// Update pre-selected response based on this calendar's status
				switch m.event.Calendars[m.selectedCalendarIdx].Status {
				case core.StatusAccepted:
					m.responseType = core.ResponseAccept
				case core.StatusRejected:
					m.responseType = core.ResponseDecline
				case core.StatusTentative:
					m.responseType = core.ResponseTentative
				}
				return m, nil
			}
		}
	}

	// Update text inputs if they're focused
	if m.focusIndex == m.messageFocusIdx() {
		m.messageInput, cmd = m.messageInput.Update(msg)
		return m, cmd
	} else if m.focusIndex == m.proposalFocusIdx() {
		m.proposalInput, cmd = m.proposalInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

// updateFocus updates which field is focused
func (m *RespondModal) updateFocus() {
	m.messageInput.Blur()
	m.proposalInput.Blur()

	if m.focusIndex == m.messageFocusIdx() {
		m.messageInput.Focus()
	} else if m.focusIndex == m.proposalFocusIdx() {
		m.proposalInput.Focus()
	}
}

// View renders the respond modal
func (m RespondModal) View() string {
	if m.width == 0 {
		return ""
	}

	// Modal dimensions
	modalWidth := 70
	if m.width < 80 {
		modalWidth = m.width - 10
	}

	// Adjust input widths to fit modal
	inputWidth := modalWidth - 6
	if inputWidth > 60 {
		inputWidth = 60
	}
	m.messageInput.Width = inputWidth
	m.proposalInput.Width = inputWidth

	var content strings.Builder

	// Header
	content.WriteString(lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("📅 Respond to Event"))
	content.WriteString("\n\n")

	// Event title
	eventTitle := m.event.Title
	if len(eventTitle) > modalWidth-4 {
		eventTitle = eventTitle[:modalWidth-7] + "..."
	}
	content.WriteString(lipgloss.NewStyle().
		Foreground(accentColor).
		Render(eventTitle))
	content.WriteString("\n")

	// Show current response status if already responded
	if m.event.Status != core.StatusAwaiting {
		currentStatus := m.formatCurrentStatus()
		content.WriteString(lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true).
			Render("Current: " + currentStatus))
	}
	content.WriteString("\n\n")

	// Section 1: Response type
	content.WriteString(m.renderResponseTypeSection())
	content.WriteString("\n")

	// Section 2: Calendar picker (only if multi-calendar event)
	if len(m.event.Calendars) > 1 {
		content.WriteString(m.renderCalendarSection())
		content.WriteString("\n")
	}

	// Section 3: Recurring scope (only if event is recurring)
	if m.event.IsRecurring() {
		content.WriteString(m.renderRecurringScopeSection())
		content.WriteString("\n")
	}

	// Section 4: Message
	content.WriteString(m.renderMessageSection())
	content.WriteString("\n")

	// Section 5: Proposed time
	content.WriteString(m.renderProposalSection())
	content.WriteString("\n\n")

	// Submit/Cancel
	content.WriteString(m.renderActions())

	// Wrap in modal box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Width(modalWidth)

	modal := boxStyle.Render(content.String())

	// Center in screen
	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)
}

func (m RespondModal) renderResponseTypeSection() string {
	focusIdx := 0
	isFocused := m.focusIndex == focusIdx

	// Label changes based on whether this is new or changing response
	headerLabel := "Response Type"
	if m.event.Status != core.StatusAwaiting {
		headerLabel = "Change Response To"
	}

	var options strings.Builder
	options.WriteString(m.renderSectionHeader(headerLabel, isFocused))
	options.WriteString("\n")

	// Option 1: Accept
	marker := " "
	if m.responseType == core.ResponseAccept {
		marker = "●"
	}
	acceptStyle := lipgloss.NewStyle()
	if isFocused && m.responseType == core.ResponseAccept {
		acceptStyle = acceptStyle.Foreground(accentColor).Bold(true)
	} else if m.responseType == core.ResponseAccept {
		acceptStyle = acceptStyle.Foreground(primaryColor)
	}
	options.WriteString(acceptStyle.Render(fmt.Sprintf("  %s 1. Accept", marker)))
	options.WriteString("\n")

	// Option 2: Decline
	marker = " "
	if m.responseType == core.ResponseDecline {
		marker = "●"
	}
	declineStyle := lipgloss.NewStyle()
	if isFocused && m.responseType == core.ResponseDecline {
		declineStyle = declineStyle.Foreground(accentColor).Bold(true)
	} else if m.responseType == core.ResponseDecline {
		declineStyle = declineStyle.Foreground(primaryColor)
	}
	options.WriteString(declineStyle.Render(fmt.Sprintf("  %s 2. Decline", marker)))
	options.WriteString("\n")

	// Option 3: Tentative
	marker = " "
	if m.responseType == core.ResponseTentative {
		marker = "●"
	}
	tentativeStyle := lipgloss.NewStyle()
	if isFocused && m.responseType == core.ResponseTentative {
		tentativeStyle = tentativeStyle.Foreground(accentColor).Bold(true)
	} else if m.responseType == core.ResponseTentative {
		tentativeStyle = tentativeStyle.Foreground(primaryColor)
	}
	options.WriteString(tentativeStyle.Render(fmt.Sprintf("  %s 3. Tentative", marker)))

	return options.String()
}

func (m RespondModal) renderCalendarSection() string {
	focusIdx := m.calendarFocusIdx()
	isFocused := m.focusIndex == focusIdx

	var section strings.Builder
	section.WriteString(m.renderSectionHeader("Respond From Calendar", isFocused))
	section.WriteString("\n")

	// Show current calendar selection with navigation hint
	if m.selectedCalendarIdx < len(m.event.Calendars) {
		currentCal := m.event.Calendars[m.selectedCalendarIdx]

		// Build display string with calendar name and current status
		calName := currentCal.Calendar.Name
		statusStr := ""
		switch currentCal.Status {
		case core.StatusAccepted:
			statusStr = " (Currently: Accepted ✓)"
		case core.StatusRejected:
			statusStr = " (Currently: Declined ✗)"
		case core.StatusTentative:
			statusStr = " (Currently: Tentative ?)"
		case core.StatusAwaiting:
			statusStr = " (Awaiting response)"
		}

		displayText := fmt.Sprintf("  %s%s", calName, statusStr)

		// Style based on focus
		style := lipgloss.NewStyle()
		if isFocused {
			style = style.Foreground(accentColor).Bold(true)
		} else {
			style = style.Foreground(primaryColor)
		}

		section.WriteString(style.Render(displayText))
		section.WriteString("\n")

		// Show navigation hint when focused
		if isFocused && len(m.event.Calendars) > 1 {
			hint := fmt.Sprintf("  ← → to switch (%d/%d)", m.selectedCalendarIdx+1, len(m.event.Calendars))
			section.WriteString(lipgloss.NewStyle().
				Foreground(mutedColor).
				Render(hint))
		}
	}

	return section.String()
}

func (m RespondModal) renderRecurringScopeSection() string {
	focusIdx := m.recurringScopeFocusIdx()
	isFocused := m.focusIndex == focusIdx

	var options strings.Builder
	options.WriteString(m.renderSectionHeader("Apply To", isFocused))
	options.WriteString("\n")

	// Option 1: This instance only
	marker := " "
	if m.recurringScope == core.RecurringScopeThisInstance {
		marker = "●"
	}
	thisStyle := lipgloss.NewStyle()
	if isFocused && m.recurringScope == core.RecurringScopeThisInstance {
		thisStyle = thisStyle.Foreground(accentColor).Bold(true)
	} else if m.recurringScope == core.RecurringScopeThisInstance {
		thisStyle = thisStyle.Foreground(primaryColor)
	}
	options.WriteString(thisStyle.Render(fmt.Sprintf("  %s This event only (n)", marker)))
	options.WriteString("\n")

	// Option 2: All instances
	marker = " "
	if m.recurringScope == core.RecurringScopeAllInstances {
		marker = "●"
	}
	allStyle := lipgloss.NewStyle()
	if isFocused && m.recurringScope == core.RecurringScopeAllInstances {
		allStyle = allStyle.Foreground(accentColor).Bold(true)
	} else if m.recurringScope == core.RecurringScopeAllInstances {
		allStyle = allStyle.Foreground(primaryColor)
	}
	options.WriteString(allStyle.Render(fmt.Sprintf("  %s All events in series (y)", marker)))

	return options.String()
}

func (m RespondModal) renderMessageSection() string {
	focusIdx := m.messageFocusIdx()
	isFocused := m.focusIndex == focusIdx

	var section strings.Builder
	section.WriteString(m.renderSectionHeader("Message (optional)", isFocused))
	section.WriteString("\n")
	section.WriteString(m.messageInput.View())

	return section.String()
}

func (m RespondModal) renderProposalSection() string {
	focusIdx := m.proposalFocusIdx()
	isFocused := m.focusIndex == focusIdx

	var section strings.Builder
	section.WriteString(m.renderSectionHeader("Propose New Time (optional)", isFocused))
	section.WriteString("\n")
	section.WriteString(m.proposalInput.View())
	section.WriteString("\n")

	// Show validation feedback if user has entered something
	proposalValue := strings.TrimSpace(m.proposalInput.Value())
	if proposalValue != "" {
		validationMsg := m.validateProposalFormat(proposalValue)
		section.WriteString(validationMsg)
		section.WriteString("\n")
	}

	// Format hints
	hints := lipgloss.NewStyle().
		Foreground(mutedColor).
		Render("  Examples: 14:00/15:00  |  2026-03-04T14:00/15:00  |  14:00Z/15:00Z")
	section.WriteString(hints)

	return section.String()
}

// validateProposalFormat checks if the proposal has the right format
func (m RespondModal) validateProposalFormat(proposal string) string {
	// Quick validation - just check basic format
	if !strings.Contains(proposal, "/") {
		return lipgloss.NewStyle().
			Foreground(errorColor).
			Render("  ⚠ Missing '/' separator")
	}

	parts := strings.SplitN(proposal, "/", 2)
	if len(parts) != 2 {
		return lipgloss.NewStyle().
			Foreground(errorColor).
			Render("  ⚠ Invalid format")
	}

	// Check if both parts are non-empty
	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])
	if start == "" || end == "" {
		return lipgloss.NewStyle().
			Foreground(errorColor).
			Render("  ⚠ Both start and end times required")
	}

	// Check if they look like times (basic pattern matching)
	if !looksLikeTime(start) {
		return lipgloss.NewStyle().
			Foreground(errorColor).
			Render("  ⚠ Start time doesn't look valid")
	}

	if !looksLikeTime(end) {
		return lipgloss.NewStyle().
			Foreground(errorColor).
			Render("  ⚠ End time doesn't look valid")
	}

	// Basic format looks okay
	return lipgloss.NewStyle().
		Foreground(primaryColor).
		Render("  ✓ Format looks good")
}

// looksLikeTime checks if a string looks like a time value
// Supports all formats: HH:MM, HH:MM:SS, YYYY-MM-DDTHH:MM, YYYY-MM-DDTHH:MM:SS, RFC3339
func looksLikeTime(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}

	// Must contain ":" for time portion
	if !strings.Contains(s, ":") {
		return false
	}

	// Try parsing with the actual time parsers to validate properly
	// This ensures we validate against the same formats we accept
	_, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return true
	}

	_, err = time.ParseInLocation("2006-01-02T15:04:05", s, time.Local)
	if err == nil {
		return true
	}

	_, err = time.ParseInLocation("2006-01-02T15:04", s, time.Local)
	if err == nil {
		return true
	}

	_, err = time.ParseInLocation("15:04:05", s, time.Local)
	if err == nil {
		return true
	}

	_, err = time.ParseInLocation("15:04", s, time.Local)
	if err == nil {
		return true
	}

	// No valid format matched
	return false
}

func (m RespondModal) renderActions() string {
	submitStyle := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	cancelStyle := lipgloss.NewStyle().
		Foreground(mutedColor)

	submit := submitStyle.Render("[Enter] Submit")
	cancel := cancelStyle.Render("[Esc] Cancel")

	return fmt.Sprintf("  %s    %s", submit, cancel)
}

func (m RespondModal) renderSectionHeader(title string, focused bool) string {
	style := lipgloss.NewStyle().Bold(true)
	if focused {
		style = style.Foreground(accentColor)
		title = "▶ " + title
	} else {
		style = style.Foreground(primaryColor)
		title = "  " + title
	}
	return style.Render(title)
}

// formatCurrentStatus returns a human-readable current response status
func (m RespondModal) formatCurrentStatus() string {
	switch m.event.Status {
	case core.StatusAccepted:
		return "Accepted ✓"
	case core.StatusRejected:
		return "Declined ✗"
	case core.StatusTentative:
		return "Tentative ?"
	case core.StatusAwaiting:
		return "Awaiting response"
	default:
		return ""
	}
}

// GetResponse returns the response options if submitted
func (m RespondModal) GetResponse() (core.RespondOptions, bool) {
	if !m.submitted {
		return core.RespondOptions{}, false
	}

	opts := core.RespondOptions{
		Response:       m.responseType,
		Comment:        strings.TrimSpace(m.messageInput.Value()),
		RecurringScope: m.recurringScope,
	}

	return opts, true
}

// Cancelled returns true if the modal was cancelled
func (m RespondModal) Cancelled() bool {
	return m.cancelled
}

// GetProposalString returns the raw proposal input string
func (m RespondModal) GetProposalString() string {
	return strings.TrimSpace(m.proposalInput.Value())
}
