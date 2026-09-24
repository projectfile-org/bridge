// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package scaffold

import (
	"fmt"
	"path/filepath"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/pflock"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
	"projectfile.org/projectfile/bridge/internal/scanners/core"
	"projectfile.org/projectfile/bridge/internal/source"

	// Blank-imported aggregator pulls every scanner driver's init() into the
	// build so they self-register with core. Adding a new scanner is a single
	// line in scanners/all, not one per consumer.
	_ "projectfile.org/projectfile/bridge/internal/scanners/all"
)

// formatYAML / formatTOML are the canonical names for the supported
// serialization formats. formatTOML is still referenced by the per-format
// write dispatch in projectfile/write.go's discriminator helper; formatYAML
// drives the picker label and the non-interactive default below.
const (
	formatYAML = "yaml"
	formatTOML = "toml"
)

// defaultFormat is the encoding used when --format is omitted and the run is
// non-interactive. YAML per spec §4.1 RECOMMENDED.
const defaultFormat = formatYAML

type Options struct {
	Dir            string
	Format         string
	NonInteractive bool
	Namespace      string
	Name           string
	License        string
	// NoScan disables the init-time scanner pass (git history, stack
	// detection). Source files (package.json etc.) still run regardless.
	NoScan bool
}

func Run(opts Options) error {
	dir := opts.Dir

	if _, err := projectfile.DetectPath(dir); err == nil {
		return fmt.Errorf("projectfile already exists in %s", dir)
	}

	// User-level defaults consulted once up-front. Each below-call uses ucfg
	// as a fallback layer — opts (CLI flags) > sources > scanners > ucfg >
	// hard-coded built-ins. Loading is cached, so a second Load() is free.
	ucfg := userconfig.Load()
	// init.no_scan: if the user habitually skips scanners, honor that
	// preference. The CLI flag --no-scan can only turn scanning OFF today
	// (default false), so a user-config "true" + no flag = skip; opt out by
	// removing the setting.
	if !opts.NoScan && ucfg.Init.NoScan {
		opts.NoScan = true
		genlog.Info("scaffold: no_scan from user config")
	}

	partial, sourceNames, err := source.ExtractAll(dir)
	if err != nil {
		return fmt.Errorf("discover sources: %w", err)
	}

	for _, name := range sourceNames {
		genlog.Debug("source detected", "name", name)
	}

	// Scanners run after sources and gap-fill — MergePartials uses
	// "existing wins" semantics so a value already supplied by a
	// declarative source (e.g. composer's author list) is never overwritten
	// by the git scanner's commit-derived guess.
	if !opts.NoScan {
		scanned, hits, scanErr := core.RunAll(dir)
		if scanErr != nil {
			// Scanner failures are advisory: log + continue. Init has already
			// extracted whatever sources are available; aborting on an
			// optional gap-fill failure punishes the user for no reason.
			genlog.Warn("scanner failures", "err", scanErr.Error())
		}
		for _, h := range hits {
			genlog.Debug("scanner", "name", h.Source, "field", h.Field)
		}
		partial = source.MergePartials(partial, scanned)
	}

	// Identity auto-attach: splice [identity] fields (orcid, url) into any
	// person whose email matches [identity].email. Runs after MergePartials
	// so the full merged person list is in scope; runs before generic
	// gap-fills so a person's ORCID is set before any downstream consumer
	// reads it.
	source.ApplyUserIdentity(partial)

	// User-config defaults are applied AFTER sources/scanners so detected
	// values win, and BEFORE CLI overrides so explicit --namespace/--license
	// still wins. Each guard checks "still empty" so we never trample a real
	// value.
	//
	// Namespace is special: in interactive runs we present the ucfg default
	// as an editable prompt prefill rather than silently adopting it — a
	// personal namespace ("me.foo") is rarely the right value for a fresh
	// project ("org.example"). The Partial stays empty here so findMissing
	// queues the prompt; the default flows in via missingDefaults below.
	// Non-interactive runs preserve the silent-apply behaviour so existing
	// scripted invocations don't suddenly require --namespace.
	if opts.NonInteractive {
		if (partial.Namespace == nil || *partial.Namespace == "") && ucfg.Init.NamespaceDefault != "" {
			partial.Namespace = source.StringPtr(ucfg.Init.NamespaceDefault)
			genlog.Info("scaffold: namespace from user config", "value", ucfg.Init.NamespaceDefault)
		}
	}
	if (partial.License == nil || *partial.License == "") && ucfg.Init.LicenseDefault != "" {
		partial.License = source.StringPtr(ucfg.Init.LicenseDefault)
		genlog.Info("scaffold: license from user config", "value", ucfg.Init.LicenseDefault)
	}

	if opts.Namespace != "" {
		partial.Namespace = source.StringPtr(opts.Namespace)
	}
	if opts.Name != "" {
		partial.Name = source.StringPtr(opts.Name)
	}
	if opts.License != "" {
		partial.License = source.StringPtr(opts.License)
	}

	// Directory-name suggestion: the basename is overwhelmingly the right
	// slug. Scripted runs adopt it silently; interactive runs get it as an
	// editable prompt prefill so identity.name is always asked directly.
	dirBase := ""
	if abs, absErr := filepath.Abs(dir); absErr == nil {
		dirBase = filepath.Base(abs)
	}
	if opts.NonInteractive && (partial.Name == nil || *partial.Name == "") && dirBase != "" {
		partial.Name = source.StringPtr(dirBase)
		genlog.Info("scaffold: name from directory", "value", dirBase)
	}

	format := opts.Format
	if format == "" && ucfg.Init.Format != "" {
		format = ucfg.Init.Format
		genlog.Info("scaffold: format from user config", "value", format)
	}
	if format == "" {
		if opts.NonInteractive {
			format = defaultFormat
		} else {
			format, err = promptFormat()
			if err != nil {
				return fmt.Errorf("format selection: %w", err)
			}
		}
	}

	if err := validateFormat(format); err != nil {
		return err
	}

	missing := findMissing(partial, missingDefaults{
		namespace: ucfg.Init.NamespaceDefault,
		name:      dirBase,
	})
	if len(missing) > 0 {
		if opts.NonInteractive {
			var names []string
			for _, f := range missing {
				names = append(names, f.Label)
			}
			return fmt.Errorf("missing required fields (use interactively to provide): %s", strings.Join(names, ", "))
		}

		if err := promptMissingFields(partial, missing); err != nil {
			return fmt.Errorf("collect fields: %w", err)
		}
	}

	if !opts.NonInteractive {
		if err := promptOptionalFields(partial); err != nil {
			return fmt.Errorf("optional fields: %w", err)
		}
	}

	doc := buildDocument(partial)

	path := filepath.Join(dir, "projectfile."+format)

	return pflock.WithLock(path, func() error {
		if err := projectfile.Write(doc, path); err != nil {
			return fmt.Errorf("write projectfile: %w", err)
		}

		fieldCount := countFields(partial)
		sourcesStr := "defaults"
		if len(sourceNames) > 0 {
			sourcesStr = strings.Join(sourceNames, ", ")
		}
		genlog.Success(fmt.Sprintf("created %s (%d fields from %s)", filepath.Base(path), fieldCount, sourcesStr))

		return nil
	})
}

