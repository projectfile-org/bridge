// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package scancmd is the pf-bridge-scan binary's command: run scanners to
// refresh projectfile metadata from filesystem/git signals.
package scancmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/pflock"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/selector"
	"projectfile.org/projectfile/bridge/internal/buildinfo"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
	"projectfile.org/projectfile/bridge/internal/rootflags"
	"projectfile.org/projectfile/bridge/internal/scanners/core"
	stackscan "projectfile.org/projectfile/bridge/internal/scanners/stack"
	"projectfile.org/projectfile/bridge/internal/source"

	// Blank-imported aggregator pulls every scanner driver's init() into the
	// build so they self-register with core. stackscan above is also imported
	// non-blank because runStacksPhase calls its Scan/Diff/Union functions.
	_ "projectfile.org/projectfile/bridge/internal/scanners/all"
)

// cmdAll is the "run everything" composite name, shared by the picker and the
// name expander.
const cmdAll = "all"

// scanCmd is the single entry point for all scanning. Without arguments it
// opens an interactive picker listing every registered scanner plus "all"
// and "git" composites. With positional args it runs the named scanners:
//
//	pf-cli scan all                         → every registered scanner
//	pf-cli scan git                         → git-authors + git-remotes + git-dates
//	pf-cli scan git-remotes stacks          → specific scanners
//	pf-cli scan git-remotes stacks ./proj   → last non-scanner arg is the directory
//
// Stack-specific flags (--check/--strict/--prune) only take effect when
// "stacks" is in the scanner list.
var scanCmd = &cobra.Command{
	Use:   "scan [scanner...] [directory]",
	Short: "Run scanners to refresh projectfile metadata from filesystem signals",
	Long: "Without arguments, scan opens an interactive picker.\n" +
		"Available scanners: all, git, git-authors, git-remotes, git-dates, stacks.\n" +
		"Combine multiple: pf-cli scan git-remotes stacks\n" +
		"\"git\" expands to git-authors + git-remotes + git-dates.\n" +
		"\"all\" runs every registered scanner.\n" +
		"\n" +
		"Stack-specific flags (--check/--strict/--prune) only apply when\n" +
		"\"stacks\" is in the scanner list.",
	Version:       buildinfo.Version,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ArbitraryArgs,
	RunE: func(_ *cobra.Command, args []string) error {
		names, dir := parseScanArgs(args)
		if len(names) == 0 {
			return runScanPicker(dir)
		}
		expanded := expandScannerNames(names)
		pfPath, err := projectfile.DetectPath(dir)
		if err != nil {
			return fmt.Errorf("detect projectfile: %w", err)
		}
		return pflock.WithLock(pfPath, func() error {
			return runScanNamed(dir, pfPath, expanded)
		})
	},
}

// knownScannerNames is the set of user-facing scanner names accepted on the
// CLI. Registry names ("git-authors", "git-remotes", "git-dates", "stacks")
// are listed directly; "all" and "git" are composites expanded at parse time.
const (
	scannerGit        = "git"
	scannerGitAuthors = "git-authors"
	scannerGitRemotes = "git-remotes"
	scannerGitDates   = "git-dates"
	scannerStacks     = "stacks"
)

var knownScannerNames = map[string]bool{
	cmdAll:            true,
	scannerGit:        true,
	scannerGitAuthors: true,
	scannerGitRemotes: true,
	scannerGitDates:   true,
	scannerStacks:     true,
}

// parseScanArgs splits positional args into scanner names and an optional
// directory. Any arg matching a known scanner name is a scanner; all others
// are treated as the directory (at most one).
func parseScanArgs(args []string) (scannerNames []string, dir string) {
	dir = "."
	for _, arg := range args {
		if knownScannerNames[arg] {
			scannerNames = append(scannerNames, arg)
		} else {
			dir = arg
		}
	}
	return scannerNames, dir
}

// expandScannerNames resolves composite names ("all", "git") into concrete
// scanner names from the registry. Order is preserved; duplicates removed.
func expandScannerNames(names []string) []string {
	var expanded []string
	seen := map[string]bool{}
	for _, name := range names {
		switch name {
		case cmdAll:
			for _, s := range core.List() {
				n := s.Name()
				if !seen[n] {
					expanded = append(expanded, n)
					seen[n] = true
				}
			}
		case scannerGit:
			for _, sub := range []string{scannerGitAuthors, scannerGitRemotes, scannerGitDates} {
				if !seen[sub] {
					expanded = append(expanded, sub)
					seen[sub] = true
				}
			}
		default:
			if !seen[name] {
				expanded = append(expanded, name)
				seen[name] = true
			}
		}
	}
	return expanded
}

