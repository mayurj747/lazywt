package projectinit

import (
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
