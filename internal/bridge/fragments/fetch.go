// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
	"projectfile.org/projectfile/bridge/internal/warn"
)

// Every crossing to a forge is bounded: a parent whose host hangs must slow one
// generate by seconds, never stall the docs build.
const gitTimeout = 30 * time.Second

// headingRE matches the H1 and H2 headings a parent document owns. They are
// dropped when nesting: the child's document owns the H1, and the copy renders
// under the child's own H2, so what survives is the parent's H3 entries — which
// the readme bridge also scrapes, so the level is load-bearing.
var headingRE = regexp.MustCompile(`^#{1,2} `)

// languageBarRE matches a whole-line cross-language bar
// (`[Español](docs/es/FEATURES.md) · [Українська](docs/uk/FEATURES.md)`). A
// parent that ships several languages carries one above its H1; the child
// builds its own bar, so the parent's never nests — a leaked bar would point
// at the PARENT's relative paths from inside the child's document.
var languageBarRE = regexp.MustCompile(`^(\[[^\]]+\]\([^)]+\))( · \[[^\]]+\]\([^)]+\))*$`)

// errNotPublished marks the one failure that is not a problem: the parent does
// not publish this document at all. Every project declares the same shells
// (features, roadmap, …) while most parents publish only some of them, so this
// must read as an absence, not as a forge that could not be reached.
var errNotPublished = errors.New("the parent does not publish this document")

// transportRE matches git's remote-helper syntax (ext::, transport::address).
// Such a URL makes git run a command of the URL author's choosing, and a parent
// URL arrives from a file another project writes — so it is refused outright.
var transportRE = regexp.MustCompile(`^[a-zA-Z0-9+.-]+::`)

// The encodings the spec allows a projectfile document to carry.
const (
	pfYAML = "projectfile.yaml"
	pfTOML = "projectfile.toml"
	pfJSON = "projectfile.json"
)

// projectfileNames are those encodings in the order a parent is tried. The
// parent is read for one field only — identity.title, the name it gives itself
// — so the first encoding present wins.
var projectfileNames = []string{pfYAML, pfTOML, pfJSON}

// fetchParents reads every declared parent's published document and returns
// the copies keyed by parent, plus, per declared language, the parent's
// localized document read at the same ref. The boolean says whether EVERY
// declared parent was read: one unreachable parent keeps the run honest for
// the whole document — the caller then preserves the committed sections
// rather than mix fresh and stale content in one file.
//
// A parent that publishes no such document is an absence, not a failure.
func fetchParents(doc pfmodel.FragmentDocument, langs []string) (map[string]inheritedCopy, map[string]map[string]inheritedCopy, bool) {
	return reportParents(doc, collectParents(doc, langs))
}

// docFetch is one document's prefetched parents, reported back in document order.
type docFetch struct {
	copies    map[string]inheritedCopy
	localized map[string]map[string]inheritedCopy
	ok        bool
	fetched   bool
}

// fetchDocs resolves every document's parents concurrently, reporting back in document order for deterministic warnings.
func fetchDocs(docs []pfmodel.FragmentDocument, langs []string) []docFetch {
	raws := make([][]parentFetch, len(docs))
	var wg sync.WaitGroup
	for i := range docs {
		if len(docs[i].Parents) == 0 {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			raws[i] = collectParents(docs[i], langs)
		}(i)
	}
	wg.Wait()
	out := make([]docFetch, len(docs))
	for i := range docs {
		if len(docs[i].Parents) == 0 {
			continue
		}
		copies, localized, ok := reportParents(docs[i], raws[i])
		out[i] = docFetch{copies: copies, localized: localized, ok: ok, fetched: true}
	}
	return out
}

// parentFetch is one parent's fetched copies plus the error that decides its fate.
type parentFetch struct {
	parent     pfmodel.FragmentParent
	copied     inheritedCopy
	translated map[string]inheritedCopy
	err        error
}

