package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/theakshaypant/tsk/internal/core"
	"github.com/theakshaypant/tsk/internal/util"
)

// KeyMap defines the keybindings for the TUI
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
	Open       key.Binding
	ViewEvent  key.Binding
	Refresh    key.Binding
	NextDay    key.Binding
	PrevDay    key.Binding
	Today      key.Binding
	Tab        key.Binding
	Quit       key.Binding
	Help       key.Binding
}

var DefaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "down"),
	),
	ScrollUp: key.NewBinding(
		key.WithKeys("ctrl+u", "pgup"),
		key.WithHelp("ctrl+u", "scroll up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("ctrl+d", "pgdown"),
		key.WithHelp("ctrl+d", "scroll down"),
	),
	Open: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "start meeting"),
	),
	ViewEvent: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "view event"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	NextDay: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "next day"),
	),
	PrevDay: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "prev day"),
	),
	Today: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "today"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

// Panel focus for compact mode
type PanelFocus int

const (
	FocusList PanelFocus = iota
	FocusDetail
)

// Model is the Bubble Tea model for the TUI
type Model struct {
	events        []core.Event
	selectedIdx   int
	currentDate   time.Time
	width         int
	height        int
	listWidth     int
	detailWidth   int
	contentHeight int
	keys          KeyMap
	provider      core.Provider
	fetchOptions  core.FetchOptions
	loading       bool
	err           error
	listView      viewport.Model
	detailView    viewport.Model
	viewportReady bool
	compactMode   bool       // True when terminal is too narrow for side-by-side
	focusedPanel  PanelFocus // Which panel is shown in compact mode
	showHelp      bool       // Whether the help overlay is visible
}

// NewModel creates a new TUI model
func NewModel(provider core.Provider, opts core.FetchOptions) Model {
	return Model{
		events:       []core.Event{},
		selectedIdx:  0,
		currentDate:  time.Now(),
		keys:         DefaultKeyMap,
		provider:     provider,
		fetchOptions: opts,
		loading:      true,
	}
}

// findNowEventIdx returns the index of the first upcoming event on today's view,
// or 0 for other days. This is the event right after where the NOW marker appears.
func (m *Model) findNowEventIdx() int {
	if len(m.events) == 0 {
		return 0
	}

	now := time.Now()
	isToday := m.currentDate.Year() == now.Year() &&
		m.currentDate.Month() == now.Month() &&
		m.currentDate.Day() == now.Day()

	if !isToday {
		return 0
	}

	// Find the first future timed event (same logic as NOW marker placement)
	for i, event := range m.events {
		if !event.IsAllDay && event.Start.After(now) {
			return i
		}
	}

	// All events are past or in progress — select the last one
	return len(m.events) - 1
}

// scrollToNow scrolls the list viewport so the NOW marker is visible.
// It places the NOW marker near the top of the viewport.
func (m *Model) scrollToNow() {
	if !m.viewportReady || len(m.events) == 0 {
		return
	}

	now := time.Now()
	isToday := m.currentDate.Year() == now.Year() &&
		m.currentDate.Month() == now.Month() &&
		m.currentDate.Day() == now.Day()

	if !isToday {
		m.listView.GotoTop()
		return
	}

	// Find the line position of the NOW divider
	nowDividerLine := -1
	linePos := 0
	for _, event := range m.events {
		if !event.IsAllDay && event.Start.After(now) {
			nowDividerLine = linePos
			break
		}
		linePos++ // each event is 1 line
	}

	// If no future event found, NOW is at the end
	if nowDividerLine == -1 {
		nowDividerLine = len(m.events)
	}

	// Scroll so the NOW marker is near the top (with a small offset for context)
	offset := nowDividerLine - 2
	if offset < 0 {
		offset = 0
	}
	m.listView.SetYOffset(offset)
}

// Messages
type eventsLoadedMsg struct {
	events []core.Event
	err    error
}

type tickMsg time.Time

