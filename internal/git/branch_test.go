package git

import "testing"

func TestParseBranches_Local(t *testing.T) {
	raw := "main\nfeature/foo\n\n"
	got := parseBranches(raw, false)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	tests := []struct {
		name     string
		idx      int
		wantName string
		wantRef  string
		wantDisp string
		wantRem  bool
	}{
		{name: "main", idx: 0, wantName: "main", wantRef: "main", wantDisp: "main", wantRem: false},
		{name: "feature", idx: 1, wantName: "feature/foo", wantRef: "feature/foo", wantDisp: "feature/foo", wantRem: false},
	}

	for _, tt := range tests {
		b := got[tt.idx]
		if b.Name != tt.wantName || b.Ref != tt.wantRef || b.Display != tt.wantDisp || b.IsRemote != tt.wantRem {
			t.Errorf("%s = %+v, want Name=%q Ref=%q Display=%q IsRemote=%v", tt.name, b, tt.wantName, tt.wantRef, tt.wantDisp, tt.wantRem)
		}
	}
}

func TestParseBranches_Remote(t *testing.T) {
	raw := "origin/HEAD\norigin/main\norigin/feature/foo\nupstream/release\n"
	got := parseBranches(raw, true)

	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}

	tests := []struct {
		name     string
		idx      int
		wantName string
		wantRef  string
	}{
		{name: "origin/main", idx: 0, wantName: "main", wantRef: "origin/main"},
		{name: "origin/feature/foo", idx: 1, wantName: "feature/foo", wantRef: "origin/feature/foo"},
		{name: "upstream/release", idx: 2, wantName: "release", wantRef: "upstream/release"},
	}

	for _, tt := range tests {
		b := got[tt.idx]
		if b.Name != tt.wantName {
			t.Errorf("%s Name = %q, want %q", tt.name, b.Name, tt.wantName)
		}
		if b.Ref != tt.wantRef {
			t.Errorf("%s Ref = %q, want %q", tt.name, b.Ref, tt.wantRef)
		}
		if b.Display != tt.wantRef {
			t.Errorf("%s Display = %q, want %q", tt.name, b.Display, tt.wantRef)
		}
		if !b.IsRemote {
			t.Errorf("%s IsRemote = false, want true", tt.name)
		}
	}
}
