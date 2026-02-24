package util

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

var (
	// Match any HTML tag
	tagRe = regexp.MustCompile(`<[^>]*>`)

	// Match <a href="..."> to extract URLs
	anchorRe = regexp.MustCompile(`(?i)<a\s[^>]*href\s*=\s*["']([^"']*)["'][^>]*>`)

	// Match closing </a>
	anchorCloseRe = regexp.MustCompile(`(?i)</a\s*>`)

	// Collapse runs of blank lines into at most two newlines (one blank line)
	blankLinesRe = regexp.MustCompile(`\n{3,}`)

	// Collapse runs of spaces (not newlines) into one
	spacesRe = regexp.MustCompile(`[^\S\n]+`)

	// <br> variants
	brRe = regexp.MustCompile(`(?i)<br\s*/?\s*>`)

	// Closing block tags that produce paragraph breaks
	blockCloseRe = regexp.MustCompile(`(?i)</(?:p|div|h[1-6]|blockquote|pre|table|tr)\s*>`)

	// Opening block tags
	blockOpenRe = regexp.MustCompile(`(?i)<(?:p|div|h[1-6]|blockquote|pre|table|tr)(?:\s[^>]*)?\s*>`)

	// List item open/close
	liOpenRe  = regexp.MustCompile(`(?i)<li(?:\s[^>]*)?\s*>`)
	liCloseRe = regexp.MustCompile(`(?i)</li\s*>`)

	// List wrappers (strip silently — <li> handles the structure)
	listWrapRe = regexp.MustCompile(`(?i)</?(?:ul|ol)(?:\s[^>]*)?\s*>`)
)

// HTMLToText converts an HTML string to readable terminal text.
// Links become clickable OSC 8 hyperlinks, truncated to width with "…".
// Block elements, lists, and entities are converted to clean plain text.
// Pass width <= 0 to skip link truncation.
func HTMLToText(s string, width int) string {
	if s == "" {
		return s
	}

	// Normalize line endings
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	// --- Block-level elements → newlines ---
	s = brRe.ReplaceAllString(s, "\n")
	s = blockCloseRe.ReplaceAllString(s, "\n\n")
	s = blockOpenRe.ReplaceAllString(s, "\n")

	// --- Lists ---
	// Strip list wrappers first (they shouldn't add extra newlines)
	s = listWrapRe.ReplaceAllString(s, "")
	// <li> → newline + bullet, </li> → nothing
	s = liOpenRe.ReplaceAllString(s, "\n  • ")
	s = liCloseRe.ReplaceAllString(s, "")

	// --- Links → clickable terminal hyperlinks ---
	s = convertLinks(s, width)

	// --- Strip all remaining HTML tags ---
	s = tagRe.ReplaceAllString(s, "")

	// --- Decode HTML entities ---
	s = html.UnescapeString(s)

	// --- Clean up whitespace ---
	s = spacesRe.ReplaceAllString(s, " ")

	// Trim spaces from each line (but preserve bullet indentation)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Preserve "  • " indent for list items
		if strings.HasPrefix(strings.TrimLeft(line, " "), "• ") {
			lines[i] = "  • " + strings.TrimPrefix(trimmed, "• ")
		} else {
			lines[i] = trimmed
		}
	}
	s = strings.Join(lines, "\n")

	// Collapse 3+ consecutive newlines into 2 (one blank line)
	s = blankLinesRe.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}

// convertLinks replaces <a href="url">text</a> with clickable terminal
// hyperlinks (OSC 8). Google redirect URLs are unwrapped to the real target.
// Display text is truncated to maxWidth with "…" if it overflows.
func convertLinks(s string, maxWidth int) string {
	for {
		aLoc := anchorRe.FindStringSubmatchIndex(s)
		if aLoc == nil {
			break
		}

		href := s[aLoc[2]:aLoc[3]]
		afterOpen := s[aLoc[1]:]

		closeLoc := anchorCloseRe.FindStringIndex(afterOpen)
		if closeLoc == nil {
			// Malformed — strip the opening tag and move on
			s = s[:aLoc[0]] + s[aLoc[1]:]
			continue
		}

		linkText := afterOpen[:closeLoc[0]]
		// Strip any nested tags in the link text
		linkText = tagRe.ReplaceAllString(linkText, "")
		linkText = strings.TrimSpace(linkText)

		// Unwrap Google redirect URLs
		href = unwrapRedirect(href)

		// Build display text, truncate if needed
		displayText := linkText
		if displayText == "" {
			displayText = href
		}
		if maxWidth > 0 {
			displayText = TruncateText(displayText, maxWidth)
		}

		// Render as a clickable terminal hyperlink
		replacement := MakeHyperlink(href, displayText)

		s = s[:aLoc[0]] + replacement + afterOpen[closeLoc[1]:]
	}
	return s
}

// unwrapRedirect extracts the real URL from Google redirect wrappers
// like https://www.google.com/url?q=REAL_URL&...
func unwrapRedirect(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	if u.Host == "www.google.com" && u.Path == "/url" {
		if q := u.Query().Get("q"); q != "" {
			return q
		}
	}

	return rawURL
}
