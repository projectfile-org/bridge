// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "sync"

// driverRegistry holds the per-kind driver factory. Each driver self-
// registers in its init() so blank-importing the package is the only
// integration step from cmd/forge.go.
//
// The factory shape (HTTPOptions → Client) keeps transport config out of
// the package's init order: cmd/forge.go reads --timeout / --insecure-skip-tls
// at runtime and passes them through.
type driverFactory func(opts HTTPOptions) Client

var (
	driverMu sync.RWMutex
	drivers  = map[string]driverFactory{}
)

// Register binds a forge kind to a driver factory. Safe to call from
// package init(); panics on duplicate registration so a misconfigured
// build fails loudly rather than silently shadowing a driver.
func Register(kind string, factory driverFactory) {
	driverMu.Lock()
	defer driverMu.Unlock()
	if _, dup := drivers[kind]; dup {
		panic("forge.core: duplicate driver registration for kind " + kind)
	}
	drivers[kind] = factory
}

// NewResolver returns a Registry closure bound to the supplied HTTPOptions.
// One instance per command invocation; the closure captures opts so every
// per-kind lookup yields a Client wired with the same transport settings.
func NewResolver(opts HTTPOptions) Registry {
	driverMu.RLock()
	snapshot := make(map[string]driverFactory, len(drivers))
	for k, v := range drivers {
		snapshot[k] = v
	}
	driverMu.RUnlock()
	return func(kind string) Client {
		f, ok := snapshot[kind]
		if !ok {
			return nil
		}
		return f(opts)
	}
}

// Kinds returns the list of registered kinds (for `forge list` capability output).
func Kinds() []string {
	driverMu.RLock()
	defer driverMu.RUnlock()
	out := make([]string, 0, len(drivers))
	for k := range drivers {
		out = append(out, k)
	}
	return out
}
