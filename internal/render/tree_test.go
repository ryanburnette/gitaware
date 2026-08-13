package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ryanburnette/gitaware/internal/model"
)

func TestTreeTable(t *testing.T) {
	report := model.Report{
		Root:        "/Users/ryan/git",
		Mode:        "offline",
		GeneratedAt: time.Now(),
		Orgs: []model.Org{
			{
				Name: "cogburnbros",
				Repos: []model.Repo{
					{
						Org: "cogburnbros", Name: "app", IsRepo: true,
						Branch: "main", HasRemote: true, Upstream: "origin/main",
						Untracked: 1, Stash: 2,
						Signals: []model.Signal{model.SignalUntracked, model.SignalStash},
					},
					{
						Org: "cogburnbros", Name: "domainpanel", IsRepo: true,
						Branch: "main", HasRemote: true, Upstream: "origin/main",
						Dirty: 1, Signals: []model.Signal{model.SignalDirty},
					},
				},
			},
			{
				Name: "ryanburnette",
				Repos: []model.Repo{
					{
						Org: "ryanburnette", Name: "domainpanel", IsRepo: true,
						Branch: "feature/multi-account", HasRemote: true, Upstream: "origin/feature/multi-account",
						Signals: []model.Signal{model.SignalNotDefault},
					},
					{
						Org: "ryanburnette", Name: "authn", IsRepo: true,
						Branch: "main", HasRemote: true, Upstream: "origin/main",
						OK: true,
					},
				},
			},
		},
		Summary: model.Summary{Repos: 4, Issues: 3, OK: 1},
	}

	var buf bytes.Buffer
	Tree(&buf, report, NewTheme(false))
	out := buf.String()

	for _, want := range []string{
		"gitaware",
		"/Users/ryan/git",
		"Repository",
		"Branch",
		"Dirty",
		"Untrk",
		"cogburnbros/app",
		"ryanburnette/domainpanel",
		"not default branch",
		"3 issues",
		"│", // table border
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestNotesOmitsCounts(t *testing.T) {
	r := model.Repo{
		IsRepo: true, Branch: "feat", Dirty: 2,
		Signals: []model.Signal{model.SignalDirty, model.SignalNotDefault},
	}
	parts := notesParts(r, NewTheme(false))
	joined := strings.Join(parts, " ")
	if strings.Contains(joined, "dirty") {
		t.Fatalf("counts in notes: %s", joined)
	}
	if !strings.Contains(joined, "not default branch") {
		t.Fatalf("got %v", parts)
	}
}
