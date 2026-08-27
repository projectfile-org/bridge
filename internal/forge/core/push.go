// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Field is the pf-side identifier for a forge field. Three values today;
// kept as named constants so the command's --field flag and the registry's
// filter logic share a single vocabulary.
const (
	FieldDescription = "description"
	FieldHomepage    = "homepage"
	FieldTopics      = "topics"
)

// AllFields is the canonical iteration order for log output. Mirrors the
// order the field-mapping table in the design doc uses.
var AllFields = []string{FieldDescription, FieldHomepage, FieldTopics}

// reasonFieldDisabled is the user-facing "this field would have been pushed
// but isn't" line, emitted for every field the --field flag or extension
// toggles off. Centralised so the wording stays consistent across rows.
const reasonFieldDisabled = "field disabled via flag or extension"

// Registry returns the driver for a given forge kind. Tests inject a fake
// here; production code populates it from each driver's init() via Register.
type Registry func(kind string) Client

// PushOptions threads the command-flag surface into the algorithm. Every
// field is optional; the zero value is the default-everything run.
type PushOptions struct {
	// DryRun skips the Apply call but still performs the Fetch (we need
	// current state to compute the diff that gets logged).
	DryRun bool

	// Repos restricts the run to a subset of repositories[].url
	// values. Empty slice = every non-archive repo runs.
	Repos []string

	// Fields restricts which fields the diff considers. Empty = AllFields.
	Fields []string

	// Forge restricts the run to a single forge kind (e.g. "github",
	// "gitlab", "forgejo"). Empty = every forge kind runs.
	Forge string

	// Resolve looks up the driver for a forge kind. Tests inject a stub;
	// the command wires the real registry.
	Resolve Registry

	// TokenLookup is a seam for tests; production uses ResolveToken directly.
	TokenLookup func(kind, host string) (token, source string)

	// Now is "current time"; unused by push today but reserved so the
	// algorithm stays test-friendly when we later add rate-limit handling.
	Now func() int64
}

// Push walks pf.Repositories, computes a per-repo Patch,
// and either applies it or (in --dry-run) records what would have been
// applied. Every per-repo, per-field, per-skip decision is logged via
// genlog so the output reads like the sync / generate runs.
func Push(ctx context.Context, pf *projectfile.Document, opts PushOptions) (PushResult, error) {
	if opts.Resolve == nil {
		return PushResult{}, fmt.Errorf("forge.Push: PushOptions.Resolve is required")
	}
	if opts.TokenLookup == nil {
		opts.TokenLookup = ResolveToken
	}

	ext, err := pfmodel.GetForgeExtension(pf)
	if err != nil {
		return PushResult{}, fmt.Errorf("read forge extension: %w", err)
	}
	// Global kill-switch. Logged at the top so even a quiet run surfaces
	// "we didn't push anything and here's why".
	if ext != nil && !ext.Push {
		genlog.Info("forge push disabled by org.projectfile.forge.push", "extension", pfmodel.ForgeExtensionNS)
		return PushResult{DryRun: opts.DryRun}, nil
	}

	desired := buildDesiredSnapshot(pf)
	allowField := buildFieldFilter(opts.Fields, ext)
	repoFilter := buildRepoFilter(opts.Repos)
	forgeFilter := buildForgeFilter(opts.Forge)

	out := PushResult{DryRun: opts.DryRun}

	var repos []projectfile.Repository
	if pf != nil {
		repos = pf.Repositories
	}
	if len(repos) == 0 {
		genlog.Warn("forge push: no repositories in top-level repositories[]")
		return out, nil
	}

	for _, repo := range repos {
		rr := processRepo(ctx, repo, desired, allowField, repoFilter, forgeFilter, ext, opts)
		out.Repos = append(out.Repos, rr)
	}
	return out, nil
}