// Commands
func (m Model) loadEvents() tea.Cmd {
	return func() tea.Msg {
		// Set date range for current day
		start := time.Date(m.currentDate.Year(), m.currentDate.Month(), m.currentDate.Day(), 0, 0, 0, 0, m.currentDate.Location())
		end := start.Add(24 * time.Hour)

		opts := m.fetchOptions
		opts.Start = start
		opts.End = end

		events, err := m.provider.FetchEvents(context.Background(), opts)
		return eventsLoadedMsg{events: events, err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Minute, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadEvents(), tickCmd())
}

// calculateLayout calculates responsive layout dimensions
func (m *Model) calculateLayout() {
	// Minimum dimensions
	minHeight := 10

	width := m.width
	height := m.height

	if height < minHeight {
		height = minHeight
	}

	// Header: ~2 lines, Help: ~2 lines, Padding: ~2 lines
	m.contentHeight = height - 6
	if m.contentHeight < 5 {
		m.contentHeight = 5
	}

	// Compact mode threshold - if too narrow for side-by-side
	compactThreshold := 70
	m.compactMode = width < compactThreshold

	if m.compactMode {
		// Single panel mode - use full width
		m.listWidth = width - 4
		m.detailWidth = width - 4
		if m.listWidth < 20 {
			m.listWidth = 20
		}
		if m.detailWidth < 20 {
			m.detailWidth = 20
		}
	} else {
		// Side-by-side mode
		// Responsive list/detail split based on width
		if width < 100 {
			// Narrow: 40% list, 60% detail
			m.listWidth = width * 40 / 100
		} else if width < 140 {
			// Medium: 35% list, 65% detail
			m.listWidth = width * 35 / 100
		} else {
			// Wide: 30% list, 70% detail (but cap list width)
			m.listWidth = width * 30 / 100
			if m.listWidth > 55 {
				m.listWidth = 55
			}
		}

		// Minimum list width
		if m.listWidth < 30 {
			m.listWidth = 30
		}

		// Detail width is remainder minus gap
		m.detailWidth = width - m.listWidth - 5
		if m.detailWidth < 35 {
			m.detailWidth = 35
		}
	}
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate layout dimensions
		m.calculateLayout()

		// Calculate viewport dimensions
		listViewportHeight := m.contentHeight - 4 // Account for borders and header
		if listViewportHeight < 1 {
			listViewportHeight = 1
		}
		listViewportWidth := m.listWidth - 4
		if listViewportWidth < 10 {
			listViewportWidth = 10
		}

		detailViewportHeight := m.contentHeight - 4 // Account for panel header and borders
		if detailViewportHeight < 1 {
			detailViewportHeight = 1
		}
		detailViewportWidth := m.detailWidth - 4 // Account for padding
		if detailViewportWidth < 10 {
			detailViewportWidth = 10
		}

		if !m.viewportReady {
			m.listView = viewport.New(listViewportWidth, listViewportHeight)
			m.listView.Style = lipgloss.NewStyle()
			m.detailView = viewport.New(detailViewportWidth, detailViewportHeight)
			m.detailView.Style = lipgloss.NewStyle()
			m.viewportReady = true
		} else {
			m.listView.Width = listViewportWidth
			m.listView.Height = listViewportHeight
			m.detailView.Width = detailViewportWidth
			m.detailView.Height = detailViewportHeight
		}
		m.updateListContent()
		m.updateDetailContent()
		return m, nil

	case eventsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.events = msg.events
			m.selectedIdx = m.findNowEventIdx()
			m.updateListContent()
			m.updateDetailContent()
			m.scrollToNow()
		}
		return m, nil

	case tickMsg:
		// Refresh the view every minute for countdown updates
		m.updateListContent()
		m.updateDetailContent()
		return m, tickCmd()

	case tea.KeyMsg:
		// When help overlay is shown, any key dismisses it
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			m.showHelp = true
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if m.selectedIdx > 0 {
				m.selectedIdx--
				m.updateListContent()
				m.scrollListToSelection()
				m.updateDetailContent()
				m.detailView.GotoTop()
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			if m.selectedIdx < len(m.events)-1 {
				m.selectedIdx++
				m.updateListContent()
				m.scrollListToSelection()
				m.updateDetailContent()
				m.detailView.GotoTop()
			}
			return m, nil

		case key.Matches(msg, m.keys.ScrollUp):
			if m.compactMode && m.focusedPanel == FocusList {
				m.listView.ViewUp()
			} else {
				m.detailView.ViewUp()
			}
			return m, nil

		case key.Matches(msg, m.keys.ScrollDown):
			if m.compactMode && m.focusedPanel == FocusList {
				m.listView.ViewDown()
			} else {
				m.detailView.ViewDown()
			}
			return m, nil

		case key.Matches(msg, m.keys.NextDay):
			m.currentDate = m.currentDate.AddDate(0, 0, 1)
			m.loading = true
			return m, m.loadEvents()

		case key.Matches(msg, m.keys.PrevDay):
			m.currentDate = m.currentDate.AddDate(0, 0, -1)
			m.loading = true
			return m, m.loadEvents()

		case key.Matches(msg, m.keys.Today):
			now := time.Now()
			isToday := m.currentDate.Year() == now.Year() &&
				m.currentDate.Month() == now.Month() &&
				m.currentDate.Day() == now.Day()

			if isToday {
				// Already on today — just scroll to now
				m.selectedIdx = m.findNowEventIdx()
				m.scrollToNow()
				return m, nil
			}
			// Switch to today (auto-scrolls to now on load)
			m.currentDate = now
			m.loading = true
			return m, m.loadEvents()

		case key.Matches(msg, m.keys.Tab):
			// Toggle between panels (works in any mode, but most useful in compact)
			if m.focusedPanel == FocusList {
				m.focusedPanel = FocusDetail
			} else {
				m.focusedPanel = FocusList
			}
			return m, nil

		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			return m, m.loadEvents()

		case key.Matches(msg, m.keys.Open):
			if len(m.events) > 0 && m.selectedIdx < len(m.events) {
				event := m.events[m.selectedIdx]
				if event.MeetingLink != "" {
					return m, openURL(event.MeetingLink)
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.ViewEvent):
			if len(m.events) > 0 && m.selectedIdx < len(m.events) {
				event := m.events[m.selectedIdx]
				if event.URL != "" {
					return m, openURL(event.URL)
				}
			}
			return m, nil
		}
	}
	return m, nil
}

