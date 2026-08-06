// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"fmt"
	"io"
	"os"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/derive"
)

// RunSync drives a single round-trip between projectfile and the external
// file owned by syn. Algorithm:
//
//  1. Resolve the effective mode. Explicit --to / --from bypass mtime
//     arbitration; default ModeSync picks the newer file as authoritative.
//  2. If the external file does not exist, push every mapper's FromPF onto
//     a fresh extDoc and mark Result.Created.
//  3. With both files present, run the authoritative direction with
//     force=true, then the reverse direction with force=false (gap-fill).
//  4. Surface PersonConflicts to opts.Stderr.
//  5. Persist via syn.Write / projectfile.Write only when !opts.DryRun.
//
// Dry-run safety: work documents are cloned, so the caller's pf and extDoc
// are never mutated.
func RunSync(syn Syncer, pf *projectfile.Document, opts Options) (*Result, error) {
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	extName, pfName := syn.Labels()
	res := &Result{
		DryRun:  opts.DryRun,
		ExtName: extName,
		PFName:  pfName,
	}

	if !syn.Exists(opts.Dir) {
		if opts.NoCreate {
			return nil, fmt.Errorf("%s does not exist and --no-create is set", syn.Filename())
		}
		return syncCreate(syn, pf, opts, res, stderr)
	}

	extDoc, err := syn.Read(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", syn.Filename(), err)
	}

	mode, err := resolveMode(opts, syn.FullPath(opts.Dir, pf))
	if err != nil {
		return nil, err
	}

	workExt, workPF := extDoc, pf
	// preSync snapshots the merged doc before mappers mutate it.  After sync
	// we ReconcileBase(basePF, preSync, workPF) so only the fields the sync
	// actually touched land in the base file — include-inherited values are
	// never materialised.  In dry-run mode the clone IS the work doc, so the
	// snapshot is implicit and we skip the write entirely.
	var preSync *projectfile.Document
	if opts.DryRun {
		workExt = syn.Clone(extDoc)
		workPF = pf.Clone()
	} else {
		preSync = pf.Clone()
	}

	mappers := syn.BuildMappers(workExt, workPF)

	switch mode {
	case ModeWrite:
		runFromPF(mappers, true, res, extName, pfName)
	case ModeRead:
		runToPF(mappers, true, res, extName, pfName)
	case ModeSync:
		return nil, fmt.Errorf("internal: sync mode not resolved")
	case modePFAuthoritative:
		runFromPF(mappers, true, res, extName, pfName)
		runToPF(mappers, false, res, extName, pfName) // gap-fill PF from ext
	case modeExtAuthoritative:
		runToPF(mappers, true, res, extName, pfName)
		runFromPF(mappers, false, res, extName, pfName) // gap-fill ext from PF
	}

	emitConflicts(workPF, stderr, extName)

	// Derivation pass runs after conflict emission so the engine sees the
	// final repository/stack state for THIS sync run. Changes are written
	// straight into workPF; they ride out alongside the bridge writes
	// below in the same opts.DryRun gate.
	derivedChanges, err := derive.Apply(workPF, derive.Options{})
	if err != nil {
		return nil, fmt.Errorf("derive: %w", err)
	}
	for _, c := range derivedChanges {
		res.PFFields = append(res.PFFields, FieldChange{Key: c.FieldPath, Value: c.NewValue, From: c.Source, To: "projectfile"})
		res.PFChanged = true
	}

	if !opts.DryRun {
		if res.ExtChanged {
			if err := syn.Write(opts.Dir, workExt); err != nil {
				return nil, fmt.Errorf("write %s: %w", syn.Filename(), err)
			}
		}
		if res.PFChanged {
			// Write the BASE document (no includes resolved) with only the
			// fields the sync mutated, so include data is never materialised
			// into the on-disk projectfile.
			basePF, err := projectfile.ReadBaseFromPath(opts.PFPath)
			if err != nil {
				return nil, fmt.Errorf("read base for reconcile: %w", err)
			}
			projectfile.ReconcileBase(basePF, preSync, workPF)
			if err := projectfile.Write(basePF, opts.PFPath); err != nil {
				return nil, fmt.Errorf("write projectfile: %w", err)
			}
		}
	}

	return res, nil
}

// Internal sentinels — ModeSync resolves into one of these before the
// switch dispatches.
const (
	modePFAuthoritative  Mode = "_pf-authoritative"
	modeExtAuthoritative Mode = "_ext-authoritative"
)

