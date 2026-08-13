package ghonline

import "testing"

func TestParseRemote(t *testing.T) {
	cases := []struct {
		in                string
		host, owner, repo string
		ok                bool
	}{
		{"git@github.com:ryanburnette/authn.git", "github.com", "ryanburnette", "authn", true},
		{"ssh://git@github.com/ryanburnette/authn.git", "github.com", "ryanburnette", "authn", true},
		{"ssh://git@github.com/therootcompany/domainpanel", "github.com", "therootcompany", "domainpanel", true},
		{"https://github.com/ryanburnette/authn.git", "github.com", "ryanburnette", "authn", true},
		{"https://gitlab.com/group/proj.git", "gitlab.com", "group", "proj", true},
		{"git@gitlab.com:group/proj.git", "gitlab.com", "group", "proj", true},
		{"", "", "", "", false},
	}
	for _, tc := range cases {
		id, ok := ParseRemote(tc.in)
		if ok != tc.ok || id.Host != tc.host || id.Owner != tc.owner || id.Name != tc.repo {
			t.Fatalf("%q => %+v ok=%v want %s/%s/%s", tc.in, id, ok, tc.host, tc.owner, tc.repo)
		}
	}
}

func TestRemoteMismatch(t *testing.T) {
	if !RemoteMismatch("git@github.com:other/authn.git", "ryanburnette", "authn") {
		t.Fatal("expected mismatch")
	}
	if RemoteMismatch("git@github.com:ryanburnette/authn.git", "ryanburnette", "authn") {
		t.Fatal("expected match")
	}
	if RemoteMismatch("git@github.com:other/authn.git", "", "") {
		t.Fatal("empty path should not mismatch")
	}
}

func TestMissingClones(t *testing.T) {
	remote := []RemoteRepo{
		{Name: "a"},
		{Name: "b", IsFork: true},
		{Name: "c", IsArchived: true},
		{Name: "d"},
	}
	local := map[string]bool{"a": true}
	miss := MissingClones("org", remote, local, true, false)
	// b fork excluded, c archived excluded, d missing
	if len(miss) != 1 || miss[0].Name != "d" {
		t.Fatalf("%+v", miss)
	}
}