// View renders the TUI
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Header
	header := m.renderHeader()

	// Main content
	var content string
	if m.loading {
		content = lipgloss.NewStyle().
			Width(m.width-4).
			Height(m.contentHeight).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Loading events...")
	} else if m.err != nil {
		content = lipgloss.NewStyle().
			Width(m.width - 4).
			Height(m.contentHeight).
			Foreground(errorColor).
			Render(fmt.Sprintf("Error: %v", m.err))
	} else if m.compactMode {
		// Single panel mode
		if m.showHelp {
			content = m.renderHelpPanel()
		} else if m.focusedPanel == FocusList {
			content = m.renderListPanel()
		} else {
			content = m.renderDetailPanel()
		}
	} else {
		// Side-by-side mode — help replaces the detail panel
		listPanel := m.renderListPanel()
		var rightPanel string
		if m.showHelp {
			rightPanel = m.renderHelpPanel()
		} else {
			rightPanel = m.renderDetailPanel()
		}
		content = lipgloss.JoinHorizontal(lipgloss.Top, listPanel, " ", rightPanel)
	}

	// Help bar
	help := m.renderHelp()

	return AppStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, header, content, help),
	)
}

func (m Model) renderHeader() string {
	dateStr := m.currentDate.Format("Monday, January 2, 2006")

	// Check if it's today
	now := time.Now()
	isToday := m.currentDate.Year() == now.Year() &&
		m.currentDate.Month() == now.Month() &&
		m.currentDate.Day() == now.Day()

	if isToday {
		dateStr = "Today • " + dateStr
	}

	title := HeaderStyle.Render("📅 tsk")
	date := lipgloss.NewStyle().Foreground(mutedColor).Render(dateStr)

	// In compact mode, show which panel is focused
	panelIndicator := ""
	if m.compactMode {
		if m.focusedPanel == FocusList {
			panelIndicator = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Render(" [Events]")
		} else {
			panelIndicator = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Render(" [Details]")
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", date, panelIndicator)
}

// updateListContent updates the list viewport with current events
func (m *Model) updateListContent() {
	if !m.viewportReady {
		return
	}

	var items []string
	if len(m.events) == 0 {
		items = append(items, NormalItemStyle.Render("No events"))
	} else {
		now := time.Now()
		isToday := m.currentDate.Year() == now.Year() &&
			m.currentDate.Month() == now.Month() &&
			m.currentDate.Day() == now.Day()

		nowLineAdded := false

		for i, event := range m.events {
			// Add "NOW" divider before the first future event (by start time)
			// For timed events: show NOW before events starting after now
			// Skip all-day events for NOW placement (they span the whole day)
			if isToday && !nowLineAdded && !event.IsAllDay && event.Start.After(now) {
				nowLine := m.renderNowDivider()
				items = append(items, nowLine)
				nowLineAdded = true
			}

			item := m.renderListItem(event, i == m.selectedIdx, m.listView.Width)
			items = append(items, item)
		}

		// If all timed events have started (or only all-day events), show NOW at the end
		if isToday && !nowLineAdded {
			// Check if there are any timed events that have ended
			hasEndedEvents := false
			for _, event := range m.events {
				if !event.IsAllDay && event.End.Before(now) {
					hasEndedEvents = true
					break
				}
			}
			if hasEndedEvents || len(m.events) > 0 {
				nowLine := m.renderNowDivider()
				items = append(items, nowLine)
			}
		}
	}

	content := strings.Join(items, "\n")
	m.listView.SetContent(content)
}

// renderNowDivider creates the "now" time indicator line
func (m Model) renderNowDivider() string {
	now := time.Now()
	timeStr := now.Format("3:04 PM")

	// Create a centered NOW indicator
	width := m.listView.Width
	nowText := fmt.Sprintf(" ▶ NOW %s ◀ ", timeStr)

	// Calculate padding for centering
	textLen := len(nowText)
	leftPad := (width - textLen) / 2
	rightPad := width - textLen - leftPad

	if leftPad < 0 {
		leftPad = 0
	}
	if rightPad < 0 {
		rightPad = 0
	}

	line := strings.Repeat("─", leftPad) + nowText + strings.Repeat("─", rightPad)

	return lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true).
		Render(line)
}

// scrollListToSelection scrolls the list viewport to keep the selected item visible
func (m *Model) scrollListToSelection() {
	if !m.viewportReady || len(m.events) == 0 {
		return
	}

	now := time.Now()
	isToday := m.currentDate.Year() == now.Year() &&
		m.currentDate.Month() == now.Month() &&
		m.currentDate.Day() == now.Day()

	// Calculate actual line position, accounting for NOW divider
	itemHeight := 1
	lineOffset := 0

	// Check if NOW divider exists and where it is
	if isToday {
		nowDividerIdx := -1
		for i, event := range m.events {
			// NOW divider appears before first future timed event
			if !event.IsAllDay && event.Start.After(now) {
				nowDividerIdx = i
				break
			}
		}
		// If no future event found, NOW is at the end
		if nowDividerIdx == -1 {
			nowDividerIdx = len(m.events)
		}
		// If selected item is at or after the NOW divider position, add 1 to offset
		if m.selectedIdx >= nowDividerIdx {
			lineOffset = 1
		}
	}

	selectedTop := (m.selectedIdx + lineOffset) * itemHeight
	selectedBottom := selectedTop + itemHeight

	viewTop := m.listView.YOffset
	viewBottom := viewTop + m.listView.Height

	// Scroll up if selected item is above viewport
	if selectedTop < viewTop {
		m.listView.SetYOffset(selectedTop)
	}
	// Scroll down if selected item is below viewport
	if selectedBottom > viewBottom {
		m.listView.SetYOffset(selectedBottom - m.listView.Height)
	}
}

func (m Model) renderListPanel() string {
	if len(m.events) == 0 {
		return ListPanelStyle.Width(m.listWidth).Height(m.contentHeight).Render(
			lipgloss.NewStyle().
				Foreground(mutedColor).
				Render("No events"),
		)
	}

	// Add scroll indicator if list is scrollable
	scrollInfo := ""
	if m.viewportReady && m.listView.TotalLineCount() > m.listView.Height {
		scrollInfo = lipgloss.NewStyle().
			Foreground(mutedColor).
			Render(fmt.Sprintf(" (%d/%d)", m.selectedIdx+1, len(m.events)))
	}

	header := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("Events") + scrollInfo

	content := m.listView.View()

	return ListPanelStyle.Width(m.listWidth).Height(m.contentHeight).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, content),
	)
}