// syncCreate handles the "external file does not exist" branch.
func syncCreate(syn Syncer, pf *projectfile.Document, opts Options, res *Result, stderr io.Writer) (*Result, error) {
	workPF := pf
	if opts.DryRun {
		workPF = pf.Clone()
	}
	extName, pfName := syn.Labels()
	extDoc := syn.NewEmpty()
	mappers := syn.BuildMappers(extDoc, workPF)

	for _, m := range mappers {
		if desc := m.FromPF(true); desc != "" {
			res.ExtFields = append(res.ExtFields, FieldChange{m.ExtKey, desc, pfName, extName})
		}
	}
	res.ExtChanged = true
	res.Created = true
	_ = stderr // create-path has no existing ext doc to merge against — no PersonConflicts possible.

	if !opts.DryRun {
		if err := syn.Write(opts.Dir, extDoc); err != nil {
			return nil, fmt.Errorf("write %s: %w", syn.Filename(), err)
		}
	}
	return res, nil
}

func runFromPF(mappers MapperList, force bool, res *Result, extName, pfName string) {
	for _, m := range mappers {
		if m.FromPF == nil {
			continue
		}
		if desc := m.FromPF(force); desc != "" {
			res.ExtFields = append(res.ExtFields, FieldChange{m.ExtKey, desc, pfName, extName})
			res.ExtChanged = true
		}
	}
}

func runToPF(mappers MapperList, force bool, res *Result, extName, pfName string) {
	for _, m := range mappers {
		if m.ToPF == nil {
			continue
		}
		if desc := m.ToPF(force); desc != "" {
			res.PFFields = append(res.PFFields, FieldChange{m.PFKey, desc, extName, pfName})
			res.PFChanged = true
		}
	}
}

func resolveMode(opts Options, extPath string) (Mode, error) {
	switch opts.Mode {
	case ModeWrite, ModeRead:
		return opts.Mode, nil
	case ModeSync, "":
		pfMod, err := fileModTime(opts.PFPath)
		if err != nil {
			return "", fmt.Errorf("stat projectfile: %w", err)
		}
		extMod, err := fileModTime(extPath)
		if err != nil {
			return "", fmt.Errorf("stat %s: %w", extPath, err)
		}
		if pfMod >= extMod {
			return modePFAuthoritative, nil
		}
		return modeExtAuthoritative, nil
	default:
		return "", fmt.Errorf("unknown sync mode %q", opts.Mode)
	}
}

func fileModTime(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.ModTime().UnixMilli(), nil
}

// emitConflicts pulls any PersonConflict records the people-mapper recorded
// on the work projectfile and writes one warning line per conflict.
const conflictsRestKey = "_pf_sync_person_conflicts"

func emitConflicts(workPF *projectfile.Document, stderr io.Writer, extName string) {
	if workPF == nil || workPF.Rest == nil {
		return
	}
	raw, ok := workPF.Rest[conflictsRestKey]
	if !ok {
		return
	}
	delete(workPF.Rest, conflictsRestKey)
	if len(workPF.Rest) == 0 {
		workPF.Rest = nil
	}
	conflicts, ok := raw.([]projectfile.PersonConflict)
	if !ok {
		return
	}
	for _, c := range conflicts {
		fmt.Fprintf(stderr,
			"warning: %s person %q has conflicting %s (existing %q vs incoming %q from %s); keeping existing\n",
			"projectfile", c.IdentityKey, c.Field, c.Existing, c.Incoming, extName)
		genlog.Warn("person conflict — keeping existing",
			"identity", c.IdentityKey,
			"field", c.Field,
			"existing", c.Existing,
			"incoming", c.Incoming,
			"source", extName)
	}
}

// RecordPersonConflicts is a helper the mapper closures call after running
// projectfile.MergePeople. It stashes conflicts on pf.Rest for RunSync to
// pick up; this keeps the FieldMapper signature simple.
func RecordPersonConflicts(pf *projectfile.Document, conflicts []projectfile.PersonConflict) {
	if len(conflicts) == 0 {
		return
	}
	if pf.Rest == nil {
		pf.Rest = map[string]any{}
	}
	prev, _ := pf.Rest[conflictsRestKey].([]projectfile.PersonConflict)
	pf.Rest[conflictsRestKey] = append(prev, conflicts...)
}
