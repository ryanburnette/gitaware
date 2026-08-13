package render

import (
	"fmt"
	"regexp"
)

// osc8 hyperlink: ESC ] 8 ; ; URL ST  text  ESC ] 8 ; ; ST
// Supported by iTerm2, Ghostty, Kitty, Windows Terminal, VTE, etc.
const (
	osc8Start = "\033]8;;"
	osc8End   = "\033\\"
)

// Hyperlink wraps display text in an OSC 8 hyperlink when enabled.
// When disabled (piped/no-color/non-TTY), returns plain text unchanged.
// Most terminals still auto-link bare https:// URLs; OSC 8 makes the
// styled label (e.g. "PR #3") itself clickable.
func Hyperlink(enabled bool, url, text string) string {
	if !enabled || url == "" || text == "" {
		return text
	}
	// Bel or ST both work; ST is preferred.
	return osc8Start + url + osc8End + text + osc8Start + osc8End
}

// HyperlinkStyle is Hyperlink after applying a lipgloss-style string to text.
// Prefer: Hyperlink(on, url, style.Render("PR #3"))
func HyperlinkStyled(enabled bool, url, styledText string) string {
	return Hyperlink(enabled, url, styledText)
}

var unsafeURL = regexp.MustCompile(`[\x00-\x1f\x7f]`)

// SafeURL strips control chars from a URL before embedding in OSC 8.
func SafeURL(url string) string {
	return unsafeURL.ReplaceAllString(url, "")
}

// PRLink renders a clickable "PR #N" label (and optional separate URL).
// Both the label and the bare URL are linked so either is clickable.
func PRLink(th Theme, number int, url string, showURL bool) string {
	url = SafeURL(url)
	label := th.OK.Render(fmt.Sprintf("PR #%d", number))
	linked := Hyperlink(th.Hyperlinks, url, label)
	if !showURL || url == "" {
		return linked
	}
	// Bare URL: OSC 8 when supported, otherwise plain (terminals auto-detect https).
	pretty := Hyperlink(th.Hyperlinks, url, th.Meta.Render(url))
	return linked + "  " + pretty
}