type MissingField struct {
	Label       string
	Description string
	// Default is the pre-filled value shown in the prompt input. The user
	// can accept it with Enter, or edit/replace it before submitting. Empty
	// = blank input. Used to surface ucfg defaults as editable suggestions
	// rather than silent auto-applications.
	Default string
	Setter  func(value string)
}

func findMissing(p *source.Partial, defaults missingDefaults) []MissingField {
	var missing []MissingField

	if p.Namespace == nil || *p.Namespace == "" {
		missing = append(missing, MissingField{
			Label:       "identity.namespace",
			Description: "Reverse-DNS owner handle (e.g. org.example) — who publishes this",
			Default:     defaults.namespace,
			Setter: func(v string) {
				p.Namespace = source.StringPtr(v)
			},
		})
	}

	if p.Name == nil || *p.Name == "" {
		missing = append(missing, MissingField{
			Label:       "identity.name",
			Description: "URL-safe project slug, lowercase with dashes (e.g. my-tool) — not the human-readable title",
			Default:     defaults.name,
			Setter: func(v string) {
				p.Name = source.StringPtr(v)
			},
		})
	}

	return missing
}

// missingDefaults carries prompt prefill values for required fields.
// Threading a typed struct keeps findMissing's signature stable when a
// future field gains its own default.
type missingDefaults struct {
	namespace string
	name      string
}