// runScanNamed dispatches the resolved scanner names. Stacks is handled
// separately for its --check/--strict/--prune semantics; everything else
// goes through the generic applyPartialToDoc path.
func runScanNamed(dir, pfPath string, names []string) error {
	if scanStrict && scanPrune {
		return fmt.Errorf("--strict and --prune are mutually exclusive")
	}

	var hasStacks bool
	var otherNames []string
	for _, n := range names {
		if n == scannerStacks {
			hasStacks = true
		} else {
			otherNames = append(otherNames, n)
		}
	}

	if hasStacks {
		if err := runStacksPhase(dir, pfPath); err != nil {
			return err
		}
	}

	if len(otherNames) > 0 {
		if err := runGenericPhase(dir, pfPath, otherNames); err != nil {
			return err
		}
	}

	return nil
}

// runGenericPhase runs non-stacks scanners and merges results via
// applyPartialToDoc.
func runGenericPhase(dir, pfPath string, names []string) error {
	effectivePF, _, err := projectfile.ReadWithOptions(dir, rootflags.ReadOpts())
	if err != nil {
		return fmt.Errorf("read projectfile: %w", err)
	}
	pf, _, err := projectfile.ReadBase(dir)
	if err != nil {
		return fmt.Errorf("read base projectfile: %w", err)
	}
	partial, hits, scanErr := core.RunNamed(dir, names)
	if scanErr != nil {
		genlog.Warn("scanner failures", "err", scanErr.Error())
	}
	logHits(hits)
	applyPartialToDoc(pf, effectivePF, partial)
	if scanDryRun {
		genlog.Plain("DRY RUN — projectfile not modified")
		return nil
	}
	return projectfile.Write(pf, pfPath)
}

// runStacksPhase handles the stacks scanner with its special merge semantics
// (Diff/Union/check/prune). Reads and writes the projectfile independently
// within the caller's lock.
func runStacksPhase(dir, pfPath string) error {
	effectivePF, _, err := projectfile.ReadWithOptions(dir, rootflags.ReadOpts())
	if err != nil {
		return fmt.Errorf("read projectfile: %w", err)
	}
	basePF, _, err := projectfile.ReadBase(dir)
	if err != nil {
		return fmt.Errorf("read base projectfile: %w", err)
	}

	detected, hits, err := stackscan.Scan(dir)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	for _, h := range hits {
		genlog.Info("marker", "path", h.Path, "tags", h.Tags)
	}

	added := stackscan.Diff(detected, effectivePF.Stack)
	stale := stackscan.Diff(effectivePF.Stack, detected)

	if scanCheck || scanStrict || scanPrune {
		for _, t := range stale {
			genlog.Warn("stale stack tag (no filesystem evidence)", "tag", t)
		}
	}

	if scanPrune && len(stale) > 0 {
		newStack := stackscan.Diff(basePF.Stack, stale)
		removed := stackscan.Diff(basePF.Stack, newStack)
		basePF.Stack = newStack
		if len(removed) > 0 {
			genlog.Info("removed stale stack tags", "tags", removed)
		}
		if scanDryRun {
			genlog.Plain("DRY RUN — projectfile not modified")
			return nil
		}
		if err := projectfile.Write(basePF, pfPath); err != nil {
			return err
		}
	}

	if len(added) == 0 {
		genlog.Info("no stack additions")
	} else {
		genlog.Info("added stack tags", "tags", added)
	}

	if !scanCheck && !scanPrune && len(added) > 0 {
		if scanDryRun {
			genlog.Plain("DRY RUN — projectfile not modified")
			return nil
		}
		basePF.Stack = stackscan.Union(basePF.Stack, added)
		if err := projectfile.Write(basePF, pfPath); err != nil {
			return err
		}
	}

	if scanStrict && len(stale) > 0 {
		return fmt.Errorf("%d stale tag(s) in [technologies]", len(stale))
	}
	return nil
}

// interactive picker --------------------------------------------------------

type scanChoice struct {
	name string
	desc string
}

func runScanPicker(dir string) error {
	items := []scanChoice{
		{cmdAll, "run every applicable scanner (gap-fill)"},
		{scannerGit, "commit log → authors, origin URL, root-commit date"},
		{scannerGitAuthors, "commit log → authors"},
		{scannerGitRemotes, "git remotes → repositories, source-code links"},
		{scannerGitDates, "commit dates → identity.created, identity.modified"},
		{scannerStacks, "filesystem markers → stack tags (merge/check/prune)"},
	}
	chosen, err := selector.Run(selector.Choices[scanChoice]{
		Title:  "What to scan?",
		Items:  items,
		Label:  func(c scanChoice) string { return c.name },
		Detail: func(c scanChoice) string { return c.desc },
	})
	if err != nil {
		if errors.Is(err, selector.ErrCancelled) {
			return fmt.Errorf("cancelled")
		}
		return err
	}

	expanded := expandScannerNames([]string{chosen.name})
	pfPath, err := projectfile.DetectPath(dir)
	if err != nil {
		return fmt.Errorf("detect projectfile: %w", err)
	}
	return pflock.WithLock(pfPath, func() error {
		return runScanNamed(dir, pfPath, expanded)
	})
}

