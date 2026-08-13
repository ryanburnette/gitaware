package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/model"
)

const maxBranchWidth = 28

// Column indexes in the status table.
const (
	colStatus = iota
	colRepo
	colBranch
	colDirty
	colUntrk
	colAhead
	colBehind
	colStash
	colNotes
	colCount
)

// Tree writes a bordered status table.
func Tree(w io.Writer, report model.Report, th Theme) {
	if len(report.Orgs) == 0 && len(report.Missing) == 0 {
		fmt.Fprintln(w, th.Meta.Render("no repositories found under "+report.Root))
		fmt.Fprintln(w)
		fmt.Fprintln(w, summaryLine(report, th))
		return
	}

	fmt.Fprintln(w, titleLine(report, th))
	fmt.Fprintln(w)

	rows := buildRows(report, th)

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(th.Border).
		BorderHeader(true).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		BorderColumn(true).
		BorderRow(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			base := th.Cell
			switch col {
			case colStatus:
				return base.Align(lipgloss.Center).Width(3)
			case colDirty, colUntrk, colAhead, colBehind, colStash:
				return base.Align(lipgloss.Center)
			case colNotes:
				return base.Align(lipgloss.Left)
			default:
				return base.Align(lipgloss.Left)
			}
		}).
		Headers(
			"", // status dot
			"Repository",
			"Branch",
			"Dirty",
			"Untrk",
			"Ahead",
			"Behind",
			"Stash",
			"Notes",
		).
		Rows(rows...)

	fmt.Fprintln(w, t.Render())
	fmt.Fprintln(w)
	fmt.Fprintln(w, summaryLine(report, th))
}

func titleLine(report model.Report, th Theme) string {
	return th.Brand.Render("gitaware") + "  " +
		th.Path.Render(report.Root) +
		th.Meta.Render("  ·  ") +
		th.Meta.Render(report.Mode)
}

func buildRows(report model.Report, th Theme) [][]string {
	var rows [][]string
	for _, org := range report.Orgs {
		for _, r := range org.Repos {
			rows = append(rows, repoRow(r, th))
		}
		for _, m := range org.Missing {
			rows = append(rows, missingRow(m, th))
		}
	}
	for _, m := range orphanMissing(report) {
		rows = append(rows, missingRow(m, th))
	}
	return rows
}

func repoRow(r model.Repo, th Theme) []string {
	// Distinct glyphs so status is clear even without color.
	status := th.StatusIssue.Render("●")
	if r.OK {
		status = th.StatusOK.Render("○")
	}

	name := r.DisplayName()
	if !r.IsRepo {
		return []string{
			status,
			th.Repo.Render(name),
			th.Meta.Render("—"),
			"", "", "", "", "",
			th.Danger.Render("not a git repo"),
		}
	}
	if r.Err != "" {
		msg := r.Err
		if lipgloss.Width(msg) > 40 {
			msg = truncate(msg, 40)
		}
		return []string{
			status,
			th.Repo.Render(name),
			th.Danger.Render("error"),
			"", "", "", "", "",
			th.Meta.Render(msg),
		}
	}

	return []string{
		status,
		styleRepoLabel(r.DisplayName(), th),
		styleBranch(r, th),
		th.pill(r.Dirty, th.DirtyFG, th.DirtyBG),
		th.pill(r.Untracked, th.UntrkFG, th.UntrkBG),
		th.pill(r.Ahead, th.AheadFG, th.AheadBG),
		th.pill(r.Behind, th.BehindFG, th.BehindBG),
		th.pill(r.Stash, th.StashFG, th.StashBG),
		notesCell(r, th),
	}
}

func missingRow(m model.MissingRepo, th Theme) []string {
	return []string{
		th.StatusIssue.Render("●"),
		styleRepoLabel(m.DisplayName(), th),
		th.Meta.Render("—"),
		"", "", "", "", "",
		th.Danger.Render("not cloned"),
	}
}

