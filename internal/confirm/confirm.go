package confirm

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// MutatingBanner is shown before any write operation.
const MutatingBanner = "This will modify git state on disk (not read-only)."

// Ask prints a warning and requires an explicit yes, unless yesFlag is set.
// Returns false if the user declines or input is non-interactive empty.
func Ask(stdout, stderr io.Writer, stdin io.Reader, yesFlag bool, action string, details ...string) bool {
	if yesFlag {
		return true
	}
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "WARNING: mutating operation")
	fmt.Fprintln(stderr, "  "+MutatingBanner)
	fmt.Fprintf(stderr, "  action: %s\n", action)
	for _, d := range details {
		if d != "" {
			fmt.Fprintf(stderr, "  %s\n", d)
		}
	}
	fmt.Fprintln(stderr)
	fmt.Fprint(stderr, "Type Y to continue, or pass -y to skip this prompt [y/N]: ")

	if stdin == nil {
		return false
	}
	sc := bufio.NewScanner(stdin)
	if !sc.Scan() {
		fmt.Fprintln(stderr, "aborted")
		return false
	}
	ans := strings.TrimSpace(sc.Text())
	if strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes") {
		return true
	}
	fmt.Fprintln(stderr, "aborted")
	return false
}