// collectParents fetches every declared parent concurrently into ordered slots.
func collectParents(doc pfmodel.FragmentDocument, langs []string) []parentFetch {
	fetchLangs := langs
	if !docLocalizable(doc) {
		fetchLangs = nil
	}
	results := make([]parentFetch, len(doc.Parents))
	var wg sync.WaitGroup
	for i, parent := range doc.Parents {
		wg.Add(1)
		go func(i int, parent pfmodel.FragmentParent) {
			defer wg.Done()
			copied, translated, err := fetchParent(context.Background(), parent, doc.Out, fetchLangs)
			results[i] = parentFetch{parent: parent, copied: copied, translated: translated, err: err}
		}(i, parent)
	}
	wg.Wait()
	return results
}

// reportParents folds collected fetches into copies, warning in declaration order.
func reportParents(doc pfmodel.FragmentDocument, results []parentFetch) (map[string]inheritedCopy, map[string]map[string]inheritedCopy, bool) {
	copies := map[string]inheritedCopy{}
	localized := map[string]map[string]inheritedCopy{}
	ok := true
	for _, r := range results {
		switch {
		case errors.Is(r.err, errNotPublished):
			genlog.Debug("fragments: parent publishes no such document, nothing to inherit",
				"parent", r.parent.URL, "document", doc.Out)
			continue
		case r.err != nil:
			warn.Record("fragments: parent unreachable, keeping the committed sections",
				"parent", r.parent.URL, "document", doc.Out, "error", r.err.Error())
			ok = false
			continue
		}
		copies[slug(r.copied.Name)] = r.copied
		for lang, lc := range r.translated {
			if localized[lang] == nil {
				localized[lang] = map[string]inheritedCopy{}
			}
			localized[lang][slug(lc.Name)] = lc
		}
		genlog.Debug("fragments: fetched parent",
			"parent", r.copied.Name, "ref", r.copied.Ref, "commit", shortCommit(r.copied.Commit),
			"document", doc.Out, "languages", len(r.translated))
	}
	return copies, localized, ok
}

// fetchParent resolves which version of the parent to read, downloads that one
// document, and reduces it to the entries a child nests — then reads the
// parent's localized document for each declared language at the same ref. The
// parent's SPDX header travels with every copy — it is the licence of the text
// being nested, so it is kept verbatim rather than replaced by this
// project's own.
//
// Everything goes over git, the transport these repositories already use: no
// forge API, no raw-file URL that differs per forge kind, and a private parent
// resolves with the credentials the developer already has. A var so tests can
// stub the forge away.
var fetchParent = func(ctx context.Context, parent pfmodel.FragmentParent, document string, langs []string) (inheritedCopy, map[string]inheritedCopy, error) {
	if err := validateRepoURL(parent.URL); err != nil {
		return inheritedCopy{}, nil, err
	}
	name := ownerRepo(parent.URL)

	ref, commit, err := resolveRef(ctx, parent.URL, parent.Ref)
	if err != nil {
		return inheritedCopy{}, nil, err
	}
	genlog.Debug("fragments: resolved parent version",
		"parent", name, "requested", parent.Ref, "ref", ref, "commit", shortCommit(commit))

	aref := archiveRef(ref)
	var body string
	var bodyErr error
	var batch map[string]string
	var title string
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		body, bodyErr = fetchDocument(ctx, parent.URL, aref, document)
	}()
	go func() {
		defer wg.Done()
		batch = fetchExtrasBatch(ctx, parent.URL, aref, document, langs)
	}()
	go func() {
		defer wg.Done()
		title = parentTitle(ctx, parent.URL, aref)
	}()
	wg.Wait()
	if bodyErr != nil {
		return inheritedCopy{}, nil, bodyErr
	}

	spdx, rest := splitSPDX(body)
	// Vendoring text with no licence statement would put an unlicensed file in
	// this repository and break REUSE for a fault upstream owns. Refuse, and say
	// which parent to fix.
	if spdx == "" {
		return inheritedCopy{}, nil, fmt.Errorf("%s in %s carries no SPDX header — refusing to vendor unlicensed text", document, name)
	}

	cop := inheritedCopy{
		Name:     name,
		Title:    title,
		URL:      parent.URL,
		Ref:      ref,
		Commit:   commit,
		Document: document,
		SPDX:     spdx,
		Body:     normalizeInherited(rest),
	}
	return cop, translationsFromBatch(ctx, parent, cop, aref, document, langs, batch), nil
}

