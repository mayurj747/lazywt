package main

import (
	"strings"
	"testing"

	"github.com/mbency/lazyworktree/internal/git"
	"github.com/mbency/lazyworktree/internal/model"
)

func TestWantsHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "long flag", args: []string{"--help"}, want: true},
		{name: "short flag", args: []string{"-h"}, want: true},
		{name: "help token", args: []string{"help"}, want: true},
		{name: "empty", args: nil, want: false},
		{name: "arg not help", args: []string{"feat-x"}, want: false},
		{name: "help plus arg", args: []string{"--help", "extra"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wantsHelp(tt.args); got != tt.want {
				t.Errorf("wantsHelp(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestUsageTextsIncludeExamples(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "main usage",
			text: mainUsageText(),
			want: []string{"lw <command> --help", "lw list [--format=json]"},
		},
		{
			name: "list usage",
			text: listUsageText(),
			want: []string{"Options:", "--format=json", "Examples:"},
		},
		{
			name: "create usage",
			text: createUsageText(),
			want: []string{"tracking branch", "lw create feat-x"},
		},
		{
			name: "delete usage",
			text: deleteUsageText(),
			want: []string{"lw delete feat-x"},
		},
		{
			name: "open usage",
			text: openUsageText(),
			want: []string{"on_open hooks", "lw open feat-x"},
		},
		{
			name: "init usage",
			text: initUsageText(),
			want: []string{"lw init [url]", "prompts interactively"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, needle := range tt.want {
				if !strings.Contains(tt.text, needle) {
					t.Errorf("usage text missing %q", needle)
				}
			}
		})
	}
}

func TestPickRemoteRef(t *testing.T) {
	tests := []struct {
		name     string
		branches []git.Branch
		branch   string
		wantRef  string
		wantOK   bool
	}{
		{
			name: "prefers origin when available",
			branches: []git.Branch{
				{Name: "feat-x", Ref: "upstream/feat-x", IsRemote: true},
				{Name: "feat-x", Ref: "origin/feat-x", IsRemote: true},
			},
			branch:  "feat-x",
			wantRef: "origin/feat-x",
			wantOK:  true,
		},
		{
			name: "uses first non-origin remote as fallback",
			branches: []git.Branch{
				{Name: "release", Ref: "upstream/release", IsRemote: true},
				{Name: "main", Ref: "origin/main", IsRemote: true},
			},
			branch:  "release",
			wantRef: "upstream/release",
			wantOK:  true,
		},
		{
			name: "ignores local branches",
			branches: []git.Branch{
				{Name: "feat-y", Ref: "feat-y", IsRemote: false},
			},
			branch: "feat-y",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRef, gotOK := pickRemoteRef(tt.branches, tt.branch)
			if gotOK != tt.wantOK {
				t.Fatalf("ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotRef != tt.wantRef {
				t.Errorf("ref = %q, want %q", gotRef, tt.wantRef)
			}
		})
	}
}

func TestFindWorktreeByBranch(t *testing.T) {
	worktrees := []model.Worktree{
		{Branch: "main", Path: "/tmp/main"},
		{Branch: "feat-z", Path: "/tmp/feat-z"},
	}

	got := findWorktreeByBranch(worktrees, "feat-z")
	if got == nil {
		t.Fatal("got nil, want worktree")
	}
	if got.Path != "/tmp/feat-z" {
		t.Errorf("Path = %q, want %q", got.Path, "/tmp/feat-z")
	}

	notFound := findWorktreeByBranch(worktrees, "missing")
	if notFound != nil {
		t.Errorf("got %v, want nil", notFound)
	}
}
