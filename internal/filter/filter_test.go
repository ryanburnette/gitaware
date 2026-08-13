package filter

import (
	"testing"

	"github.com/ryanburnette/gitaware/internal/config"
	"github.com/ryanburnette/gitaware/internal/model"
)

func TestLeaveIgnoresBehind(t *testing.T) {
	r := model.Repo{
		IsRepo:    true,
		Branch:    "main",
		HasRemote: true,
		Upstream:  "origin/main",
		Behind:    3,
	}
	opts := config.Options{StrictBranch: true}
	DeriveSignals(&r, opts, ModeLeave)
	if !r.OK {
		t.Fatalf("leave should ignore behind-only, signals=%v", r.Signals)
	}
}

func TestLeaveCatchesDirtyAndAhead(t *testing.T) {
	r := model.Repo{
		IsRepo:    true,
		Branch:    "main",
		HasRemote: true,
		Upstream:  "origin/main",
		Dirty:     1,
		Ahead:     2,
	}
	opts := config.Options{StrictBranch: true}
	DeriveSignals(&r, opts, ModeLeave)
	if r.OK {
		t.Fatal("expected issues")
	}
}

func TestLeaveStrictBranch(t *testing.T) {
	r := model.Repo{
		IsRepo:    true,
		Branch:    "feature",
		HasRemote: true,
		Upstream:  "origin/feature",
	}
	opts := config.Options{StrictBranch: true}
	DeriveSignals(&r, opts, ModeLeave)
	if r.OK {
		t.Fatal("expected not_default issue")
	}
}

func TestArriveCatchesBehind(t *testing.T) {
	r := model.Repo{
		IsRepo:    true,
		Branch:    "main",
		HasRemote: true,
		Upstream:  "origin/main",
		Behind:    1,
	}
	opts := config.Options{}
	DeriveSignals(&r, opts, ModeArrive)
	if r.OK {
		t.Fatal("expected behind issue on arrive")
	}
}

func TestStatusCleanOK(t *testing.T) {
	r := model.Repo{
		IsRepo:    true,
		Branch:    "main",
		HasRemote: true,
		Upstream:  "origin/main",
	}
	opts := config.Options{}
	DeriveSignals(&r, opts, ModeStatus)
	if !r.OK {
		t.Fatalf("signals=%v", r.Signals)
	}
}