// fetchExtrasBatch reads one parent's localized documents in a single archive call, yielding nil when any language misses so the caller falls back to per-file reads.
func fetchExtrasBatch(ctx context.Context, repoURL, aref, document string, langs []string) map[string]string {
	var paths []string
	for _, lang := range langs {
		paths = append(paths, core.LocalizedFilename(document, lang))
	}
	if len(paths) == 0 {
		return nil
	}
	batch, err := fetchArchiveMap(ctx, repoURL, aref, paths)
	if err != nil {
		genlog.Debug("fragments: batched extras fetch missed, falling back to per-file reads", "parent", repoURL, "error", err.Error())
		return nil
	}
	return batch
}

// translationsFromBatch layers each language's copy from the batch, reading per file only the languages the batch missed.
func translationsFromBatch(ctx context.Context, parent pfmodel.FragmentParent, canonical inheritedCopy, aref, document string, langs []string, batch map[string]string) map[string]inheritedCopy {
	if len(langs) == 0 {
		return nil
	}
	type langCopy struct {
		cop inheritedCopy
		err error
	}
	results := make([]langCopy, len(langs))
	var wg sync.WaitGroup
	for i, lang := range langs {
		wg.Add(1)
		go func(i int, lang string) {
			defer wg.Done()
			path := core.LocalizedFilename(document, lang)
			if body, ok := batch[path]; ok {
				cop, err := localizedCopyFromBody(canonical, lang, path, body)
				results[i] = langCopy{cop: cop, err: err}
				return
			}
			cop, err := fetchLocalizedCopy(ctx, parent, canonical, aref, document, lang)
			results[i] = langCopy{cop: cop, err: err}
		}(i, lang)
	}
	wg.Wait()
	out := map[string]inheritedCopy{}
	for i, lang := range langs {
		cop, err := results[i].cop, results[i].err
		switch {
		case errors.Is(err, errNotPublished):
			genlog.Debug("fragments: parent publishes no such document, variant falls back to the canonical copy",
				"parent", parent.URL, "document", core.LocalizedFilename(document, lang))
			continue
		case err != nil:
			warn.Record("fragments: localized parent fetch failed, variant falls back to the canonical copy",
				"parent", parent.URL, "document", core.LocalizedFilename(document, lang), "lang", lang, "error", err.Error())
			continue
		}
		out[lang] = cop
		genlog.Debug("fragments: fetched localized parent copy",
			"parent", cop.Name, "lang", lang, "ref", cop.Ref, "document", cop.Document)
	}
	return out
}

// fetchLocalizedCopy reads one language's document from the parent and reduces
// it to the same nesting shape as the canonical copy, sharing its provenance.
func fetchLocalizedCopy(ctx context.Context, parent pfmodel.FragmentParent, canonical inheritedCopy, ref, document, lang string) (inheritedCopy, error) {
	path := core.LocalizedFilename(document, lang)
	body, err := fetchDocument(ctx, parent.URL, archiveRef(ref), path)
	if err != nil {
		return inheritedCopy{}, err
	}
	return localizedCopyFromBody(canonical, lang, path, body)
}

// localizedCopyFromBody reduces an already-fetched localized document to the nesting shape.
func localizedCopyFromBody(canonical inheritedCopy, lang, path, body string) (inheritedCopy, error) {
	spdx, rest := splitSPDX(body)
	if spdx == "" {
		return inheritedCopy{}, fmt.Errorf("%s in %s carries no SPDX header — refusing to vendor unlicensed text", path, canonical.Name)
	}
	cop := canonical
	cop.SPDX = spdx
	cop.Body = normalizeInherited(rest)
	cop.Document = path
	cop.Lang = lang
	return cop, nil
}

// refCache memoizes which version of a parent to read, keyed by repository plus wanted ref.
var refCache sync.Map // string → refResult

// titleCache memoizes what the parent calls itself, keyed by repository plus archive ref.
var titleCache sync.Map // string → string

// refMu serializes concurrent first misses so one shared parent resolves once.
var refMu sync.Mutex

// titleMu serializes concurrent first misses so one shared parent reads its title once.
var titleMu sync.Mutex

type refResult struct {
	ref    string
	commit string
}

