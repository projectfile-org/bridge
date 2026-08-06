// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"fmt"
	"sort"
	"sync"

	"projectfile.org/projectfile/bridge/internal/source"
)

// Mirrors internal/bridge/registry.go: thread-safe map keyed by Name(),
// populated from each driver's init(). Registration order is preserved
// (registered slice) so RunAll's merge precedence is deterministic — first
// registered wins on conflicts, since MergePartials uses "existing wins".
var (
	mu         sync.RWMutex
	byName     = map[string]Scanner{}
	registered []Scanner
)

// Register adds s to the registry. Panics on empty Name or duplicate — both
// are init-time programmer errors and a busy CLI is worse off failing
// silently here than aborting startup.
func Register(s Scanner) {
	if s.Name() == "" {
		panic("scanners.Register: empty Name")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := byName[s.Name()]; dup {
		panic("scanners.Register: duplicate Name " + s.Name())
	}
	byName[s.Name()] = s
	registered = append(registered, s)
}

// Lookup returns the scanner registered under name, or (nil, false).
func Lookup(name string) (Scanner, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := byName[name]
	return s, ok
}

// List returns every registered scanner, sorted by Name for stable --list output.
func List() []Scanner {
	mu.RLock()
	out := make([]Scanner, len(registered))
	copy(out, registered)
	mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// RunAll runs every applicable scanner against root and merges the results.
// Iteration follows registration order so merge precedence is deterministic
// (first-registered wins under MergePartials' existing-wins rule). A failure
// in one scanner is logged via the returned error chain but does not abort
// the rest — partial output is more useful than no output at init time.
func RunAll(root string) (*source.Partial, []Hit, error) {
	mu.RLock()
	scanners := make([]Scanner, len(registered))
	copy(scanners, registered)
	mu.RUnlock()

	merged := &source.Partial{}
	var hits []Hit
	var errs []error

	for _, s := range scanners {
		// Cheap probe first — skip scan entirely when not applicable.
		if !s.Detect(root) {
			continue
		}
		p, h, err := s.Scan(root)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name(), err))
			continue
		}
		if p != nil {
			merged = source.MergePartials(merged, p)
		}
		hits = append(hits, h...)
	}

	if len(errs) > 0 {
		// Join into a single error so callers can decide to surface or ignore.
		return merged, hits, joinErrors(errs)
	}
	return merged, hits, nil
}

// RunNamed runs the scanners identified by names against root and merges
// results. Iteration follows the names slice order so the caller controls
// precedence. Unknown names produce an error but don't abort the rest.
// A failure in one scanner is logged via the returned error chain but does
// not abort the rest — partial output is more useful than no output.
func RunNamed(root string, names []string) (*source.Partial, []Hit, error) {
	mu.RLock()
	defer mu.RUnlock()

	merged := &source.Partial{}
	var hits []Hit
	var errs []error

	for _, name := range names {
		s, ok := byName[name]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown scanner: %s", name))
			continue
		}
		if !s.Detect(root) {
			continue
		}
		p, h, err := s.Scan(root)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if p != nil {
			merged = source.MergePartials(merged, p)
		}
		hits = append(hits, h...)
	}

	if len(errs) > 0 {
		return merged, hits, joinErrors(errs)
	}
	return merged, hits, nil
}

// joinErrors collapses multiple scanner failures into a single error value
// without pulling in errors.Join semantics (project targets go 1.24, but we
// keep the shape simple and grep-friendly for log scraping).
func joinErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("scanners failed: %v", msgs)
}
