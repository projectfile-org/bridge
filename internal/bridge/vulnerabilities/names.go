// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package vulnerabilities

import (
	"sync"

	"golang.org/x/sync/errgroup"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/advisory"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// lookup resolves one advisory title; tests swap it for a fake.
var lookup = advisory.Name

// resolved memoises titles across the four scanner renders of one run; offline marks a failed lookup so the rest read the local cache only.
var resolved = struct {
	sync.Mutex
	names   map[string]string
	offline bool
}{names: map[string]string{}}

// lookupWorkers bounds the concurrent osv.dev requests of one render.
var lookupWorkers = 8

// fillNames gives every unnamed entry its osv.dev title, caching it in the base projectfile on a writing run.
func fillNames(entries []pfmodel.VulnerabilitySuppress, opts core.Options) []pfmodel.VulnerabilitySuppress {
	resolved.Lock()
	defer resolved.Unlock()
	resolveNames(entries, opts.Offline)
	found := map[string]string{}
	for i, e := range entries {
		if name := resolved.names[e.ID]; e.Name == "" && name != "" {
			entries[i].Name = name
			found[e.ID] = name
		}
	}
	if opts.Check || opts.DryRun || opts.PFPath == "" {
		genlog.Debug("vulnerability names: projectfile left as is", "found", len(found), "check", opts.Check, "dry-run", opts.DryRun)
		return entries
	}
	storeNames(opts.PFPath, found)
	return entries
}

// resolveNames looks every unnamed, unmemoised ID up, lookupWorkers at a time; after one failure the rest read the local cache only.
func resolveNames(entries []pfmodel.VulnerabilitySuppress, offline bool) {
	var mu sync.Mutex
	var g errgroup.Group
	g.SetLimit(lookupWorkers)
	for _, e := range entries {
		mu.Lock()
		_, seen := resolved.names[e.ID]
		if !seen {
			resolved.names[e.ID] = ""
		}
		mu.Unlock()
		if seen || e.Name != "" || advisory.Key(e.ID) == "" {
			continue
		}
		g.Go(func() error {
			mu.Lock()
			off := offline || resolved.offline
			mu.Unlock()
			name, err := lookup(e.ID, off)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				genlog.Warn("vulnerability names: osv.dev unreachable, the rest read the local cache only", "id", e.ID, "error", err.Error())
				resolved.offline = true
				return nil
			}
			resolved.names[e.ID] = name
			return nil
		})
	}
	_ = g.Wait()
}

// storeNames writes found titles onto the base projectfile’s own suppress entries; include-inherited entries stay where they are declared.
func storeNames(pfPath string, found map[string]string) {
	if len(found) == 0 {
		return
	}
	base, err := projectfile.ReadBaseFromPath(pfPath)
	if err != nil {
		genlog.Warn("vulnerability names: base projectfile unreadable, names not cached", "path", pfPath, "error", err.Error())
		return
	}
	raw, _ := projectfile.LookupExtension(base, pfmodel.VulnerabilitiesExtensionNS)
	ns, _ := raw.(map[string]any)
	list, _ := ns["suppress"].([]any)
	stored := 0
	for i, item := range list {
		switch v := item.(type) {
		case string:
			if name, ok := found[v]; ok {
				list[i] = map[string]any{"id": v, "name": name}
				stored++
			}
		case map[string]any:
			id, _ := v["id"].(string)
			if name, ok := found[id]; ok && v["name"] == nil {
				v["name"] = name
				stored++
			}
		}
	}
	genlog.Debug("vulnerability names: cached in projectfile", "path", pfPath, "stored", stored, "found", len(found))
	if stored == 0 {
		return
	}
	if err := projectfile.Write(base, pfPath); err != nil {
		genlog.Warn("vulnerability names: projectfile write failed", "path", pfPath, "error", err.Error())
	}
}
