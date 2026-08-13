// Package progress reports multi-step command progress on stderr.
//
// Usage:
//
//	p := progress.New(os.Stderr, progress.Options{Enabled: !jsonMode, Color: colorOn})
//	p.Start("prs")
//	p.Step("scan", "39 repos under ~/git")
//	p.Step("status", "reading local git")
//	p.Live("prs", "3/12 ryanburnette/eon")
//	p.StepDone("prs", "2 open")
//	p.End("2 open PRs")
//
// Stdout stays clean for tables/JSON/scripts. All progress goes to the writer (stderr).
package progress

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Options control progress output.
type Options struct {
	Enabled bool // false silences all output (e.g. --json)
	Color   bool
	Verbose bool // extra detail lines
	TTY     bool // allow in-place \r live counters
}

// Progress is a thread-safe step reporter.
type Progress struct {
	w    io.Writer
	opts Options

	mu       sync.Mutex
	started  time.Time
	stepName string
	liveOn   bool // true if last write was a \r live line
}

// New returns a Progress writer. If opts.Enabled is false, methods are no-ops.
func New(w io.Writer, opts Options) *Progress {
	if w == nil {
		w = io.Discard
	}
	return &Progress{w: w, opts: opts, started: time.Now()}
}

// Start begins a command run.
func (p *Progress) Start(command string) {
	if p == nil || !p.opts.Enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = time.Now()
	p.clearLive()
	fmt.Fprintf(p.w, "%s %s\n", p.paint(bold, "gitaware"), p.paint(dim, command))
}

// Step marks a named step with an optional detail suffix.
// Example: Step("scan", "39 repos")
func (p *Progress) Step(name, detail string) {
	if p == nil || !p.opts.Enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearLive()
	p.stepName = name
	line := fmt.Sprintf("  %s %-12s", p.paint(cyan, "•"), name)
	if detail != "" {
		line += " " + p.paint(dim, detail)
	}
	fmt.Fprintln(p.w, line)
}

// Stepf is Step with fmt formatting for detail.
func (p *Progress) Stepf(name, format string, args ...any) {
	p.Step(name, fmt.Sprintf(format, args...))
}

// Detail adds a secondary line under the current step (verbose or notable events).
func (p *Progress) Detail(msg string) {
	if p == nil || !p.opts.Enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearLive()
	fmt.Fprintf(p.w, "             %s\n", p.paint(dim, msg))
}

// VerboseDetail only prints when Verbose is set.
func (p *Progress) VerboseDetail(msg string) {
	if p == nil || !p.opts.Enabled || !p.opts.Verbose {
		return
	}
	p.Detail(msg)
}

// Live updates a single in-place status line (for counters). No-op when not a TTY
// so piped/CI logs stay clean (only Step/StepDone lines appear).
func (p *Progress) Live(name, detail string) {
	if p == nil || !p.opts.Enabled || !p.opts.TTY {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stepName = name
	// pad/clear so shorter lines wipe longer ones
	msg := fmt.Sprintf("  %s %-12s %s", p.paint(cyan, "•"), name, p.paint(dim, detail))
	msg = padRightVisible(msg, 90)
	fmt.Fprintf(p.w, "\r%s", msg)
	p.liveOn = true
}

// Livef is Live with formatting.
func (p *Progress) Livef(name, format string, args ...any) {
	p.Live(name, fmt.Sprintf(format, args...))
}

// StepDone finishes the current live step with a final static line.
func (p *Progress) StepDone(name, detail string) {
	if p == nil || !p.opts.Enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearLive()
	line := fmt.Sprintf("  %s %-12s", p.paint(green, "✓"), name)
	if detail != "" {
		line += " " + p.paint(dim, detail)
	}
	fmt.Fprintln(p.w, line)
	p.stepName = ""
}

// Warn reports a non-fatal problem under the current step.
func (p *Progress) Warn(msg string) {
	if p == nil || !p.opts.Enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearLive()
	fmt.Fprintf(p.w, "  %s %s\n", p.paint(yellow, "!"), msg)
}

// Fail reports a fatal step error.
func (p *Progress) Fail(msg string) {
	if p == nil || !p.opts.Enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearLive()
	fmt.Fprintf(p.w, "  %s %s\n", p.paint(red, "✗"), msg)
}

// End closes the run with a summary line and elapsed time.
func (p *Progress) End(summary string) {
	if p == nil || !p.opts.Enabled {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clearLive()
	elapsed := time.Since(p.started).Round(time.Millisecond)
	if elapsed < time.Second {
		elapsed = time.Since(p.started).Round(time.Millisecond)
	}
	msg := summary
	if msg == "" {
		msg = "done"
	}
	fmt.Fprintf(p.w, "  %s %s %s\n",
		p.paint(green, "✓"),
		msg,
		p.paint(dim, fmt.Sprintf("(%s)", elapsed)),
	)
}

func (p *Progress) clearLive() {
	if !p.liveOn {
		return
	}
	// wipe the live line
	fmt.Fprintf(p.w, "\r%s\r", strings.Repeat(" ", 90))
	p.liveOn = false
}

type colorCode int

const (
	dim colorCode = iota
	bold
	cyan
	green
	yellow
	red
)

func (p *Progress) paint(c colorCode, s string) string {
	if !p.opts.Color || s == "" {
		return s
	}
	var code string
	switch c {
	case dim:
		code = "2"
	case bold:
		code = "1"
	case cyan:
		code = "36"
	case green:
		code = "32"
	case yellow:
		code = "33"
	case red:
		code = "31"
	default:
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func padRightVisible(s string, width int) string {
	// crude: strip ANSI for length
	n := visibleLen(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func visibleLen(s string) int {
	n := 0
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}
