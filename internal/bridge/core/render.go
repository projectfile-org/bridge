// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
)

// LocalTemplatesDir is the project-local override directory. When a file
// named <name> exists under this path (relative to the project dir), it
// takes precedence over the embedded template registered by the bridge.
const LocalTemplatesDir = ".projectfile/templates"

// ReadLocalTemplate reads a project-local template override, resolving rel
// under dir via os.Root so a name containing ".." cannot escape the project
// directory (the G304 traversal vector). The read goes through os.Root
// rather than a plain os.ReadFile precisely so the sandboxing is enforced
// by the runtime, not by hand-rolled path validation.
//
// Returns (body, true, nil) on a hit; (nil, false, nil) when the override
// does not exist or names a directory (fall through to the embedded tier);
// (nil, false, err) only on a real read failure.
func ReadLocalTemplate(dir, rel string) ([]byte, bool, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, false, nil // dir missing/unreadable → no override
	}
	defer root.Close()
	f, err := root.Open(rel)
	if err != nil {
		return nil, false, nil // not found → embedded tier
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return nil, false, nil
	}
	body, err := io.ReadAll(f)
	if err != nil {
		return nil, false, fmt.Errorf("bridge: read local template %s: %w", rel, err)
	}
	return body, true, nil
}

// Render executes the named template against data and returns the rendered
// body. Templates are resolved in two tiers:
//
//  1. Project-local override: <dir>/.projectfile/templates/<name>.
//  2. Embedded template registered by the bridge at init() time.
//
// Templates are parsed exactly once per (dir, name) pair and cached for
// the process lifetime. Parse errors surface from the first Render call
// rather than at init(), keeping init() free of panic.
func Render(dir string, name string, data any) ([]byte, error) {
	tpl, err := lookup(dir, name)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("bridge: exec %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// templateLoader is the read-template-bytes callback subpackages hand to
// RegisterTemplates. It returns the template body (typically by calling
// embedFS.ReadFile under the hood) so the renderer never has to know
// about embed.FS layout.
type templateLoader func(name string) ([]byte, error)

var loaders = map[string]templateLoader{}

// parsedMu guards both parsed and parsedLocal.
var parsedMu sync.RWMutex

// parsed caches embedded templates by name.
var parsed = map[string]*template.Template{}

// parsedLocal caches project-local templates by "dir\x00name".
var parsedLocal = map[string]*template.Template{}

// RegisterTemplates wires a bridge's loader into the renderer. Called from
// each bridge's init(): the first arg is the template name (the filename
// inside the bridge's templates/ directory), the second is the read
// callback. Re-registering a name panics — name collisions across bridges
// are programmer errors caught at startup.
func RegisterTemplates(name string, load templateLoader) {
	if _, dup := loaders[name]; dup {
		panic("bridge: template " + name + " registered twice")
	}
	loaders[name] = load
}

// RegisterTemplateTree registers every *.tmpl directly under dir in a
// bridge's embedded FS, keyed by base name. Convention over configuration:
// shipping a new locale (CONTRIBUTING.uk.md.tmpl) becomes a one-file change
// with no register.go edit, and the set of installed translations is exactly
// the set of files on disk.
//
// Nested directories are skipped — the README bridge's per-block tree reads
// its own FS directly and must not collide with these flat names.
func RegisterTemplateTree(fsys interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
}, dir string,
) {
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		panic("bridge: read template dir " + dir + ": " + err.Error())
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".tmpl") {
			continue
		}
		RegisterTemplates(name, func(n string) ([]byte, error) {
			return fsys.ReadFile(dir + "/" + n)
		})
	}
}

// localCacheKey builds the cache key for a project-local template.
func localCacheKey(dir, name string) string {
	return dir + "\x00" + name
}

// lookup returns the parsed template for (dir, name), parsing on first use
// and caching the result. When dir is non-empty and a file exists at
// <dir>/.projectfile/templates/<name>, it takes precedence over the
// embedded template.
func lookup(dir, name string) (*template.Template, error) {
	// Project-local override: scoped under dir via os.Root (see
	// ReadLocalTemplate) so a name containing ".." can't traverse out.
	if dir != "" {
		rel := filepath.Join(LocalTemplatesDir, name)
		if body, ok, err := ReadLocalTemplate(dir, rel); err != nil {
			return nil, err
		} else if ok {
			key := localCacheKey(dir, name)
			parsedMu.RLock()
			tpl, cached := parsedLocal[key]
			parsedMu.RUnlock()
			if cached {
				return tpl, nil
			}
			tpl, err = template.New(name).Option("missingkey=zero").Parse(string(body))
			if err != nil {
				return nil, fmt.Errorf("bridge: parse local %s: %w", name, err)
			}
			parsedMu.Lock()
			parsedLocal[key] = tpl
			parsedMu.Unlock()
			return tpl, nil
		}
	}

	// Embedded (registered) template — cached globally by name.
	parsedMu.RLock()
	tpl, ok := parsed[name]
	parsedMu.RUnlock()
	if ok {
		return tpl, nil
	}
	load, ok := loaders[name]
	if !ok {
		return nil, fmt.Errorf("bridge: template %s not registered", name)
	}
	body, err := load(name)
	if err != nil {
		return nil, fmt.Errorf("bridge: read %s: %w", name, err)
	}
	tpl, err = template.New(name).Option("missingkey=zero").Parse(string(body))
	if err != nil {
		return nil, fmt.Errorf("bridge: parse %s: %w", name, err)
	}
	parsedMu.Lock()
	parsed[name] = tpl
	parsedMu.Unlock()
	return tpl, nil
}