// parentTitle reads what the parent calls ITSELF — identity.title — so a
// heading reads as prose ("B19/Ubuntu") instead of as a repository path
// ("b19/ubuntu"). Read at the same ref as the document, so the name and the
// version in one heading always describe the same release.
//
// Every failure degrades to the empty string, and the caller then names the
// parent by owner/repo — which the URL always yields. A parent that publishes
// no projectfile, or one that omits the title, stays inheritable.
func parentTitle(ctx context.Context, repoURL, ref string) string {
	key := repoURL + "\x00" + ref
	if v, ok := titleCache.Load(key); ok {
		return v.(string)
	}
	titleMu.Lock()
	defer titleMu.Unlock()
	if v, ok := titleCache.Load(key); ok {
		return v.(string)
	}
	title := parentTitleUncached(ctx, repoURL, ref)
	titleCache.Store(key, title)
	return title
}

// parentTitleUncached tries each projectfile encoding in turn for the parent's title.
func parentTitleUncached(ctx context.Context, repoURL, ref string) string {
	for _, name := range projectfileNames {
		body, err := fetchDocument(ctx, repoURL, ref, name)
		switch {
		case errors.Is(err, errNotPublished):
			continue
		case err != nil:
			genlog.Debug("fragments: parent projectfile unreadable, naming the parent by repository",
				"parent", repoURL, "file", name, "error", err.Error())
			return ""
		}
		title := identityTitle([]byte(body), name)
		genlog.Debug("fragments: read the parent title", "parent", repoURL, "file", name, "title", title)
		return title
	}
	genlog.Debug("fragments: parent publishes no projectfile, naming it by repository", "parent", repoURL)
	return ""
}

// identityTitle pulls identity.title out of one projectfile. Only the document
// itself is read: resolving its includes would mean more forge round trips for
// a field that names the project locally.
func identityTitle(body []byte, name string) string {
	var doc struct {
		Identity struct {
			// The spec allows a bare string or a language map, so the shape is
			// decided after decoding rather than by the target type.
			Title any `toml:"title" yaml:"title" json:"title"`
		} `toml:"identity" yaml:"identity" json:"identity"`
	}

	// YAML 1.2 is a superset of JSON, so one decoder covers both encodings.
	unmarshal := yaml.Unmarshal
	if strings.HasSuffix(name, ".toml") {
		unmarshal = toml.Unmarshal
	}
	if err := unmarshal(body, &doc); err != nil {
		genlog.Debug("fragments: parent projectfile did not parse, naming the parent by repository",
			"file", name, "error", err.Error())
		return ""
	}
	return localizedTitle(doc.Identity.Title)
}

