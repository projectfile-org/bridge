// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package core defines the Bridge interface every projectfile↔external-file
// handler implements, plus the two capability interfaces (Syncer for
// round-trip, Renderer for derive-only) and the shared algorithm that drives
// them. Concrete bridges live one level up under internal/bridge/<name>/.
package core

import "kiota.ch/projectfile/core/v2/pkg/projectfile"

// Bridge is the identity surface. Both Syncer and Renderer extend it; the
// dispatcher in cmd/bridge.go consults only Bridge methods until it has to
// branch on capability.
//
// `any` for the external document is unavoidable: each Syncer owns the
// concrete type and the only code that type-asserts is the driver itself.
type Bridge interface {
	Name() string                                         // stable id: "cff", "license", ...
	Filename() string                                     // canonical on-disk name
	Aliases() []string                                    // alternative spellings (may be nil)
	Labels() (extName, pfName string)                     // labels rendered in FieldChange
	FullPath(dir string, pf *projectfile.Document) string // absolute write path
	Exists(dir string) bool
	Policy() Policy
}

// Syncer is a round-trip bridge — reads/writes the external file and maps
// fields in both directions.
type Syncer interface {
	Bridge
	NewEmpty() any
	Clone(doc any) any
	Read(dir string) (any, error)
	Write(dir string, doc any) error
	BuildMappers(extDoc any, pf *projectfile.Document) MapperList
}

// Renderer is a derive-only bridge — produces output bytes from pf with no
// reverse read.
type Renderer interface {
	Bridge
	Render(pf *projectfile.Document, opts Options) (Output, error)
}

// Output collapses the old (Body, Files) split into a single multi-file map
// keyed by path relative to opts.Dir. Single-file renderers return a
// one-entry map.
type Output struct {
	Files map[string][]byte
}

// Policy controls the dispatcher's overwrite behaviour. Marker unset means
// the dispatcher always overwrites (used for pure-data files with no
// user-edit expectation).
type Policy struct {
	// Marker requires the pf-cli-managed sentinel in any existing file
	// before overwriting (--force bypasses): the sentinel's presence means
	// the file is still pf-cli's to regenerate, and the dispatcher overwrites
	// it freely; its absence means a human detached it by removing the
	// comment, and the dispatcher leaves it alone.
	Marker bool
}

// RequiredFieldsBridge is an optional capability for either Syncer or
// Renderer — declares projectfile fields the bridge needs but did not find.
// The dispatcher walks Missing in TTY fill-mode; non-TTY runs fall through
// to whatever error the bridge raises on its own.
type RequiredFieldsBridge interface {
	Bridge
	RequiredFields(pf *projectfile.Document) []Missing
}

// StackAware is an optional capability for bridges that only apply to
// specific technology stacks. A non-empty return means the bridge should
// only run when at least one declared tag overlaps with pf.Stack. A nil or
// empty return (or not implementing the interface at all) means the bridge
// is universal — it runs regardless of stack.
type StackAware interface {
	Bridge
	Stacks() []string
}

// Describer is an optional capability for bridges whose Filename alone does
// not say what they manage. Describe returns the complete one-line
// self-introduction for the dispatcher's listing probe (direction included).
// Bridges whose Filename is the whole story need not implement it.
type Describer interface {
	Bridge
	Describe() string
}

// Missing is one prompt the dispatcher shows in fill-mode. Setter mutates
// pf in place; a nil Setter signals "I just need you to know about this but
// I cannot fix it" — the dispatcher surfaces the row read-only.
type Missing struct {
	Field  string
	Hint   string
	Setter func(value string) error
}
