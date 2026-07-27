package scanner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
)

// Executor runs external scanner commands and captures output.
type Executor struct {
	log      *logging.Logger
	timeout  time.Duration
	workDir  string
}

// ExecutorOption configures the executor.
type ExecutorOption func(*Executor)

// WithTimeout sets the command execution timeout.
func WithTimeout(d time.Duration) ExecutorOption {
	return func(e *Executor) { e.timeout = d }
}

// WithWorkDir sets the working directory for the command.
func WithWorkDir(dir string) ExecutorOption {
	return func(e *Executor) { e.workDir = dir }
}

// NewExecutor creates a new command executor.
func NewExecutor(log *logging.Logger, opts ...ExecutorOption) *Executor {
	e := &Executor{
		log:     log,
		timeout: 10 * time.Minute,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Result holds the output of a command execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Run executes a command with arguments and returns the result.
func (e *Executor) Run(ctx context.Context, name string, args ...string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if e.workDir != "" {
		cmd.Dir = e.workDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	e.log.Debug("running command", "cmd", name, "args", strings.Join(args, " "))

	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	result.ExitCode = 0
	return result, nil
}

// RunWithInput executes a command with stdin input.
func (e *Executor) RunWithInput(ctx context.Context, input string, name string, args ...string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, fmt.Errorf("command failed: %w\nstderr: %s", err, stderr.String())
	}

	result.ExitCode = 0
	return result, nil
}

// CheckDependencies verifies that required binaries are installed.
func CheckDependencies(log *logging.Logger, binaries ...string) error {
	for _, bin := range binaries {
		path, err := exec.LookPath(bin)
		if err != nil {
			return fmt.Errorf("required binary %q not found: %w", bin, err)
		}
		log.Info("found dependency", "binary", bin, "path", path)
	}
	return nil
}