// processRepo is the per-entry body of the main loop. Extracted so the
// loop body stays readable and each early-return is one statement.
func processRepo(
	ctx context.Context,
	repo projectfile.Repository,
	desired Snapshot,
	allowField func(string) bool,
	repoFilter func(string) bool,
	forgeFilter func(string) bool,
	ext *pfmodel.ForgeExtension,
	opts PushOptions,
) RepoResult {
	rr := RepoResult{URL: repo.URL}

	if !repoFilter(repo.URL) {
		genlog.Info("forge push: skip (not in --repo filter)", "url", repo.URL)
		rr.Skipped = true
		rr.Reason = "not in --repo filter"
		return rr
	}
	// archive entries are frozen by design — pushing to them would defeat
	// the whole point of the role. Origin + mirror both get synced.
	if repo.Role == projectfile.RepositoryRoleArchive {
		genlog.Info("forge push: skip (archive role)", "url", repo.URL, "role", repo.Role)
		rr.Skipped = true
		rr.Reason = "archive role"
		return rr
	}

	rule, host := hostmatch.Resolve(repo.URL)
	rr.Host = host
	// Resolve the forge kind. Extension-declared kind wins over hostmatch:
	// the user knows what's running on code.example.com better than we do,
	// and the same override fixes the rare case where hostmatch guesses
	// wrong (a non-Forgejo instance happening to live at "gitea.foo.bar").
	kind := resolveKind(ext, rule, host)
	if kind == "" {
		genlog.Warn("forge push: unsupported host", "url", repo.URL, "host", host,
			"hint", `set ["org.projectfile.forge".kinds]."`+host+`" = "forgejo"|"gitlab"|"github"`)
		rr.Skipped = true
		rr.Reason = `unsupported host (declare ["org.projectfile.forge".kinds]."` + host + `")`
		return rr
	}
	rr.Kind = kind

	if !forgeFilter(host) {
		genlog.Info("forge push: skip (not in --forge filter)", "url", repo.URL, "host", host)
		rr.Skipped = true
		rr.Reason = "not in --forge filter"
		return rr
	}

	if ext != nil && ext.Hosts != nil {
		if allow, ok := ext.Hosts[host]; ok && !allow {
			genlog.Info("forge push: skip (host opt-out)", "url", repo.URL, "host", host)
			rr.Skipped = true
			rr.Reason = "host opt-out via org.projectfile.forge.hosts"
			return rr
		}
	}

	token, tokenSource := opts.TokenLookup(kind, host)
	if token == "" {
		envName := HostOverrideEnvVar(host)
		if v := CanonicalEnvVar(kind); v != "" {
			envName = envName + " or " + v
		}
		genlog.Warn("forge push: skip (missing token)", "url", repo.URL, "env", envName)
		rr.Skipped = true
		rr.Reason = "no token (set " + envName + ")"
		return rr
	}
	_ = token       // forwarded to the driver via context value below
	_ = tokenSource // surfaced in `forge list`, not in the push line

	driver := opts.Resolve(kind)
	if driver == nil {
		genlog.Warn("forge push: no driver registered", "kind", kind)
		rr.Skipped = true
		rr.Reason = "no driver registered for kind " + kind
		return rr
	}

	owner, name, err := driver.Owner(repo.URL)
	if err != nil {
		genlog.Error("forge push: parse url failed", "url", repo.URL, "err", err.Error())
		rr.Failed = true
		rr.Error = err.Error()
		return rr
	}

	repoCtx := WithHost(WithToken(ctx, token), host)
	current, err := driver.Fetch(repoCtx, owner, name)
	if err != nil {
		genlog.Error("forge push: fetch failed", "url", repo.URL, "err", err.Error())
		rr.Failed = true
		rr.Error = err.Error()
		return rr
	}

	patch, changes := diffWithFilter(current, desired, allowField, hostmatch.Kind(kind))
	rr.Changes = changes
	if patch.IsEmpty() {
		genlog.Info("forge push: up to date", "url", repo.URL)
		rr.UpToDate = true
		return rr
	}

	genlog.Section(fmt.Sprintf("forge push: %s", repo.URL))
	for _, fc := range changes {
		switch {
		case fc.Skipped:
			genlog.Decision(fc.Field, "(skipped)", fc.Source, fc.Reason)
		default:
			genlog.Decision(fc.Field, fc.Value, fc.Source, "")
		}
	}

	if opts.DryRun {
		genlog.Info("forge push: dry-run, no API call", "url", repo.URL)
		return rr
	}
	if err := driver.Apply(repoCtx, owner, name, patch); err != nil {
		genlog.Error("forge push: apply failed", "url", repo.URL, "err", err.Error())
		rr.Failed = true
		rr.Error = err.Error()
		return rr
	}
	genlog.Info("forge push: applied", "url", repo.URL, "fields", len(changes))
	return rr
}

