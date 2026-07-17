package testui

import (
	"bufio"
	"bytes"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// PackageInfo holds test metadata for a single package.
type PackageInfo struct {
	ImportPath string     `json:"import_path"`
	Name       string     `json:"name"`
	Tests      []TestInfo `json:"tests"`
}

// TestInfo holds a single test function's metadata.
type TestInfo struct {
	Name string `json:"name"`
}

type discoverer struct {
	mu      sync.RWMutex
	cached  []PackageInfo
	modDir  string
}

func newDiscoverer(modDir string) *discoverer {
	return &discoverer{modDir: modDir}
}

// Discover runs go list and go test -list to find all test functions.
// Results are cached; the cache TTL is 30 seconds.
func (d *discoverer) Discover() ([]PackageInfo, error) {
	d.mu.RLock()
	if d.cached != nil {
		d.mu.RUnlock()
		return d.cached, nil
	}
	d.mu.RUnlock()

	packages, err := d.listPackages()
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	var all []PackageInfo
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // limit concurrency

	for _, pkg := range packages {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			tests, err := d.listTests(p)
			if err != nil || len(tests) == 0 {
				return
			}
			info := PackageInfo{
				ImportPath: p,
				Name:       shortName(p),
				Tests:      tests,
			}
			mu.Lock()
			all = append(all, info)
			mu.Unlock()
		}(pkg)
	}
	wg.Wait()

	d.mu.Lock()
	d.cached = all
	// auto-clear after 30 seconds
	time.AfterFunc(30*time.Second, func() {
		d.mu.Lock()
		d.cached = nil
		d.mu.Unlock()
	})
	d.mu.Unlock()

	return all, nil
}

// InvalidateCache clears the cached test list so the next call re-discovers.
func (d *discoverer) InvalidateCache() {
	d.mu.Lock()
	d.cached = nil
	d.mu.Unlock()
}

// listPackages runs go list ./... and returns all package import paths.
func (d *discoverer) listPackages() ([]string, error) {
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = d.modDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var pkgs []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}
	return pkgs, nil
}

// listTests runs go test -list for a single package.
func (d *discoverer) listTests(pkg string) ([]TestInfo, error) {
	cmd := exec.Command("go", "test", "-list", ".*", pkg)
	cmd.Dir = d.modDir
	out, err := cmd.Output()
	if err != nil {
		// skip packages that have no test files
		return nil, nil
	}
	var tests []TestInfo
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Filter out lines that are not test names
		// go test -list output: test names, then "ok   pkg  0.123s" or "?    pkg [no test files]"
		if line == "" || strings.HasPrefix(line, "ok") || strings.HasPrefix(line, "?") {
			continue
		}
		if strings.HasPrefix(line, "Test") || strings.HasPrefix(line, "Benchmark") || strings.HasPrefix(line, "Example") {
			tests = append(tests, TestInfo{Name: line})
		}
	}
	return tests, nil
}

// shortName returns the short package display name (last component after last "/").
func shortName(importPath string) string {
	if idx := strings.LastIndex(importPath, "/"); idx >= 0 {
		return importPath[idx+1:]
	}
	return importPath
}
