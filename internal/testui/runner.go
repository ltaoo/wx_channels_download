package testui

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// RunStatus represents the current state of a test run.
type RunStatus string

const (
	StatusRunning RunStatus = "running"
	StatusPassed  RunStatus = "passed"
	StatusFailed  RunStatus = "failed"
	StatusError   RunStatus = "error"
)

// RunResult holds the output and metadata for a single test execution.
type RunResult struct {
	RunID      string    `json:"run_id"`
	Pkg        string    `json:"pkg"`
	Test       string    `json:"test"` // empty means all tests in the package
	Status     RunStatus `json:"status"`
	Output     string    `json:"output"`
	DurationMs int64     `json:"duration_ms"`
	StartedAt  time.Time `json:"started_at"`
}

type runner struct {
	mu       sync.RWMutex
	runs     map[string]*RunResult
	modDir   string
	timeout  time.Duration
}

func newRunner(modDir string, timeout time.Duration) *runner {
	r := &runner{
		runs:    make(map[string]*RunResult),
		modDir:  modDir,
		timeout: timeout,
	}
	// periodic cleanup of old runs (every 60 seconds)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			r.cleanup()
		}
	}()
	return r
}

// StartRun launches a test execution in a goroutine and returns a run ID.
// If testName is empty, runs all tests in the package.
func (r *runner) StartRun(pkg, testName string) string {
	runID := randID()[:8]

	result := &RunResult{
		RunID:     runID,
		Pkg:       pkg,
		Test:      testName,
		Status:    StatusRunning,
		StartedAt: time.Now(),
	}

	r.mu.Lock()
	r.runs[runID] = result
	r.mu.Unlock()

	go r.execute(runID, pkg, testName)
	return runID
}

// GetResult returns the current result for a run, or nil if not found.
func (r *runner) GetResult(runID string) *RunResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.runs[runID]
}

// execute runs the test and populates the result.
func (r *runner) execute(runID, pkg, testName string) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	args := []string{"test", "-v", "-count=1"}
	if testName != "" {
		args = append(args, "-run", fmt.Sprintf("^%s$", testName))
	}
	args = append(args, pkg)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = r.modDir

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start).Milliseconds()

	output := outBuf.String()

	r.mu.Lock()
	defer r.mu.Unlock()

	result, ok := r.runs[runID]
	if !ok {
		return
	}

	if ctx.Err() != nil {
		result.Status = StatusError
		output += "\n--- TIMEOUT: test exceeded " + r.timeout.String() + " ---"
	} else if err != nil {
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0 {
			// shouldn't happen, but be safe
			result.Status = StatusPassed
		} else {
			result.Status = StatusFailed
		}
	} else {
		result.Status = StatusPassed
	}

	result.Output = output
	result.DurationMs = elapsed
}

// cleanup removes runs older than 5 minutes.
func (r *runner) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute)
	for id, run := range r.runs {
		if run.StartedAt.Before(cutoff) {
			delete(r.runs, id)
		}
	}
}

// randID returns a short random hex string suitable for run IDs.
func randID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// fallback: timestamp-based
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
