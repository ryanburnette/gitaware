package gitlocal

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// Status is parsed output of git status --porcelain=v2 --branch.
type Status struct {
	OID       string
	Branch    string
	Upstream  string
	Ahead     int
	Behind    int
	Detached  bool
	Dirty     int
	Untracked int
	Unmerged  int
	HasAB     bool // true if branch.ab was present
}

// ParsePorcelainV2 parses porcelain v2 status output (with --branch headers).
func ParsePorcelainV2(out []byte) (Status, error) {
	var s Status
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		switch line[0] {
		case '#':
			if err := parseHeader(&s, line); err != nil {
				return s, err
			}
		case '1', '2':
			// ordinary or rename/copy changed entry
			s.Dirty++
		case 'u':
			s.Unmerged++
			s.Dirty++
		case '?':
			s.Untracked++
		case '!':
			// ignored — skip
		default:
			// unknown line type; ignore
		}
	}
	if err := sc.Err(); err != nil {
		return s, fmt.Errorf("scan porcelain: %w", err)
	}
	if s.Branch == "(detached)" || strings.HasPrefix(s.Branch, "(") {
		s.Detached = true
	}
	return s, nil
}

func parseHeader(s *Status, line string) error {
	// # branch.oid <hex>|"(initial)"
	// # branch.head <branch>|" (detached)"
	// # branch.upstream <upstream>
	// # branch.ab +<ahead> -<behind>
	const prefix = "# "
	if !strings.HasPrefix(line, prefix) {
		return nil
	}
	rest := strings.TrimPrefix(line, prefix)
	key, val, ok := strings.Cut(rest, " ")
	if !ok {
		key = rest
		val = ""
	}
	switch key {
	case "branch.oid":
		s.OID = val
	case "branch.head":
		s.Branch = val
		if val == "(detached)" {
			s.Detached = true
		}
	case "branch.upstream":
		s.Upstream = val
	case "branch.ab":
		s.HasAB = true
		// format: +N -M
		parts := strings.Fields(val)
		for _, p := range parts {
			if len(p) < 2 {
				continue
			}
			n, err := strconv.Atoi(p[1:])
			if err != nil {
				return fmt.Errorf("parse branch.ab %q: %w", val, err)
			}
			switch p[0] {
			case '+':
				s.Ahead = n
			case '-':
				s.Behind = n
			}
		}
	}
	return nil
}

// ParseStashCount counts lines from git stash list.
func ParseStashCount(out []byte) int {
	n := 0
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			n++
		}
	}
	return n
}