func (m Model) renderListItem(event core.Event, selected bool, maxWidth int) string {
	now := time.Now()
	isPast := event.End.Before(now)
	isInProgress := event.InProgress(now)

	// Time - convert to local timezone for display
	localStart := event.Start.Local()
	timeStr := localStart.Format("3:04 PM")
	if event.IsAllDay {
		timeStr = "All day"
	}
	if isPast {
		timeStr = "✓ " + timeStr
	}

	var timeStyled string
	if isPast {
		timeStyled = PastTimeStyle.Render(timeStr)
	} else {
		timeStyled = TimeStyle.Render(timeStr)
	}

	// Duration
	dur := event.End.Sub(event.Start)
	durStr := formatDuration(dur)
	duration := DurationStyle.Render(durStr)

	// Calculate available width for title
	// Time (12) + Duration (6) + icons (~6) + spaces (~3)
	titleWidth := maxWidth - 27
	if titleWidth < 10 {
		titleWidth = 10
	}

	// Title (truncate if needed)
	title := event.Title
	if len(title) > titleWidth {
		title = title[:titleWidth-3] + "..."
	}

	// Status indicator (only for in-progress, past events get checkmark in time)
	statusIcon := ""
	if isInProgress {
		statusIcon = " 🟢"
	}

	// Meeting link indicator
	meetingIcon := ""
	if event.MeetingLink != "" {
		meetingIcon = " 📹"
	}

	line := fmt.Sprintf("%s %s %s%s%s", timeStyled, duration, title, meetingIcon, statusIcon)

	// Apply appropriate style based on state
	if selected {
		if isPast {
			return SelectedPastStyle.Render(line)
		}
		return SelectedItemStyle.Render(line)
	}
	if isPast {
		return PastItemStyle.Render(line)
	}
	return NormalItemStyle.Render(line)
}