// buildDesiredSnapshot extracts the three pushable fields from the
// projectfile, applying the documented fallback rules. Sole reader of
// pf-side metadata in this package — keeps the mapping logic in one place.
func buildDesiredSnapshot(pf *projectfile.Document) Snapshot {
	snap := Snapshot{}
	if pf == nil {
		return snap
	}
	// Description: summary first, then first line of description. Forges
	// expose a single description slot and don't render markdown there, so
	// the first paragraph of the full description is the closest analogue.
	summary := projectfile.ExtractLocalizedString(pf.Identity.Summary)
	if summary != "" {
		snap.Description = summary
	} else {
		desc := projectfile.ExtractLocalizedString(pf.Identity.Description)
		snap.Description = firstLine(desc)
	}
	snap.Homepage = pfmodel.LinkURL(pf, projectfile.LinkHomepage)
	snap.Topics = LowerTopics(pf.Keywords)
	return snap
}

// resolveKind merges the extension's per-host kinds override with
// hostmatch's automatic guess. The extension wins on conflict: a user who
// writes [org.projectfile.forge.kinds]."code.example.com" = "forgejo" is
// asserting authoritative knowledge about that host, including for the
// edge case where hostmatch would have guessed differently. Returns ""
// when neither source produces a kind — the caller surfaces the hint.
func resolveKind(ext *pfmodel.ForgeExtension, rule *hostmatch.Rule, host string) string {
	if ext != nil && ext.Kinds != nil {
		if k, ok := ext.Kinds[host]; ok && k != "" {
			return k
		}
	}
	if rule != nil {
		return string(rule.Kind)
	}
	return ""
}

// firstLine returns the first non-empty line of s, trimmed. Used by the
// description fallback so a multi-paragraph identity.description still
// produces a single-line forge description.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t != "" {
			return t
		}
	}
	return ""
}

// buildFieldFilter combines the --field CLI flag and the extension's per-
// field toggles. Both sides default to "on": the CLI flag is empty (every
// field) and the extension toggles default to true (every field).
func buildFieldFilter(cliFields []string, ext *pfmodel.ForgeExtension) func(string) bool {
	allowed := map[string]bool{}
	if len(cliFields) == 0 {
		for _, f := range AllFields {
			allowed[f] = true
		}
	} else {
		for _, f := range cliFields {
			allowed[strings.ToLower(strings.TrimSpace(f))] = true
		}
	}
	if ext != nil {
		if !ext.Fields.Description {
			allowed[FieldDescription] = false
		}
		if !ext.Fields.Homepage {
			allowed[FieldHomepage] = false
		}
		if !ext.Fields.Topics {
			allowed[FieldTopics] = false
		}
	}
	return func(field string) bool { return allowed[field] }
}

// buildRepoFilter turns the --repo CLI flag into a membership predicate.
// Empty flag means "every repo allowed"; otherwise exact-string match
// against the on-disk URL (no scheme normalisation — match what the user typed).
func buildRepoFilter(repos []string) func(string) bool {
	if len(repos) == 0 {
		return func(string) bool { return true }
	}
	set := map[string]bool{}
	for _, r := range repos {
		set[r] = true
	}
	return func(url string) bool { return set[url] }
}