// shared helpers -----------------------------------------------------------

func logHits(hits []core.Hit) {
	for _, h := range hits {
		genlog.Info("scanner", "name", h.Source, "field", h.Field)
	}
}

// applyPartialToDoc gap-fills scanner output into an existing projectfile
// document. Mirrors source.MergePartials' "existing wins" rule, but at the
// projectfile.Document level (Partial's pointer-soup and Document's spec
// shape don't share a single type). Only the fields scanners populate today
// are handled — stack, people, repository, created.
//
// effectivePF is the full resolved document (base + includes): its people set
// gates the people dedup so include-inherited people are never re-materialised
// into the base file, its title drives the source-code label rewrite, and its
// i18n declaration decides whether rewritten labels are Bare or a Langs map.
func applyPartialToDoc(doc *projectfile.Document, effectivePF *projectfile.Document, p *source.Partial) {
	if p == nil {
		return
	}
	effectivePeople := effectivePF.People
	effectiveOrgs := effectivePF.Organizations
	effectiveTitle := effectivePF.Identity.Title
	if len(p.Stack) > 0 {
		doc.Stack = stackscan.Union(doc.Stack, p.Stack)
	}
	if len(p.People) > 0 {
		incomingPeople, incomingOrgs := partialPeopleToProjectfile(p.People)
		if len(incomingPeople) > 0 {
			withEffective, _ := projectfile.MergePeople(effectivePeople, incomingPeople)
			newOnly := withEffective[len(effectivePeople):]
			if len(newOnly) > 0 {
				doc.People, _ = projectfile.MergePeople(doc.People, newOnly)
			}
		}
		if len(incomingOrgs) > 0 {
			withEffective, _ := projectfile.MergeOrganizations(effectiveOrgs, incomingOrgs)
			newOnly := withEffective[len(effectiveOrgs):]
			if len(newOnly) > 0 {
				doc.Organizations, _ = projectfile.MergeOrganizations(doc.Organizations, newOnly)
			}
		}
	}
	for _, r := range p.Repositories {
		applyRepository(doc, r)
	}
	// Guarantee exactly one primary repository after merging scanner output.
	if len(doc.Repositories) > 0 {
		ensurePrimaryRepository(doc.Repositories)
	}
	for _, l := range p.Links {
		applyLink(doc, l, scanForce)
	}
	// Promote scanner-generated source-code labels from the "Source Code on X"
	// noun placeholder to "{title} on X", so the label carries the project
	// identity rather than a generic description. Localized: the placeholder
	// and the promotion both follow the declared languages. Only links still
	// carrying the placeholder are touched — a curated or already-promoted
	// label is left alone (PromoteSourceCodeLabel returns nil).
	if effectiveTitle != nil {
		for i := range doc.Links {
			l := &doc.Links[i]
			if l.Type != projectfile.LinkSourceCode {
				continue
			}
			if promoted := pfmodel.PromoteSourceCodeLabel(effectivePF, l.Label, effectiveTitle); promoted != nil {
				l.Label = promoted
			}
		}
	}
	// Gap-fill the `support` tag onto the issues tracker so SUPPORT.md's
	// "Before You Ask" lists it by default. SetLinkTags gap-fills (a curated
	// tags key, even tags: [], is left alone); scanForce overrides. The issues
	// tracker is the one link a reader should check before asking.
	for i := range doc.Links {
		l := &doc.Links[i]
		if l.Type == projectfile.LinkBugs {
			pfmodel.SetLinkTags(l, []string{pfmodel.TagSupport}, scanForce)
		}
	}
	if p.Created != nil && *p.Created != "" && doc.Identity.Created == "" {
		doc.Identity.Created = *p.Created
	}
	// Gap-fill identity.modified from the latest-commit date. Guarded on empty
	// so a curated value is never clobbered (HEAD's date churns every commit).
	if p.Modified != nil && *p.Modified != "" && doc.Identity.Modified == "" {
		doc.Identity.Modified = *p.Modified
	}
}