// updateDetailContent updates the viewport with the current event details
func (m *Model) updateDetailContent() {
	if len(m.events) == 0 || !m.viewportReady {
		return
	}

	event := m.events[m.selectedIdx]
	width := m.detailView.Width
	var lines []string

	// Title (wrap to panel width)
	lines = append(lines, TitleStyle.Render(ansi.Wordwrap(event.Title, width, "")))
	lines = append(lines, "")

	// Calendar(s)
	if len(event.Calendars) > 1 {
		var calNames []string
		for _, cr := range event.Calendars {
			calNames = append(calNames, cr.Calendar.Name)
		}
		lines = append(lines, renderWrappedField("📅 Calendars", strings.Join(calNames, ", "), width))
	} else if event.Calendar.Name != "" {
		lines = append(lines, renderField("📅 Calendar", event.Calendar.Name))
	}

	// Time
	timeStr := formatEventTime(event.Start, event.End, event.IsAllDay)
	lines = append(lines, renderField("🕐 When", timeStr))

	// Duration
	if !event.IsAllDay {
		dur := event.End.Sub(event.Start)
		lines = append(lines, renderField("⏱️  Duration", formatDuration(dur)))
	}

	// Status: Past / In Progress / Upcoming
	now := time.Now()
	if event.End.Before(now) {
		// Event has passed
		ago := now.Sub(event.End)
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true).
			Render(fmt.Sprintf("✓ Ended %s ago", formatDuration(ago))))
	} else if event.InProgress(now) {
		remaining := event.End.Sub(now)
		lines = append(lines, "")
		lines = append(lines, InProgressStyle.Render(fmt.Sprintf("🟢 IN PROGRESS • %s remaining", formatDuration(remaining))))
	} else if event.Start.After(now) {
		until := event.Start.Sub(now)
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().Foreground(accentColor).Render(fmt.Sprintf("⏳ Starts in %s", formatDuration(until))))
	}

	lines = append(lines, "")

	// Location
	if event.Location != "" {
		lines = append(lines, renderWrappedField("📍 Location", event.Location, width))
	}

	// Meeting link
	if event.MeetingLink != "" {
		labelWidth := lipgloss.Width(LabelStyle.Render("📹 Join")) + 1
		maxLinkLen := width - labelWidth
		displayURL := util.TruncateText(event.MeetingLink, maxLinkLen)
		styledText := LinkStyle.Render(displayURL)
		linkText := util.MakeHyperlink(event.MeetingLink, styledText)
		lines = append(lines, renderField("📹 Join", linkText))
	}

	// Response(s)
	if len(event.Calendars) > 1 {
		lines = append(lines, LabelStyle.Render("📊 Responses"))
		for _, cr := range event.Calendars {
			lines = append(lines, fmt.Sprintf("   %s: %s", ValueStyle.Render(cr.Calendar.Name), formatStatus(cr.Status)))
		}
	} else {
		lines = append(lines, renderField("📊 Response", formatStatus(event.Status)))
	}

	// Description (convert HTML → plain text, then word-wrap)
	if event.Description != "" {
		lines = append(lines, "")
		lines = append(lines, LabelStyle.Render("📝 Description"))
		desc := util.HTMLToText(event.Description, width)
		wrapped := ansi.Wordwrap(desc, width, "")
		lines = append(lines, ValueStyle.Render(wrapped))
	}

	// Attachments
	if len(event.Attachments) > 0 {
		lines = append(lines, "")
		lines = append(lines, LabelStyle.Render("📎 Attachments"))
		maxAttLen := width - 5 // "   • " prefix = 5 chars
		for _, att := range event.Attachments {
			if att.URL != "" {
				displayName := util.TruncateText(att.Name, maxAttLen)
				styledName := LinkStyle.Render(displayName)
				linkText := util.MakeHyperlink(att.URL, styledName)
				lines = append(lines, fmt.Sprintf("   • %s", linkText))
			} else {
				lines = append(lines, fmt.Sprintf("   • %s", util.TruncateText(att.Name, maxAttLen)))
			}
		}
	}

	content := strings.Join(lines, "\n")
	m.detailView.SetContent(content)
}

