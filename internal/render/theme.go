package render

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Theme holds lipgloss styles for the status table.
type Theme struct {
	Enabled    bool // color
	Hyperlinks bool // OSC 8 clickable links (TTY)

	Brand    lipgloss.Style
	Path     lipgloss.Style
	Meta     lipgloss.Style
	Header   lipgloss.Style
	Border   lipgloss.Style
	Cell     lipgloss.Style
	Org      lipgloss.Style
	Repo     lipgloss.Style
	Branch   lipgloss.Style
	BranchHL lipgloss.Style

	// Count pills: foreground + background
	DirtyFG, DirtyBG   lipgloss.TerminalColor
	UntrkFG, UntrkBG   lipgloss.TerminalColor
	AheadFG, AheadBG   lipgloss.TerminalColor
	BehindFG, BehindBG lipgloss.TerminalColor
	StashFG, StashBG   lipgloss.TerminalColor

	StatusIssue lipgloss.Style
	StatusOK    lipgloss.Style
	OK          lipgloss.Style
	Warn        lipgloss.Style
	Danger      lipgloss.Style
	Summary     lipgloss.Style
	Issues      lipgloss.Style
}

// NewTheme builds styles. When color is false, output is plain.
// Hyperlinks follow color by default; use NewThemeTTY to set them independently.
func NewTheme(color bool) Theme {
	return NewThemeOpts(color, color)
}

// NewThemeOpts builds a theme with independent color and hyperlink switches.
func NewThemeOpts(color, hyperlinks bool) Theme {
	enabled := color
	if !enabled || os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
		enabled = false
	}

	muted := lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	subtle := lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"}
	text := lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F3F4F6"}
	brand := lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"}
	org := lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#93C5FD"}
	warn := lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	danger := lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	ok := lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"}

	// Soft backgrounds for count pills (work on dark + light).
	dirtyBG := lipgloss.AdaptiveColor{Light: "#FEE2E2", Dark: "#7F1D1D"}
	dirtyFG := lipgloss.AdaptiveColor{Light: "#991B1B", Dark: "#FECACA"}
	untrkBG := lipgloss.AdaptiveColor{Light: "#FAE8FF", Dark: "#701A75"}
	untrkFG := lipgloss.AdaptiveColor{Light: "#86198F", Dark: "#F5D0FE"}
	aheadBG := lipgloss.AdaptiveColor{Light: "#CFFAFE", Dark: "#155E75"}
	aheadFG := lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#A5F3FC"}
	behindBG := lipgloss.AdaptiveColor{Light: "#DBEAFE", Dark: "#1E3A8A"}
	behindFG := lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#BFDBFE"}
	stashBG := lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#374151"}
	stashFG := lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#D1D5DB"}

	t := Theme{Enabled: enabled, Hyperlinks: hyperlinks}
	t.Brand = lipgloss.NewStyle().Foreground(brand).Bold(true)
	t.Path = lipgloss.NewStyle().Foreground(text).Bold(true)
	t.Meta = lipgloss.NewStyle().Foreground(muted)
	t.Header = lipgloss.NewStyle().Foreground(muted).Bold(true)
	t.Border = lipgloss.NewStyle().Foreground(subtle)
	t.Cell = lipgloss.NewStyle().Padding(0, 1)
	t.Org = lipgloss.NewStyle().Foreground(org)
	t.Repo = lipgloss.NewStyle().Foreground(text)
	t.Branch = lipgloss.NewStyle().Foreground(muted)
	t.BranchHL = lipgloss.NewStyle().Foreground(warn).Bold(true)

	t.DirtyFG, t.DirtyBG = dirtyFG, dirtyBG
	t.UntrkFG, t.UntrkBG = untrkFG, untrkBG
	t.AheadFG, t.AheadBG = aheadFG, aheadBG
	t.BehindFG, t.BehindBG = behindFG, behindBG
	t.StashFG, t.StashBG = stashFG, stashBG

	t.StatusIssue = lipgloss.NewStyle().Foreground(danger).Bold(true)
	t.StatusOK = lipgloss.NewStyle().Foreground(ok)
	t.OK = lipgloss.NewStyle().Foreground(ok)
	t.Warn = lipgloss.NewStyle().Foreground(warn)
	t.Danger = lipgloss.NewStyle().Foreground(danger)
	t.Summary = lipgloss.NewStyle().Foreground(muted)
	t.Issues = lipgloss.NewStyle().Foreground(danger).Bold(true)
	return t
}

// pill renders a count with background block color; empty when n==0.
func (t Theme) pill(n int, fg, bg lipgloss.TerminalColor) string {
	if n <= 0 {
		return ""
	}
	s := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	if t.Enabled {
		s = s.Foreground(fg).Background(bg)
	}
	return s.Render(itoa(n))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	// small, allocation-light
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// IsTTY reports whether f is a terminal.
func IsTTY(f *os.File) bool {
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}
