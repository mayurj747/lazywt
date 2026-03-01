package git

import (
	"testing"
)

func TestParsePorcelain(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantLen    int
		wantPath   string
		wantBranch string
	}{
		{
			name: "single worktree main",
			input: `worktree /home/user/project
HEAD abc123def456
branch refs/heads/main
`,
			wantLen:    1,
			wantPath:   "/home/user/project",
			wantBranch: "main",
		},
		{
			name: "bare entry is filtered out",
			input: `worktree /home/user/project.git
bare

worktree /home/user/project/worktrees/main
HEAD abc123def456
branch refs/heads/main
`,
			wantLen:    1,
			wantPath:   "/home/user/project/worktrees/main",
			wantBranch: "main",
		},
		{
			name: "multiple worktrees",
			input: `worktree /home/user/project
HEAD abc123def456
branch refs/heads/main

worktree /home/user/project/worktrees/feat-x
HEAD def456abc789
branch refs/heads/feat-x
`,
			wantLen:    2,
			wantPath:   "/home/user/project/worktrees/feat-x",
			wantBranch: "feat-x",
		},
		{
			name: "detached HEAD",
			input: `worktree /home/user/project
HEAD abc123def456
`,
			wantLen:    1,
			wantPath:   "/home/user/project",
			wantBranch: "",
		},
		{
			name: "branch without refs/heads prefix",
			input: `worktree /home/user/project
HEAD abc123def456
branch main
`,
			wantLen:    1,
			wantPath:   "/home/user/project",
			wantBranch: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worktrees, err := parsePorcelain(tt.input)
			if err != nil {
				t.Fatalf("parsePorcelain error: %v", err)
			}
			if len(worktrees) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(worktrees), tt.wantLen)
			}
			if tt.wantLen > 0 {
				last := worktrees[len(worktrees)-1]
				if last.Path != tt.wantPath {
					t.Errorf("Path = %q, want %q", last.Path, tt.wantPath)
				}
				if last.Branch != tt.wantBranch {
					t.Errorf("Branch = %q, want %q", last.Branch, tt.wantBranch)
				}
			}
		})
	}
}

func TestParseStanza(t *testing.T) {
	tests := []struct {
		name       string
		stanza     string
		wantPath   string
		wantBranch string
		wantHash   string
		wantBare   bool
	}{
		{
			name: "basic worktree",
			stanza: `worktree /home/user/project
HEAD abc123
branch refs/heads/main`,
			wantPath:   "/home/user/project",
			wantBranch: "main",
			wantHash:   "abc123",
		},
		{
			name:       "detached HEAD",
			stanza:     "worktree /tmp/wt\nHEAD def456",
			wantPath:   "/tmp/wt",
			wantBranch: "",
			wantHash:   "def456",
		},
		{
			name:     "bare entry detected",
			stanza:   "worktree /home/user/project.git\nbare",
			wantPath: "/home/user/project.git",
			wantBare: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wt, isBare, err := parseStanza(tt.stanza)
			if err != nil {
				t.Fatalf("parseStanza error: %v", err)
			}
			if isBare != tt.wantBare {
				t.Errorf("isBare = %v, want %v", isBare, tt.wantBare)
			}
			if wt.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", wt.Path, tt.wantPath)
			}
			if wt.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", wt.Branch, tt.wantBranch)
			}
			if wt.LastCommitHash != tt.wantHash {
				t.Errorf("LastCommitHash = %q, want %q", wt.LastCommitHash, tt.wantHash)
			}
		})
	}
}