func (m Model) renderDetailPanel() string {
	if len(m.events) == 0 {
		return DetailPanelStyle.Width(m.detailWidth).Height(m.contentHeight).Render(
			lipgloss.NewStyle().
				Foreground(mutedColor).
				Render("No event selected"),
		)
	}

	// Add scroll indicator if content is scrollable
	scrollInfo := ""
	if m.viewportReady && m.detailView.TotalLineCount() > m.detailView.Height {
		scrollPct := int(m.detailView.ScrollPercent() * 100)
		scrollInfo = lipgloss.NewStyle().
			Foreground(mutedColor).
			Render(fmt.Sprintf(" (%d%%)", scrollPct))
	}

	header := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("Event Details") + scrollInfo

	content := m.detailView.View()

	return DetailPanelStyle.Width(m.detailWidth).Height(m.contentHeight).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, "", content),
	)
}

func (m Model) renderHelp() string {
	var keys []string

	keys = []string{
		HelpKeyStyle.Render("↑/↓") + " nav",
		HelpKeyStyle.Render("←/→") + " day",
		HelpKeyStyle.Render("tab") + " panel",
		HelpKeyStyle.Render("t") + " now",
		HelpKeyStyle.Render("enter") + " meet",
		HelpKeyStyle.Render("v") + " view",
		HelpKeyStyle.Render("r") + " refresh",
		HelpKeyStyle.Render("q") + " quit",
	}

	fullLine := strings.Join(keys, "  •  ")

	// Check if the full help bar fits in the available width
	// Calculate visual length (without ANSI escape codes)
	maxWidth := m.width - 4
	visualLen := lipgloss.Width(fullLine)

	if visualLen > maxWidth {
		// Doesn't fit — show minimal hint
		return HelpStyle.Render(HelpKeyStyle.Render("?") + " help")
	}

	return HelpStyle.Render(fullLine)
}