// styleRepoLabel dims the org/ prefix when present.
func styleRepoLabel(label string, th Theme) string {
	if i := strings.LastIndex(label, "/"); i > 0 && i < len(label)-1 {
		return th.Org.Render(label[:i+1]) + th.Repo.Render(label[i+1:])
	}
	return th.Repo.Render(label)
}

func branchPlain(r model.Repo) string {
	if r.Detached {
		return "DETACHED"
	}
	if r.Branch == "" {
		return "—"
	}
	return r.Branch
}

func styleBranch(r model.Repo, th Theme) string {
	plain := branchPlain(r)
	if lipgloss.Width(plain) > maxBranchWidth {
		plain = truncate(plain, maxBranchWidth)
	}
	if r.Detached {
		return th.Danger.Render(plain)
	}
	if plain == "—" || plain == "" {
		return th.Meta.Render(plain)
	}
	defaults := config.DefaultBranches
	if r.DefaultBranch != "" {
		defaults = []string{r.DefaultBranch}
	}
	if config.IsDefaultBranch(branchPlain(r), defaults) {
		return th.Branch.Render(plain)
	}
	return th.BranchHL.Render(plain)
}

func notesCell(r model.Repo, th Theme) string {
	parts := notesParts(r, th)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, th.Meta.Render(" · "))
}

func notesParts(r model.Repo, th Theme) []string {
	// ok is shown via green status dot; keep Notes empty when clean.
	if r.OK && len(r.Signals) == 0 {
		return nil
	}
	var parts []string
	if r.HasSignal(model.SignalRemoteNewer) {
		parts = append(parts, th.Warn.Render("remote has updates"))
	}
	if r.HasSignal(model.SignalNoUpstream) {
		parts = append(parts, th.Warn.Render("no upstream"))
	}
	if r.HasSignal(model.SignalNoRemote) {
		parts = append(parts, th.Warn.Render("no remote"))
	}
	if r.HasSignal(model.SignalRemoteMismatch) {
		parts = append(parts, th.Danger.Render("remote ≠ path"))
	}
	if r.Detached {
		parts = append(parts, th.Danger.Render("detached HEAD"))
	}
	if r.HasSignal(model.SignalNotDefault) && !r.Detached {
		parts = append(parts, th.Warn.Render("not default branch"))
	}
	if r.PR != nil {
		parts = append(parts, PRLink(th, r.PR.Number, r.PR.URL, false))
	}
	if r.HasSignal(model.SignalNotRepo) {
		parts = append(parts, th.Danger.Render("not a git repo"))
	}
	if r.HasSignal(model.SignalError) {
		parts = append(parts, th.Danger.Render("error"))
	}
	return parts
}

func orphanMissing(report model.Report) []model.MissingRepo {
	if len(report.Missing) == 0 {
		return nil
	}
	attached := map[string]bool{}
	for _, org := range report.Orgs {
		for _, m := range org.Missing {
			attached[m.DisplayName()] = true
		}
	}
	var out []model.MissingRepo
	for _, m := range report.Missing {
		if !attached[m.DisplayName()] {
			out = append(out, m)
		}
	}
	return out
}

func summaryLine(report model.Report, th Theme) string {
	s := report.Summary
	dot := th.Meta.Render("  ·  ")
	if s.Issues == 0 && s.Missing == 0 {
		return th.OK.Render("all clear") +
			dot + th.Summary.Render(fmt.Sprintf("%d repos", s.Repos)) +
			dot + th.Meta.Render(report.Mode)
	}
	parts := []string{
		th.Issues.Render(fmt.Sprintf("%d issues", s.Issues)),
		th.Summary.Render(fmt.Sprintf("%d repos", s.Repos)),
	}
	if s.Missing > 0 {
		parts = append(parts, th.Summary.Render(fmt.Sprintf("%d not cloned", s.Missing)))
	}
	if s.OK > 0 {
		parts = append(parts, th.Summary.Render(fmt.Sprintf("%d ok", s.OK)))
	}
	parts = append(parts, th.Meta.Render(report.Mode))
	return strings.Join(parts, dot)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw >= width {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + "…"
}
