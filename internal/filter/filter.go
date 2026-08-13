package filter

import (
	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/model"
)

// Mode selects which signals count as issues.
type Mode int

const (
	// ModeStatus is the neutral report: all non-ok signals.
	ModeStatus Mode = iota
	// ModeLeave is offline unfinished work (no behind/missing).
	ModeLeave
	// ModeArrive is online catch-up (behind, dirty leftovers, drift). Missing is separate.
	ModeArrive
)

// DeriveSignals fills r.Signals and r.OK from fields and mode options.
func DeriveSignals(r *model.Repo, opts config.Options, mode Mode) {
	r.Signals = r.Signals[:0]

	if r.Err != "" {
		r.Signals = append(r.Signals, model.SignalError)
		r.OK = false
		return
	}
	if !r.IsRepo {
		r.Signals = append(r.Signals, model.SignalNotRepo)
		r.OK = false
		return
	}

	if r.Detached {
		r.Signals = append(r.Signals, model.SignalDetached)
	}
	if r.Dirty > 0 {
		r.Signals = append(r.Signals, model.SignalDirty)
	}
	if r.Untracked > 0 {
		r.Signals = append(r.Signals, model.SignalUntracked)
	}
	if r.Stash > 0 {
		r.Signals = append(r.Signals, model.SignalStash)
	}
	if !r.HasRemote {
		r.Signals = append(r.Signals, model.SignalNoRemote)
	}
	if r.HasRemote && r.Upstream == "" && !r.Detached {
		r.Signals = append(r.Signals, model.SignalNoUpstream)
	}
	if r.Ahead > 0 {
		r.Signals = append(r.Signals, model.SignalAhead)
	}
	if r.Behind > 0 {
		r.Signals = append(r.Signals, model.SignalBehind)
	}
	if r.RemoteNewer {
		r.Signals = append(r.Signals, model.SignalRemoteNewer)
	}

	def := r.DefaultBranch
	defaults := config.DefaultBranches
	if def != "" {
		defaults = []string{def}
	}
	if !r.Detached && r.Branch != "" && !config.IsDefaultBranch(r.Branch, defaults) {
		r.Signals = append(r.Signals, model.SignalNotDefault)
	}
	if r.PR != nil {
		r.Signals = append(r.Signals, model.SignalOpenPR)
	}
	// remote_mismatch is set by enricher before DeriveSignals

	r.OK = !isIssue(r, opts, mode)
}

func isIssue(r *model.Repo, opts config.Options, mode Mode) bool {
	if r.Err != "" || !r.IsRepo {
		return true
	}
	switch mode {
	case ModeLeave:
		return leaveIssue(r, opts)
	case ModeArrive:
		return arriveIssue(r, opts)
	default:
		return statusIssue(r, opts)
	}
}

func statusIssue(r *model.Repo, opts config.Options) bool {
	for _, s := range r.Signals {
		switch s {
		case model.SignalDirty, model.SignalUntracked, model.SignalAhead,
			model.SignalNoUpstream, model.SignalDetached, model.SignalStash,
			model.SignalNoRemote, model.SignalNotRepo, model.SignalError,
			model.SignalRemoteMismatch:
			return true
		case model.SignalBehind, model.SignalRemoteNewer:
			if opts.IncludeBehind || opts.Fetch || opts.CheckRemote || opts.Online {
				return true
			}
		case model.SignalNotDefault:
			// always surface on status issues-only
			return true
		case model.SignalOpenPR:
			// informational; only issue if also not default or explicitly wanted
			if r.HasSignal(model.SignalNotDefault) {
				return true
			}
		}
	}
	return false
}

func leaveIssue(r *model.Repo, opts config.Options) bool {
	for _, s := range r.Signals {
		switch s {
		case model.SignalDirty, model.SignalUntracked, model.SignalAhead,
			model.SignalNoUpstream, model.SignalDetached, model.SignalStash,
			model.SignalNoRemote, model.SignalError, model.SignalRemoteMismatch:
			return true
		case model.SignalNotDefault:
			if opts.StrictBranch {
				return true
			}
		}
	}
	return false
}

func arriveIssue(r *model.Repo, opts config.Options) bool {
	for _, s := range r.Signals {
		switch s {
		case model.SignalDirty, model.SignalUntracked, model.SignalAhead,
			model.SignalBehind, model.SignalRemoteNewer, model.SignalNoUpstream,
			model.SignalDetached, model.SignalStash, model.SignalNoRemote,
			model.SignalError, model.SignalRemoteMismatch, model.SignalNotDefault:
			return true
		case model.SignalOpenPR:
			return true
		}
	}
	_ = opts
	return false
}

// Apply filters report orgs/repos for display.
// When issuesOnly, drops ok repos and empty orgs (unless they have missing).
func Apply(report *model.Report, issuesOnly bool, mode Mode) {
	if !issuesOnly {
		// still recompute summary
		reSummary(report)
		return
	}

	var orgs []model.Org
	for _, org := range report.Orgs {
		var repos []model.Repo
		for _, r := range org.Repos {
			// re-derive OK under mode (already done); keep non-ok
			if !r.OK {
				repos = append(repos, r)
			}
		}
		missing := org.Missing
		if mode == ModeLeave {
			missing = nil
		}
		if len(repos) == 0 && len(missing) == 0 {
			continue
		}
		org.Repos = repos
		org.Missing = missing
		orgs = append(orgs, org)
	}
	report.Orgs = orgs

	if mode == ModeLeave {
		report.Missing = nil
	}
	reSummary(report)
}

func reSummary(report *model.Report) {
	var s model.Summary
	for _, org := range report.Orgs {
		for _, r := range org.Repos {
			s.Repos++
			if r.OK {
				s.OK++
			} else {
				s.Issues++
			}
		}
		s.Missing += len(org.Missing)
	}
	// top-level missing not already in orgs
	if s.Missing == 0 {
		s.Missing = len(report.Missing)
	}
	report.Summary = s
}

// RepoIsIssue is exported for tests.
func RepoIsIssue(r model.Repo, opts config.Options, mode Mode) bool {
	cp := r
	DeriveSignals(&cp, opts, mode)
	return !cp.OK
}
