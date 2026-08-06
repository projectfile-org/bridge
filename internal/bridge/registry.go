// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package bridge is the filename-keyed registry of every projectfile↔external
// file handler the CLI ships. Concrete bridges live in subpackages under
// internal/bridge/<name>/ and self-register here in init() so cmd/bridge.go
// can dispatch off a single, on-disk-name-keyed registry.
//
// A registered bridge implements either core.Syncer (round-trip) or
// core.Renderer (derive-only). The dispatcher filters by capability when
// listing or routing.
package bridge

import (
	"sort"
	"sync"

	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// The registry is keyed by both Filename and every Alias so Lookup is O(1).
// primary holds only canonical entries so List/Filter don't double-count
// alias collisions.
var (
	mu      sync.RWMutex
	byKey   = map[string]core.Bridge{}
	primary = map[string]core.Bridge{}
)

// Register adds b to the registry. Panics on:
//   - empty Filename (programmer error caught at init time)
//   - duplicate Filename (two handlers fighting for the same on-disk name)
//   - alias collision with any existing Filename or alias
//
// Panics at init are appropriate here: the harm is mis-routing on a busy
// CLI, not a recoverable runtime fault.
func Register(b core.Bridge) {
	name := b.Filename()
	if name == "" {
		panic("bridge.Register: empty Filename")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := primary[name]; dup {
		panic("bridge.Register: duplicate Filename " + name)
	}
	if _, dup := byKey[name]; dup {
		panic("bridge.Register: Filename " + name + " collides with an existing alias")
	}
	primary[name] = b
	byKey[name] = b
	for _, a := range b.Aliases() {
		if a == "" {
			continue
		}
		if _, dup := byKey[a]; dup {
			panic("bridge.Register: alias " + a + " for " + name + " collides with existing key")
		}
		byKey[a] = b
	}
}

// Lookup matches Filename or any registered alias. Returns (nil, false)
// when the name is unknown.
func Lookup(name string) (core.Bridge, bool) {
	mu.RLock()
	defer mu.RUnlock()
	b, ok := byKey[name]
	return b, ok
}

// List returns every registered bridge (one entry per Filename, aliases
// folded), sorted by Filename for stable --list output.
func List() []core.Bridge {
	mu.RLock()
	out := make([]core.Bridge, 0, len(primary))
	for _, b := range primary {
		out = append(out, b)
	}
	mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Filename() < out[j].Filename() })
	return out
}

// Filter returns bridges matching pred, preserving List's sort order.
func Filter(pred func(core.Bridge) bool) []core.Bridge {
	all := List()
	out := make([]core.Bridge, 0, len(all))
	for _, b := range all {
		if pred(b) {
			out = append(out, b)
		}
	}
	return out
}

// IsSyncer reports whether b is round-trip capable (handy as a filter for
// `bridge --list --syncers` or similar capability gates).
func IsSyncer(b core.Bridge) bool {
	_, ok := b.(core.Syncer)
	return ok
}

// IsRenderer reports whether b is derive-only capable.
func IsRenderer(b core.Bridge) bool {
	_, ok := b.(core.Renderer)
	return ok
}

// MatchesStack reports whether b should run given the project's declared
// stacks. Returns true when the bridge does not implement StackAware
// (universal), when its Stacks() returns nil/empty (also universal), or
// when at least one bridge stack tag appears in projectStacks. An empty
// projectStacks means "no stacks declared" — all bridges match (we have no
// information to filter on).
func MatchesStack(b core.Bridge, projectStacks []string) bool {
	sa, ok := b.(core.StackAware)
	if !ok {
		return true
	}
	bridgeStacks := sa.Stacks()
	if len(bridgeStacks) == 0 {
		return true
	}
	if len(projectStacks) == 0 {
		return true
	}
	for _, bs := range bridgeStacks {
		for _, ps := range projectStacks {
			if bs == ps {
				return true
			}
		}
	}
	return false
}