// buildForgeFilter turns the --forge CLI flag into a host-matching
// predicate. Empty flag means "every forge allowed"; otherwise the repo's
// host must contain the filter substring (case-insensitive), so 'github'
// matches github.com, 'codeberg' matches codeberg.org, 'kiota' matches
// kiota.ch.
func buildForgeFilter(forge string) func(string) bool {
	if forge == "" {
		return func(string) bool { return true }
	}
	needle := strings.ToLower(forge)
	return func(host string) bool { return strings.Contains(strings.ToLower(host), needle) }
}

// diffWithFilter runs Diff but honours the per-field allow predicate and
// the per-kind asymmetries (GitLab has no homepage slot). Returns the
// filtered patch plus the per-field change records for logging.
func diffWithFilter(
	current, desired Snapshot,
	allow func(string) bool,
	kind hostmatch.Kind,
) (Patch, []FieldChange) {
	full := Diff(current, desired)
	patch := Patch{}
	changes := []FieldChange{}

	if full.Description != nil {
		if !allow(FieldDescription) {
			changes = append(changes, FieldChange{
				Field: FieldDescription, Skipped: true,
				Reason: reasonFieldDisabled,
			})
		} else {
			patch.Description = full.Description
			changes = append(changes, FieldChange{
				Field: FieldDescription, Value: Trunc(*full.Description),
				Source: "identity.summary",
			})
		}
	}
	if full.Homepage != nil {
		switch {
		case !allow(FieldHomepage):
			changes = append(changes, FieldChange{
				Field: FieldHomepage, Skipped: true,
				Reason: reasonFieldDisabled,
			})
		case kind == hostmatch.KindGitLab:
			// GitLab's API has no homepage field — log the asymmetry once
			// per repo rather than aborting the whole push.
			changes = append(changes, FieldChange{
				Field: FieldHomepage, Skipped: true,
				Reason: "forge:gitlab-no-homepage",
			})
		default:
			patch.Homepage = full.Homepage
			changes = append(changes, FieldChange{
				Field: FieldHomepage, Value: Trunc(*full.Homepage),
				Source: "links[type=homepage]",
			})
		}
	}
	if full.Topics != nil {
		if !allow(FieldTopics) {
			changes = append(changes, FieldChange{
				Field: FieldTopics, Skipped: true,
				Reason: reasonFieldDisabled,
			})
		} else {
			patch.Topics = full.Topics
			changes = append(changes, FieldChange{
				Field:  FieldTopics,
				Value:  Trunc(fmt.Sprintf("%v", *full.Topics)),
				Source: "keywords",
			})
		}
	}
	return patch, changes
}

// tokenCtxKey is the context key drivers use to retrieve the per-request
// auth token. Unexported so external packages must go through WithToken /
// TokenFromContext rather than poking the key directly.
type tokenCtxKey struct{}

// WithToken attaches token to ctx for later retrieval by a driver inside
// Fetch/Apply. Decoupling token transport from the Client interface keeps
// the interface stable when a future driver adds OAuth.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenCtxKey{}, token)
}

// TokenFromContext returns the token previously attached with WithToken,
// or "" when none was set. Drivers MUST tolerate the empty case so unit
// tests can run them without ceremony.
func TokenFromContext(ctx context.Context) string {
	v, _ := ctx.Value(tokenCtxKey{}).(string)
	return v
}

// hostCtxKey is the context key drivers use to retrieve the per-request
// repo host. Same WithToken-style decoupling: passing the host through
// context rather than the Client interface keeps the interface stable.
type hostCtxKey struct{}

// WithHost attaches host to ctx so drivers can build the right API base URL
// for self-hosted instances. Without this, a Forgejo at code.example.com
// would have its API requests sent to codeberg.org — wrong host, 401s.
func WithHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, hostCtxKey{}, host)
}

// HostFromContext returns the host previously attached with WithHost, or ""
// when none was set. Drivers fall back to their canonical public host on "".
func HostFromContext(ctx context.Context) string {
	v, _ := ctx.Value(hostCtxKey{}).(string)
	return v
}
