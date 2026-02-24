package util

import "fmt"

// MakeHyperlink creates a terminal hyperlink using OSC 8 escape sequences.
// This allows URLs to be clickable without displaying the full URL text.
// Most modern terminals support this standard (iTerm2, Konsole, GNOME Terminal, Windows Terminal, etc.)
func MakeHyperlink(url, displayText string) string {
	// OSC 8 format with BEL terminator: \033]8;;URL\007DISPLAY_TEXT\033]8;;\007
	// Using BEL (\a or \007) instead of ST (\033\\) for better compatibility
	return fmt.Sprintf("\033]8;;%s\a%s\033]8;;\a", url, displayText)
}

// TruncateText truncates s to maxLen runes, appending "…" if truncated.
func TruncateText(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}
