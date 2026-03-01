package hooks

import (
	"strings"
	"testing"
)

func TestRun_ZeroExit(t *testing.T) {
	exec := NewExecutor("sh -c")
	result := exec.Run("echo hello", nil)

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Err != nil {
		t.Errorf("Err = %v, want nil", result.Err)
	}
	if got := strings.TrimSpace(result.Stdout); got != "hello" {
		t.Errorf("Stdout = %q, want %q", got, "hello")
	}
}

func TestRun_NonZeroExit(t *testing.T) {
	exec := NewExecutor("sh -c")
	result := exec.Run("exit 42", nil)

	if result.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", result.ExitCode)
	}
	if result.Err != nil {
		t.Errorf("Err should be nil for exec.ExitError, got %v", result.Err)
	}
}

func TestRun_EmptyHookIsNoop(t *testing.T) {
	exec := NewExecutor("sh -c")
	result := exec.Run("", nil)

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0 for no-op", result.ExitCode)
	}
	if result.Stdout != "" {
		t.Errorf("Stdout = %q, want empty for no-op", result.Stdout)
	}
}

func TestRun_EnvVarsPassedCorrectly(t *testing.T) {
	exec := NewExecutor("sh -c")
	env := map[string]string{
		"LW_BRANCH":  "feat-x",
		"LW_PATH":    "/tmp/wt/feat-x",
		"LW_ACTION":  "open",
		"LW_PROJECT": "/tmp/project",
	}
	result := exec.Run("echo $LW_BRANCH", env)

	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stdout); got != "feat-x" {
		t.Errorf("Stdout = %q, want %q", got, "feat-x")
	}
}

func TestRun_StderrCaptured(t *testing.T) {
	exec := NewExecutor("sh -c")
	result := exec.Run("echo oops >&2", nil)

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stderr); got != "oops" {
		t.Errorf("Stderr = %q, want %q", got, "oops")
	}
}

func TestRun_CustomShell(t *testing.T) {
	exec := NewExecutor("bash -c")
	result := exec.Run("echo from_bash", nil)

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stdout); got != "from_bash" {
		t.Errorf("Stdout = %q, want %q", got, "from_bash")
	}
}

func TestRunPre_ZeroExitNotBlocked(t *testing.T) {
	exec := NewExecutor("sh -c")
	result, blocked := exec.RunPre("echo ok", nil)

	if blocked {
		t.Error("blocked = true, want false for zero exit")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestRunPre_NonZeroExitBlocked(t *testing.T) {
	exec := NewExecutor("sh -c")
	result, blocked := exec.RunPre("echo denied >&2; exit 1", nil)

	if !blocked {
		t.Error("blocked = false, want true for non-zero exit")
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stderr); got != "denied" {
		t.Errorf("Stderr = %q, want %q", got, "denied")
	}
}

func TestRunPre_EmptyHookNotBlocked(t *testing.T) {
	exec := NewExecutor("sh -c")
	_, blocked := exec.RunPre("", nil)

	if blocked {
		t.Error("blocked = true, want false for empty hook")
	}
}

func TestNewExecutor_DefaultShell(t *testing.T) {
	exec := NewExecutor("")
	if exec.Shell != "sh -c" {
		t.Errorf("Shell = %q, want %q", exec.Shell, "sh -c")
	}
}