// localizedTitle narrows a title to the one string a heading can carry. A bare
// string is language-agnostic and wins outright; a language map prefers English
// and otherwise takes the lowest language tag.
//
// That last tiebreak is sorted rather than "any entry" on purpose: the
// assembled document is compared byte for byte by the drift gate, so a title
// chosen by map iteration order would report drift at random.
func localizedTitle(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		langs := make([]string, 0, len(t))
		for lang := range t {
			langs = append(langs, lang)
		}
		sort.Strings(langs)
		if _, ok := t["en"]; ok {
			langs = append([]string{"en"}, langs...)
		}
		for _, lang := range langs {
			if s, ok := t[lang].(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// resolveRef answers WHICH version of the parent to read. An explicit ref is
// honoured verbatim; an empty one floats to the parent's newest release tag,
// and falls back to the default branch for a parent that publishes no tags.
//
// `--sort=-v:refname` makes git itself order the versions, so nothing here has
// to know semver — and one ls-remote covers every forge, needing no API and no
// token beyond the git access the developer already has.
func resolveRef(ctx context.Context, repoURL, want string) (ref, commit string, err error) {
	key := repoURL + "\x00" + want
	if v, ok := refCache.Load(key); ok {
		r := v.(refResult)
		return r.ref, r.commit, nil
	}
	refMu.Lock()
	defer refMu.Unlock()
	if v, ok := refCache.Load(key); ok {
		r := v.(refResult)
		return r.ref, r.commit, nil
	}
	ref, commit, err = resolveRefUncached(ctx, repoURL, want)
	if err != nil {
		return "", "", err
	}
	refCache.Store(key, refResult{ref: ref, commit: commit})
	return ref, commit, nil
}

// resolveRefUncached answers WHICH version of the parent to read against the live forge.
func resolveRefUncached(ctx context.Context, repoURL, want string) (ref, commit string, err error) {
	if want != "" {
		out, err := gitLsRemote(ctx, repoURL, "refs/tags/"+want, "refs/heads/"+want)
		if err != nil {
			return "", "", err
		}
		sha, _, ok := firstRef(out)
		if !ok {
			return "", "", fmt.Errorf("ref %q not found at %s", want, repoURL)
		}
		return want, sha, nil
	}

	out, err := gitLsRemote(ctx, repoURL, "--tags", "--refs", "--sort=-v:refname")
	if err != nil {
		return "", "", err
	}
	if sha, name, ok := firstRef(out); ok {
		return strings.TrimPrefix(name, "refs/tags/"), sha, nil
	}

	// No tags published: read the default branch. The copy's provenance keeps
	// the commit (the logs print it); the heading names the parent only.
	genlog.Warn("fragments: parent publishes no tags, reading the default branch", "parent", repoURL)
	out, err = gitLsRemote(ctx, repoURL, "HEAD")
	if err != nil {
		return "", "", err
	}
	sha, _, ok := firstRef(out)
	if !ok {
		return "", "", fmt.Errorf("no HEAD at %s", repoURL)
	}
	return "", sha, nil
}

// archiveRef is the ref name to ask the server for. Ref NAMES are used rather
// than the resolved commit because a server serves archives of what it
// advertises; the exact commit is still recorded in the copy's provenance.
func archiveRef(ref string) string {
	if ref == "" {
		return "HEAD"
	}
	return ref
}

// fetchDocument asks the remote for ONE path at one ref. `git archive --remote`
// transfers just that path, so fetching a document costs no clone.
func fetchDocument(ctx context.Context, repoURL, ref, document string) (string, error) {
	out, err := runGit(ctx, "archive", "--format=tar", "--remote="+repoURL, ref, document)
	if err != nil {
		// A path the parent does not carry makes git exit non-zero with
		// pathspec wording; that is an absence, not a failure to reach it.
		if strings.Contains(err.Error(), "did not match any files") {
			return "", fmt.Errorf("%s: %w", document, errNotPublished)
		}
		return "", err
	}
	body, err := tarEntry([]byte(out), document)
	if err != nil {
		return "", fmt.Errorf("%s at %s %s: %w", document, repoURL, ref, err)
	}
	return body, nil
}

// fetchArchiveMap reads several paths at one ref in a single archive call. Any miss fails the whole call by git's own hand, and the caller falls back to per-file reads.
func fetchArchiveMap(ctx context.Context, repoURL, aref string, paths []string) (map[string]string, error) {
	argv := append([]string{"archive", "--format=tar", "--remote=" + repoURL, aref}, paths...)
	out, err := runGit(ctx, argv...)
	if err != nil {
		return nil, err
	}
	return tarEntries([]byte(out), paths)
}

// tarEntries pulls every requested file out of a tar stream, matching each member to its longest requested suffix so a localized path never shadows its canonical one.
func tarEntries(stream []byte, paths []string) (map[string]string, error) {
	found := map[string]string{}
	reader := tar.NewReader(bytes.NewReader(stream))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return found, nil
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		best := ""
		for _, p := range paths {
			if header.Name != p && !strings.HasSuffix(header.Name, "/"+p) {
				continue
			}
			if len(p) > len(best) {
				best = p
			}
		}
		if best == "" {
			continue
		}
		if _, taken := found[best]; taken {
			continue
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		found[best] = string(body)
	}
}

// tarEntry pulls one file out of a tar stream. `git archive <ref> <path>` yields
// a stream holding just that path, but the entry is matched by name so a server
// that adds a prefix or extra members cannot hand back the wrong file.
func tarEntry(stream []byte, document string) (string, error) {
	reader := tar.NewReader(bytes.NewReader(stream))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return "", errNotPublished
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg || !strings.HasSuffix(header.Name, document) {
			genlog.Debug("fragments: skipping archive member", "member", header.Name, "want", document)
			continue
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			return "", err
		}
		return string(body), nil
	}
}

// gitLsRemote runs one bounded ls-remote against a validated repository URL.
func gitLsRemote(ctx context.Context, repoURL string, args ...string) (string, error) {
	argv := append([]string{"ls-remote"}, args...)
	return runGit(ctx, append(argv, repoURL)...)
}

// runGit runs one bounded git command and returns its stdout. Credential
// prompting is disabled: a fetch that blocks on a password would hang a build
// with no output explaining why.
func runGit(ctx context.Context, argv ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	// #nosec G204 -- every URL passed validateRepoURL, so it can be neither an
	// option nor a remote-helper transport that executes a command.
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("git %s: %w: %s", argv[0], err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", argv[0], err)
	}
	return string(out), nil
}

// firstRef takes the leading "<sha>\t<ref>" line of ls-remote output.
func firstRef(out string) (sha, name string, ok bool) {
	for _, line := range strings.Split(out, "\n") {
		sha, name, found := strings.Cut(strings.TrimSpace(line), "\t")
		if found && sha != "" {
			return sha, name, true
		}
	}
	return "", "", false
}

// validateRepoURL vets a parent URL before it reaches git. Any git-cloneable
// form is accepted — these repositories are reached over ssh — except the two
// shapes that would turn a metadata field into command execution: a
// remote-helper transport, and anything git would read as an option.
func validateRepoURL(raw string) error {
	switch {
	case strings.TrimSpace(raw) == "":
		return errors.New("parent url is empty")
	case strings.HasPrefix(raw, "-"):
		return fmt.Errorf("parent url %q would be read as a git option", raw)
	case transportRE.MatchString(raw):
		return fmt.Errorf("parent url %q uses a git remote helper, which runs a command; use a plain ssh or https url", raw)
	}
	return nil
}

// ownerRepo reduces a repository URL to "owner/repo" — the parent's name in
// every heading. Handles both url forms git accepts: ssh://host/owner/repo.git
// and the scp-like git@host:owner/repo.git. The LAST two segments win, so a
// nested group still names the project rather than the group it sits in.
func ownerRepo(repoURL string) string {
	p := repoURL
	if _, after, found := strings.Cut(p, "://"); found {
		p = after
		if _, rest, ok := strings.Cut(p, "/"); ok { // drop host[:port]
			p = rest
		}
	} else if _, after, found := strings.Cut(p, ":"); found { // scp-like
		p = after
	}

	p = strings.TrimSuffix(strings.Trim(p, "/"), ".git")
	parts := strings.Split(p, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return p
}

// slugUnsafeRE matches everything a cache filename must not carry.
var slugUnsafeRE = regexp.MustCompile(`[^a-z0-9._-]+`)

// slug turns a parent name into one cache filename (b19/ubuntu → b19-ubuntu).
func slug(name string) string {
	return strings.Trim(slugUnsafeRE.ReplaceAllString(strings.ToLower(name), "-"), "-")
}

// splitSPDX separates a leading REUSE comment from the rest of a document.
func splitSPDX(raw string) (header, body string) {
	trimmed := strings.TrimSpace(raw)
	header = strings.TrimSpace(leadingSPDXRE.FindString(trimmed))
	return header, strings.TrimSpace(strings.TrimPrefix(trimmed, header))
}

// normalizeInherited drops the parent's H1 and H2 headings so its entries nest
// one level below this project's own section heading, and the textlint wrap a
// published localized document carries — the cache file re-wraps itself on
// disk, and assembly wraps the whole variant, so the pair never nests. Fenced
// blocks are left alone — a shell comment inside an example starts with "# "
// too, and dropping those lines would quietly rewrite the parent's code
// samples.
//
// What survives is the parent's whole chain in one hop: its published document
// already carries what IT inherited, so no walk up the ancestry is needed.
func normalizeInherited(raw string) string {
	raw = textlintDirectiveRE.ReplaceAllString(raw, "")
	var kept []string
	fence := ""
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case fence != "":
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
		case strings.HasPrefix(trimmed, "```"):
			fence = "```"
		case strings.HasPrefix(trimmed, "~~~"):
			fence = "~~~"
		case headingRE.MatchString(line):
			continue
		case languageBarRE.MatchString(line):
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(multiBlankRE.ReplaceAllString(strings.Join(kept, "\n"), "\n\n"))
}