// applyRepository gap-fills a scanner-supplied repository entry into the
// document. If the URL is already listed, only branch/type/role are
// filled where empty; otherwise the entry is appended. Same-URL collisions
// across sources are common (npm's `repository.url` and the git scanner's
// origin remote both point at the same forge), so this is the merge point.
func applyRepository(doc *projectfile.Document, repo projectfile.Repository) {
	for i := range doc.Repositories {
		existing := &doc.Repositories[i]
		if existing.URL != repo.URL {
			continue
		}
		if existing.Role != projectfile.RepositoryRoleOrigin && repo.Role == projectfile.RepositoryRoleOrigin {
			existing.Role = projectfile.RepositoryRoleOrigin
		}
		if existing.Role == "" {
			existing.Role = repo.Role
		}
		if existing.Branch == "" {
			existing.Branch = repo.Branch
		}
		if existing.Type == "" {
			existing.Type = repo.Type
		}
		if existing.Path == "" {
			existing.Path = repo.Path
		}
		return
	}
	doc.Repositories = append(doc.Repositories, repo)
}

// applyLink gap-fills a scanner-supplied link entry into the document. The
// dedup key is (type, url) so multiple links of the same type with distinct
// URLs (e.g. two source-code links for origin + mirror) coexist; an identical
// pair from a second source is a no-op. When the existing link lacks a field
// the incoming one carries (label, preferred, tags), it is gap-filled —
// existing values win, so a manually set preferred:true is never downgraded
// and a curated capability list is never overruled.
//
// force applies to the capability tags ONLY. It is how a fleet that already
// wrote one tag vocabulary can be moved to the next one; label and preferred
// stay gap-fill, because nothing generates a better label than the user.
func applyLink(doc *projectfile.Document, link projectfile.Link, force bool) {
	for i := range doc.Links {
		existing := &doc.Links[i]
		if existing.Type == link.Type && existing.URL == link.URL {
			if existing.Label == nil && link.Label != nil {
				existing.Label = link.Label
			}
			if !existing.Preferred && link.Preferred {
				existing.Preferred = link.Preferred
			}
			if tags := pfmodel.LinkTags(link); pfmodel.SetLinkTags(existing, tags, force) {
				genlog.Info("scan: capability tags written", "url", existing.URL,
					"tags", tags, "force", force)
			}
			return
		}
	}
	doc.Links = append(doc.Links, link)
}

func partialPeopleToProjectfile(in []source.PersonEntry) ([]projectfile.Person, []projectfile.Organization) {
	var people []projectfile.Person
	var orgs []projectfile.Organization
	for _, p := range in {
		if p.FamilyNames == "" && p.Name != "" {
			orgs = append(orgs, projectfile.Organization{
				Name:  p.Name,
				Email: p.Email,
				Orcid: p.Orcid,
				URL:   p.URL,
				Roles: p.Roles,
			})
		} else {
			people = append(people, projectfile.Person{
				FamilyNames: p.FamilyNames,
				GivenNames:  p.GivenNames,
				Email:       p.Email,
				Orcid:       p.Orcid,
				URL:         p.URL,
				Roles:       p.Roles,
				Affiliation: p.Affiliation,
			})
		}
	}
	return people, orgs
}

// ensurePrimaryRepository guarantees exactly one repository is marked origin.
// If an entry already has role == origin it wins; otherwise the first entry is
// promoted. Keeps scan output valid even when multiple remotes exist with no
// clear origin.
func ensurePrimaryRepository(repos []projectfile.Repository) {
	for _, r := range repos {
		if r.Role == projectfile.RepositoryRoleOrigin {
			return
		}
	}
	repos[0].Role = projectfile.RepositoryRoleOrigin
}

var (
	scanDryRun bool
	scanCheck  bool
	scanStrict bool
	scanPrune  bool
	scanForce  bool
)

// Main is the pf-bridge-scan entry point.
func Main(binName string) {
	scanCmd.Use = binName + " [scanner...] [directory]"
	scanCmd.Flags().BoolVarP(&scanDryRun, "dry-run", "n", false,
		"preview without writing the projectfile")
	scanCmd.Flags().BoolVar(&scanCheck, "check", false,
		"warn about tags in [technologies] with no filesystem evidence (read-only, stacks only)")
	scanCmd.Flags().BoolVar(&scanStrict, "strict", false,
		"--check, but stale tags exit non-zero (stacks only)")
	scanCmd.Flags().BoolVar(&scanPrune, "prune", false,
		"remove stale tags from pf in place (mutually exclusive with --strict, stacks only)")
	scanCmd.Flags().BoolVarP(&scanForce, "force", "f", false,
		"overwrite a declared links[].tags with the host proposal (links only; label and preferred stay gap-fill)")
	rootflags.Bind(scanCmd)

	if err := scanCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