func (m Model) renderHelpPanel() string {
	header := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("Keyboard Shortcuts")

	lines := []string{
		"",
		HelpKeyStyle.Render("  ↑          ") + " Move up",
		HelpKeyStyle.Render("  ↓          ") + " Move down",
		HelpKeyStyle.Render("  ctrl+u/d   ") + " Scroll detail panel",
		HelpKeyStyle.Render("  →          ") + " Next day",
		HelpKeyStyle.Render("  ←          ") + " Previous day",
		HelpKeyStyle.Render("  t          ") + " Jump to now / today",
		HelpKeyStyle.Render("  tab        ") + " Switch panel",
		HelpKeyStyle.Render("  enter      ") + " Start meeting / open event",
		HelpKeyStyle.Render("  v          ") + " View event in calendar",
		HelpKeyStyle.Render("  r          ") + " Refresh events",
		HelpKeyStyle.Render("  q / ctrl+c ") + " Quit",
		"",
		lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("  Press any key to close"),
	}

	body := strings.Join(lines, "\n")

	// Use the same dimensions as the detail panel
	panelWidth := m.detailWidth
	if m.compactMode {
		panelWidth = m.listWidth
	}

	return DetailPanelStyle.Width(panelWidth).Height(m.contentHeight).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, body),
	)
}

// Helper functions
func renderField(label, value string) string {
	return LabelStyle.Render(label) + " " + ValueStyle.Render(value)
}

// renderWrappedField renders a label-value field, word-wrapping the value
// to fit within maxWidth. Continuation lines are indented to align with the value.
func renderWrappedField(label, value string, maxWidth int) string {
	labelRendered := LabelStyle.Render(label)
	labelWidth := lipgloss.Width(labelRendered) + 1 // +1 for the space
	valueWidth := maxWidth - labelWidth
	if valueWidth < 10 {
		valueWidth = 10
	}
	wrapped := ansi.Wordwrap(value, valueWidth, "")
	wrapLines := strings.Split(wrapped, "\n")
	indent := strings.Repeat(" ", labelWidth)
	for i := 1; i < len(wrapLines); i++ {
		wrapLines[i] = indent + wrapLines[i]
	}
	return labelRendered + " " + ValueStyle.Render(strings.Join(wrapLines, "\n"))
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	days := int(d.Hours()) / 24
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
		return localStart.Format("Mon, Jan 2") + " (all day)"
	}
	if localStart.Day() == localEnd.Day() {
		return fmt.Sprintf("%s, %s - %s",
			localStart.Format("Mon, Jan 2"),
			localStart.Format("3:04 PM"),
			localEnd.Format("3:04 PM"))
	}
	return fmt.Sprintf("%s - %s",
		localStart.Format("Mon, Jan 2 3:04 PM"),
		localEnd.Format("Mon, Jan 2 3:04 PM"))
}

func formatStatus(status core.EventStatus) string {
	switch status {
	case core.StatusAccepted:
		return StatusAcceptedStyle.Render("Accepted ✓")
	case core.StatusRejected:
		return StatusDeclinedStyle.Render("Declined ✗")
	case core.StatusTentative:
		return StatusPendingStyle.Render("Tentative ?")
	case core.StatusAwaiting:
		return StatusPendingStyle.Render("Awaiting response")
	case core.StatusNoResponse:
		return lipgloss.NewStyle().Foreground(mutedColor).Render("No response needed")
	default:
		return "Unknown"
	}
}

// openURL opens a URL in the default browser
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			return nil
		}
		_ = cmd.Start()
		return nil
	}
}
