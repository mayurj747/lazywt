package projectinit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExtractProjectName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "HTTPS with .git suffix",
			url:  "https://github.com/user/acme.git",
			want: "acme",
		},
		{
			name: "HTTPS without .git suffix",
			url:  "https://github.com/user/acme",
			want: "acme",
		},
		{
			name: "SSH with .git suffix",
			url:  "git@github.com:user/acme.git",
			want: "acme",
		},
		{
			name: "SSH without .git suffix",
			url:  "git@github.com:user/acme",
			want: "acme",
		},
		{
			name: "trailing slash stripped",
			url:  "https://github.com/user/acme/",
			want: "acme",
		},
		{
			name: "local path with .git suffix",
			url:  "/home/user/repos/acme.git",
			want: "acme",
		},
		{
			name: "local path without .git suffix",
			url:  "/home/user/repos/acme",
			want: "acme",
		},
		{
			name: "whitespace trimmed",
			url:  "  https://github.com/user/acme.git  ",
			want: "acme",
		},
		{
			name: "nested SSH path",
			url:  "git@gitlab.com:org/team/acme.git",
			want: "acme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractProjectName(tt.url)
			if got != tt.want {
				t.Errorf("ExtractProjectName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// makeTestRepo creates a minimal git repo at dir with one commit and an
// "origin" remote pointing to a sibling bare repo. Returns the repo path.
func makeTestRepo(t *testing.T, parent, name string) string {
	t.Helper()

	repoDir := filepath.Join(parent, name)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Init with an explicit initial branch to avoid ambiguity.
	run(repoDir, "init", "-b", "main")
	run(repoDir, "config", "user.email", "test@test.com")
	run(repoDir, "config", "user.name", "Test")

	// Create a commit so the repo has a branch.
	readmeFile := filepath.Join(repoDir, "README")
	if err := os.WriteFile(readmeFile, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	run(repoDir, "add", ".")
	run(repoDir, "commit", "-m", "init")

	// Create a bare clone to act as the remote, then set it as "origin".
	bareRemote := filepath.Join(parent, name+"-remote.git")
	run(parent, "clone", "--bare", repoDir, bareRemote)
	run(repoDir, "remote", "add", "origin", bareRemote)

	return repoDir
}

func TestMigrate_CreatesLazytwtLayout(t *testing.T) {
	tmp := t.TempDir()
	srcRepo := makeTestRepo(t, tmp, "myrepo")

	// Run migration; project is created as a sibling of srcRepo.
	if err := Migrate(srcRepo, "myproject"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	projectDir := filepath.Join(tmp, "myproject")

	// Check bare repo exists.
	bareDir := filepath.Join(projectDir, "myproject.git")
	if _, err := os.Stat(bareDir); err != nil {
		t.Errorf("bare repo missing: %v", err)
	}

	// Check worktrees/main exists.
	wtDir := filepath.Join(projectDir, "worktrees", "main")
	if _, err := os.Stat(wtDir); err != nil {
		t.Errorf("worktrees/main missing: %v", err)
	}

	// Check scripts/ exists.
	if _, err := os.Stat(filepath.Join(projectDir, "scripts")); err != nil {
		t.Errorf("scripts/ missing: %v", err)
	}

	// Check lazywt.toml exists.
	if _, err := os.Stat(filepath.Join(projectDir, "lazywt.toml")); err != nil {
		t.Errorf("lazywt.toml missing: %v", err)
	}
}

func TestMigrate_ErrorsOnNonRepo(t *testing.T) {
	tmp := t.TempDir()
	notARepo := filepath.Join(tmp, "notarepo")
	if err := os.MkdirAll(notARepo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(notARepo, "proj"); err == nil {
		t.Error("expected error for non-git directory, got nil")
	}
}

func TestMigrate_ErrorsOnEmptySourcePath(t *testing.T) {
	if err := Migrate("", ""); err == nil {
		t.Error("expected error for empty source path, got nil")
	}
}

func TestMigrate_ErrorsWhenProjectAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	srcRepo := makeTestRepo(t, tmp, "myrepo2")

	// Pre-create the target project dir.
	if err := os.MkdirAll(filepath.Join(tmp, "conflict"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(srcRepo, "conflict"); err == nil {
		t.Error("expected error when project dir already exists, got nil")
	}
}