func buildDocument(p *source.Partial) *projectfile.Document {
	doc := &projectfile.Document{}

	// $schema goes onto the document unconditionally. projectfile.Write hoists
	// it to a `#:schema` header comment for TOML and emits it as a top-level
	// key for YAML/JSON — the format-specific dispatch (and the TOML-only
	// spec_version discriminator) lives there, not here.
	doc.Schema = "https://projectfile.org/schema/v1.json"

	if p.Kind != nil {
		doc.Kind = *p.Kind
	}
	if doc.Kind == "" {
		doc.Kind = "SoftwareSourceCode"
	}

	doc.Identity = projectfile.Identity{}
	if p.Namespace != nil {
		doc.Identity.Namespace = *p.Namespace
	}
	if p.Name != nil {
		doc.Identity.Name = *p.Name
	}
	if p.Version != nil {
		doc.Identity.Version = *p.Version
	}
	if p.Title != nil {
		doc.Identity.Title = &projectfile.LocalizedString{
			Bare: p.Title.Bare,
		}
		if len(p.Title.Langs) > 0 {
			doc.Identity.Title.Langs = p.Title.Langs
		}
	}
	if p.Summary != nil {
		doc.Identity.Summary = &projectfile.LocalizedString{
			Bare: p.Summary.Bare,
		}
		if len(p.Summary.Langs) > 0 {
			doc.Identity.Summary.Langs = p.Summary.Langs
		}
	}
	if len(p.Repositories) > 0 {
		doc.Repositories = append([]projectfile.Repository(nil), p.Repositories...)
	}
	if len(doc.Repositories) > 0 {
		ensurePrimary(doc.Repositories)
	}
	if len(p.Links) > 0 {
		doc.Links = append(doc.Links, p.Links...)
	}
	backfillLinkLabels(doc)

	if p.Created != nil && *p.Created != "" {
		doc.Identity.Created = *p.Created
	}

	if p.Modified != nil && *p.Modified != "" {
		doc.Identity.Modified = *p.Modified
	}

	if p.License != nil && *p.License != "" {
		doc.License = &projectfile.License{
			Spdx: *p.License,
		}
	}

	if len(p.People) > 0 {
		for _, pe := range p.People {
			if pe.FamilyNames == "" && pe.Name != "" {
				doc.Organizations = append(doc.Organizations, projectfile.Organization{
					Name:  pe.Name,
					Email: pe.Email,
					Orcid: pe.Orcid,
					Roles: pe.Roles,
					URL:   pe.URL,
				})
			} else {
				doc.People = append(doc.People, projectfile.Person{
					FamilyNames: pe.FamilyNames,
					GivenNames:  pe.GivenNames,
					Email:       pe.Email,
					Orcid:       pe.Orcid,
					Roles:       pe.Roles,
					URL:         pe.URL,
					Affiliation: pe.Affiliation,
				})
			}
		}
	}

	if len(p.Keywords) > 0 {
		doc.Keywords = p.Keywords
	}
	if len(p.Stack) > 0 {
		doc.Stack = p.Stack
	}

	return doc
}

// ensurePrimary guarantees exactly one repository is marked origin. If an
// entry already has role == origin it wins; otherwise the first entry is
// promoted. This keeps init output valid even when the git scanner
// produced multiple remotes with no clear origin.
func ensurePrimary(repos []projectfile.Repository) {
	for _, r := range repos {
		if r.Role == projectfile.RepositoryRoleOrigin {
			return
		}
	}
	repos[0].Role = projectfile.RepositoryRoleOrigin
}

// linkLabelNoun is the plain-noun label backfillLinkLabels falls back to per link type.
var linkLabelNoun = map[string]string{
	projectfile.LinkBugs:          pfmodel.NounIssues,
	projectfile.LinkHomepage:      pfmodel.NounHomepage,
	projectfile.LinkDocumentation: pfmodel.NounDocumentation,
}

// backfillLinkLabels sets a plain noun label on any link left unlabeled by the source extractors.
func backfillLinkLabels(doc *projectfile.Document) {
	for i := range doc.Links {
		l := &doc.Links[i]
		if l.Label != nil {
			continue
		}
		noun, ok := linkLabelNoun[l.Type]
		if !ok {
			continue
		}
		l.Label = pfmodel.NounLabel(doc, noun)
	}
}

func validateFormat(f string) error {
	switch f {
	case formatYAML, "yml", formatTOML, "json":
		return nil
	default:
		return fmt.Errorf("unsupported format %q (use yaml, toml, or json)", f)
	}
}

func countFields(p *source.Partial) int {
	n := 0
	if p.Namespace != nil && *p.Namespace != "" {
		n++
	}
	if p.Name != nil && *p.Name != "" {
		n++
	}
	if p.Title != nil {
		n++
	}
	if p.Summary != nil {
		n++
	}
	if p.Version != nil && *p.Version != "" {
		n++
	}
	if p.License != nil && *p.License != "" {
		n++
	}
	if p.Kind != nil {
		n++
	}
	if len(p.People) > 0 {
		n++
	}
	if len(p.Keywords) > 0 {
		n++
	}
	if len(p.Repositories) > 0 {
		n++
	}
	if len(p.Links) > 0 {
		n++
	}
	if len(p.Stack) > 0 {
		n++
	}
	return n
}
