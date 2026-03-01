package hooks

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type HookResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

type OutputLine struct {
	Stream    string // "stdout" or "stderr"
	Text      string
	Timestamp time.Time
	Hook      string // hook event name, e.g. "on_open", or "git" for git commands
}

type Executor struct {
	Shell string // e.g. "sh -c", "bash -c", "bash -ic"
}

func NewExecutor(shell string) *Executor {
	if shell == "" {
		shell = "sh -c"
	}
	return &Executor{Shell: shell}
}

// Run executes a hook command and captures stdout/stderr.
// Returns a no-op result if hookCmd is empty.
func (e *Executor) Run(hookCmd string, env map[string]string) HookResult {
	if hookCmd == "" {
		return HookResult{}
	}

	shellParts := strings.Fields(e.Shell)
	args := append(shellParts[1:], hookCmd)
	cmd := exec.Command(shellParts[0], args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Env = buildEnv(env)

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return HookResult{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: -1,
				Err:      err,
			}
		}
	}

	return HookResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// RunPre executes a pre-hook. Returns (result, blocked).
// blocked=true when exit code != 0, meaning the action should be aborted.
func (e *Executor) RunPre(hookCmd string, env map[string]string) (HookResult, bool) {
	result := e.Run(hookCmd, env)
	return result, result.ExitCode != 0
}

// RunStreaming executes a hook command and streams stdout/stderr line-by-line
// via the onLine callback. Returns a HookResult with ExitCode/Err only
// (Stdout/Stderr fields are left empty).
func (e *Executor) RunStreaming(hookCmd string, env map[string]string, onLine func(OutputLine)) HookResult {
	if hookCmd == "" {
		return HookResult{}
	}

	shellParts := strings.Fields(e.Shell)
	args := append(shellParts[1:], hookCmd)
	cmd := exec.Command(shellParts[0], args...)
	cmd.Env = buildEnv(env)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return HookResult{ExitCode: -1, Err: err}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return HookResult{ExitCode: -1, Err: err}
	}

	if err := cmd.Start(); err != nil {
		return HookResult{ExitCode: -1, Err: err}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			onLine(OutputLine{
				Stream:    "stdout",
				Text:      scanner.Text(),
				Timestamp: time.Now(),
			})
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			onLine(OutputLine{
				Stream:    "stderr",
				Text:      scanner.Text(),
				Timestamp: time.Now(),
			})
		}
	}()

	wg.Wait()

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return HookResult{ExitCode: -1, Err: err}
		}
	}

	return HookResult{ExitCode: exitCode}
}

func buildEnv(vars map[string]string) []string {
	if len(vars) == 0 {
		return nil
	}

	env := os.Environ()
	for k, v := range vars {
		env = append(env, k+"="+v)
	}
	return env
}
