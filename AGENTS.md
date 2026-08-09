<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# projectfile/bridge — agent guide

## Purpose

`pf-bridge` — **projections and detection** over a projectfile document. Adapts
the projectfile to external representations and back. Since the §5 split it is a
**git-style multi-binary tool**: a thin `pf-bridge` dispatcher execs one binary
per bridge on PATH.

- `pf-bridge <name> [to|from] [dir]` → execs `pf-bridge-<name>`. Each per-bridge binary round-trips its metadata file(s) (package.json via `npm`, CITATION.cff via `cff`, …) or derive-renders (LICENSE via `license`, .gitignore family via `ignore`, …). Direction is `to`/`from`/none; a single-file binary implies its filename, a multi-file one (`ignore`, `vulnerabilities`) takes the name or `all`. The `license` bridge renders the substituted root LICENSE **and** `LICENSES/<id>.txt` per SPDX term (also substituted by default; `--reuse-canonical` keeps the literal SPDX placeholders to match `reuse download`).
- `pf-bridge all` / `to all` / `from all` — fan out across every installed file bridge.
- `pf-bridge forge|scan|init` → execs `pf-bridge-forge` (push identity to GitHub/GitLab/Forgejo), `pf-bridge-scan` (filesystem/Git scanners), or
    `pf-bridge-init` (scaffold a new projectfile).

The **document backend** (read/query/mutate/convert/validate) is the sibling
`projectfile/core` module (`pf-cli`). This module was extracted from it in the
Bridge Revolution Phase 2 cut — see [../bridge-revolution.md](../bridge-revolution.md).

## The seam — how bridge consumes core

`go.mod` requires `kiota.ch/projectfile/core` as a pinned module (no `replace`;
core tags releases — currently `v1.0.1`). Every core dependency crosses through
core’s **`pkg/*` façades** — this module
holds **zero** `cli/internal/*` imports. The core API in use: `pkg/projectfile`
(the typed `*Document` — alias identity means it crosses zero-cost),
`pkg/{genlog,rawdoc,userconfig,spdx,selector,pflock,fieldpath}`. Mutable core
toggles are driven via setters (`genlog.SetQuiet`, `projectfile.SetYAMLOutputSorted`,
`spdx.SetEmbedded`, …) exposed on those façades.

## SPDX licence corpus (`internal/bridge/license/spdx/`)

Bridge owns the SPDX boilerplate texts and registers them with core’s resolver as
lookup tier 1 (`spdx.SetEmbedded`, called from the license bridge’s `init`). Core
ships **none** — it is a consumed library, so a `go:embed` asset there would have
to be tracked in a repository that never reads it; when it was untracked instead,
`core/v2@v2.0.0` shipped an empty set and broke every offline consumer.

The texts are **committed**, not fetched in CI. On the forge plane every DAG node
is its own job with its own checkout, and the corpus is needed in three of them
(`source-is-tested`, `binaries-built`, `image-is-built`); `pf-ci` refuses to place
one tool in three unrelated nodes (“node=job cannot place it”). Committing serves
all three with no mechanism at all and keeps a network round-trip out of every
build. `.scripts/download-spdx.sh` is the manual refresh — run it, commit the diff.

Each `<id>.txt` carries a `<id>.txt.license` REUSE sidecar (a licence text cannot
hold an inline header without corrupting the text it is). The tags record what we
received: SPDX publishes `license-list-data` under CC0-1.0 with no asserted holder,
so `LICENSES/CC0-1.0.txt` ships alongside `MIT.txt`. The sidecars are written by
`download-spdx.sh` next to each fetch, so a newly added ID can never arrive
unannotated and break `reuse-lint` in a later, unrelated run. Note the SPDX ID
correction: `BUSL-1.1` is canonical for the Business Source License — `BSL-1.1`
404s upstream and is a different licence (Boost).

Bridge-owned typed shapes of `org.projectfile.*` extension namespaces
(citation, readme, forge, funding, codeowners, contributing, support,
security, release, conventions, cli-derive, ignores, vulnerabilities,
editors, dei) and their accessors live in `internal/pfmodel` below — moved out of
core in the core-2.0 cut. Bridge code reaches them there, not through
`pkg/projectfile`.

## Layout

Read [docs/readme-generator.md](docs/readme-generator.md) before touching
`internal/bridge/readme/` — it is the canonical description of the block list,
the 4-tier block resolution, the artifact-driven command sections, and the
interpolation/fan-out rules. Two contracts there are easy to break by accident:

- **Sections are a LIST of groups**, one per artifact, because installation
  alternatives are not steps. A group whose commands all fail to resolve drops
  whole; a section with no surviving group falls back to its file probe.
- **Install and usage instructions come from `org.projectfile.artifacts`**, not
  from which namespace a project lives in. A shared m6e fragment declares a
  recipe per ecosystem unconditionally and the drop rule selects. Never add a
  Go-side table of package managers or an `if kind == …` in a lowering — `kind`
  stays advisory for lowering by spec.

```text
bridge/
├── main.go                 pf-bridge dispatcher (PATH discovery + exec; links NO bridges)
├── cmd/pf-bridge-*/         one tiny main per binary (17 bridges + forge/scan/init);
│                            generated by .scripts/gen-bridge-cmds.sh
├── go.mod                  module …/bridge; require …/core (pinned, no replace)
└── internal/
    ├── bridgerun/          bridge command run logic + Main(binName) (registry-driven)
    ├── rootflags/          shared persistent flags + PersistentPreRun + ReadOpts/Offline
    ├── buildinfo/          Version var (single -ldflags -X target for every binary)
    ├── forgecmd/ scancmd/ initcmd/   each: a command + Main(binName)
    ├── pfmodel/            bridge-owned projectfile extension shapes (citation, readme,
    │                        forge, funding, ...) — moved out of core in the core-2.0 cut
    ├── bridge/             every projectfile↔external-file bridge + core/ contract + registry
    ├── forge/              forge push (core/ + drivers/{github,gitlab,forgejo} + hostmatch/)
    ├── scanners/           stack + git scanners (core registry)
    ├── source/             `init` ecosystem auto-detection (reuses bridge parsers)
    ├── scaffold/           interactive `init` TUI (runs scanners)
    ├── derive/             inference engine (repo URL → forge links, stack → registries)
    └── warn/               warning ledger + end-of-run summary (+ fan-out handoff)
```

### The drift gate (`--check`)

`--check` renders, compares, and writes nothing; a content mismatch AND a
missing file both count as drift, each reported with its unified diff.

**Drift WARNS by default; it does not fail.** `--fail-on-drift` (or
`PF_BRIDGE_FAIL_ON_DRIFT=1`) restores the hard gate. The reason the default sits
on the lenient side is the fleet’s two-step rollout: a bridge fix is published
first and the image every project runs is rebuilt second, and in between every
consumer drifts through no fault of its own. A hard gate at that moment blocks
the commits that carry the fix forward — the gate ends up defending the bug.

The library keeps the strict contract: `core.Options.WarnOnly` defaults to false,
so a caller that does nothing gets an error on drift. Only the CLI opts into
leniency, and the environment variable exists because the `--check` call sites
live in m6e tool rows pinned per project: flipping the fleet back through them
is a library edit plus a pin bump everywhere, while a CI plane exports one
variable.

What makes the lenient default safe is that a warning is no longer quiet:

- `internal/warn` keeps every warning and re-states the set as the LAST thing
  the process writes, with the remedy under it.
- `m6e-run` greps the captured log of a SUCCESSFUL run and quotes the warning
  lines under its green pointer (`M6E_WARN_REVEAL=N` disables). Without that
  half, a warn-only gate under `make` would print nothing at all — the runner
  buffers the whole tool log and shows one line when the command exits 0.

Do not "fix" a drift warning by muting it. `--check` reporting nothing when a
file has drifted is the failure mode both halves exist to prevent.

### Fragment inheritance (`--refresh`)

`internal/bridge/fragments/` assembles `docs/<name>.d/*.md` into one document
(`FEATURES.md`, `ROADMAP.md`, …) and nests what upstream projects publish under
it. Parents are **repository URLs**, never filesystem paths: a path assumes
every project sits in one working copy, which silently inherited nothing for
every consumer who checked out one project alone.

Two rules hold the design together:

- **Assembly is local and offline.** Each parent’s published document is cached
  in the repository under `docs/<name>.d/.inherited/<owner>-<repo>.md` and
  committed. An ordinary run and the drift gate read those files and nothing
  else, so `--check` never needs a network and two runs always agree.
- **`--refresh` is the only step that reaches a forge.** It resolves each
  parent’s newest release tag (`git ls-remote --sort=-v:refname`, so Git orders
  the versions), fetches ONE path with `git archive --remote`, and rewrites the
  cache. Everything goes over Git — the transport these repositories already
  use — so there is no forge API, no per-forge raw-URL table, and a private
  parent resolves with the developer’s own credentials. `--refresh --check`
  answers “has upstream moved?” without writing.

Why a version, not a hash pin: the heading states `## Inherited from B19/Ubuntu
1.0.0`, and that claim stays true however far upstream moves afterwards. An
anonymous “Inherited Features” is what goes stale. Refs float by default, and
the tag actually read is recorded in the cached copy’s `pf-bridge:inherited`
comment — that recorded value is what the heading prints.

The parent is named by its own `identity.title`, read at the same ref as the
document and recorded in that same comment. A heading is prose, and `owner/repo`
is a path: every prose linter downstream reads `b19/ubuntu` as a misspelling of
the product, which failed `documentation-passes-lint` in every consumer at once.
A parent that publishes no title, and a copy cached before this existed, fall
back to `owner/repo`.

Three behaviours are load-bearing and easy to break:

- The parent’s document already carries what IT inherited, so one hop copies the
  whole chain. There is no ancestry walk and no cycle problem to solve.
- The parent’s H1/H2 headings are dropped so its H3 entries nest — the level the
  readme bridge scrapes. Fenced blocks are exempt, or a `# comment` inside a
  shell example would be eaten.
- The projectfile stays the authority on who a project inherits from: a cached
  copy whose parent is no longer declared is ignored and reported, because
  nothing else would ever remove that file.

A parent that cannot be reached warns and keeps the committed copy. Refusing to
vendor a document with no SPDX header is deliberate — the alternative is an
unlicensed file in this repository over a fault upstream owns.

Two things bite when adding a parent:

- **The vendored copy is linted by the consumer, not by upstream.** Prose that
  passes upstream can fail this project’s textlint or markdownlint. Fix it in
  the parent and cut a release — refresh reads the newest release TAG, so an
  unreleased fix on the parent’s `main` never reaches a child. Editing the
  cached copy is pointless; the next refresh overwrites it.
- **GitHub cannot serve a parent.** It disables `upload-archive`, so `git
  archive --remote` fails there with “operation not supported by protocol”.
  Point parents at a forge that serves it (kiota.ch, Codeberg), not at a GitHub
  mirror.

### Fan-out verbs

`pf-bridge all` (sync every file bridge) and `pf-bridge check` (the same sweep
with `--check` forced, writing nothing) are built into the dispatcher — there is
no `pf-bridge-check` binary, because check is a MODE every bridge already has.
`pf-bridge check <name>…` narrows it; `--all` spells the default explicitly.

**Every flag typed after a fan-out verb is forwarded to each child.** It was not
always: the child argv was rebuilt from scratch, so `pf-bridge all --check`
reached each bridge as a plain `all` and OVERWROTE every derived file — a
read-only verb doing the one thing it promised not to. `splitNamesFlags` owns
that split now and is table-tested.

Children inherit `PF_BRIDGE_WARNINGS_DIR` and hand their ledgers back instead of
each printing its own block, so a 20-bridge sweep ends in one summary.

### The binary split (§5)

One binary per bridge, git-style dispatched (decision §11.2). Each
`cmd/pf-bridge-<pkg>/main.go` blank-imports **only its own** bridge package and
calls `bridgerun.Main` — so that binary links only its code, and the same
registry-driven command sees only its own filename(s). `pf-bridge` (main.go)
carries no bridges: it discovers `pf-bridge-*` on PATH and `syscall.Exec`s the
match; `all` fans out across the file-bridge set (skipping forge/scan/init).
Adding a bridge = add the package + re-run `.scripts/gen-bridge-cmds.sh`
(idempotent). Escape hatch (§5): to collapse binaries later, drop the extra
mains — the `pf-bridge <name>` call sites never change.

The deep subsystem docs (bridge interfaces, sync/render algorithms, per-bridge
notes, how-to-add-a-bridge, forge/scanner/derive detail) currently live in
[../core/AGENTS.md](../core/AGENTS.md) and are being migrated here.

## Localized community health files

`org.projectfile.i18n` is the **document-level** localization declaration. Two
keys:

- `languages` — the locale list (BCP 47 tags) honoured by every localizable
  renderer.
- `default-language` — the project’s primary language. A BCP 47 tag; defaults
  to `en`. The canonical root-level files are written in this language; every
  OTHER language renders under `docs/<lang>/`.

Localizable set: readme, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, SUPPORT, DEI.
LICENSE is deliberately excluded — a licence’s legal force lives in its
canonical text.

**Layout.** The locale lives in the directory, not the filename: the canonical
(default-language) file renders at the repository root, and each other
language renders under `docs/<lang>/` keeping the canonical basename:

```text
README.md              CONTRIBUTING.md              SECURITY.md          ← default (en)
docs/es/README.md      docs/es/CONTRIBUTING.md      docs/es/SECURITY.md
docs/uk/README.md      docs/uk/CONTRIBUTING.md      docs/uk/SECURITY.md
```

When `default-language` is not `en`, the English copies move under `docs/en/`
and the root files render in the declared default language. The default
language is never also a variant (it is the root file), so listing it in
`languages` is a no-op — it is dropped with a diagnostic.

The machinery is `internal/bridge/core/localize.go`: `LocalizedSpec` +
`RenderLocalized` own the naming rule (`LocalizedFilename` → `docs/<lang>/`),
the missing-translation policy, the cross-language bar and the final assembly,
so a bridge opts in by handing over a per-language view instead of calling
`Render` itself. Translated bodies come from sibling templates that keep the
infix form (`SUPPORT.es.md.tmpl`) — only OUTPUT relocated under `docs/<lang>/`,
template lookup did not — registered by walking each bridge’s embedded
`templates/` dir (`core.RegisterTemplateTree`). Adding a locale is one file,
never a `register.go` edit. `es` and `uk` ship embedded.

**textlint terminology vs non-English files.** The shared textlint config’s
`terminology` rule (`textlint-rule-terminology`) is English-tuned and
false-positives on legitimate non-English text (e.g. Spanish “Todos” = “all”).
Inline `<!-- textlint-disable terminology -->` comments do NOT suppress this
rule — it does not implement textlint’s comment-directive API. The supported
fix is `.textlintignore` + `.fdignore` (the latter is load-bearing: auto-textlint
discovers files with `fd`, so explicit-arg paths bypass `.textlintignore`
discovery but still honour it as an ignore). TODO: add `docs/<lang>/` non-default
locale dirs to both ignore files so localized health files lint clean.

**Default-language re-anchoring.** The render loop’s `""` sentinel is the
canonical root render. `core.ResolveLang(lang, pf)` maps that sentinel to the
default language for STRING resolution (catalog messages, LocalizedString
fields), so a Spanish-first project’s root readme resolves Spanish. PATH
resolution (`LocalizedFilename`, `LocalizedSibling`) keeps the raw sentinel so
the default language renders at the root, not under `docs/<defLang>/`. Each
bridge’s `View(lang)` closure applies this split: string-resolving helpers
receive `ResolveLang(lang, pf)`, path-resolving helpers receive the raw `lang`.

Two rules that shape the code:

- **Prose belongs in templates, data belongs in Go.** Anything a translator
    must be able to reword lives in the `.tmpl`: the SUPPORT “Where to Ask”
    question column (Go emits a stable `Kind`, the template maps it to a
    localized question via a `{{define “want”}}` block), the SECURITY
    disclosure-window fallback, the SUPPORT response-time fallback. Go decides
    *which* rows exist and where they point; the template says what they mean.
- **A missing translation is skipped, never faked.** No default-language
    body ever ships under a localized name; the warning names the template to
    add, and the language is dropped from the cross-language bar of the files
    that did render.

Cross-references stay in-language (`core.LocalizedSibling` →
`docs/<lang>/SECURITY.md`). It cannot verify the sibling — the §5 binary split
hides other bridges’ templates from a given `pf-bridge-*` — so the declared
language set is the contract, and the sibling bridge warns by name about
anything it is missing.

The readme is the one exception to “a locale is a template”, and deliberately
so. Its eighteen blocks are three-line fragments, not prose: a per-language
copy of each would mean 18×N near-identical templates and every structural
change re-applied N times. So the readme localizes its **strings** from one
flat catalog per language (`internal/bridge/readme/messages/<lang>.yaml`,
reached from templates via the `t` function and from Go via `lookupMessage`)
and keeps one structural template per block. A block that needs a different
*shape* — not just different words — in some language still ships
`readme.md/<block>.<lang>.tmpl` (resolved via `LocalizedTemplateInfix`, which
keeps the infix form), which wins over the neutral template within its own
tier.

Two consequences worth knowing: the catalog is the home of every user-visible
label the readme derives (policies link text, link-group headings, link-type
names), keyed by a **derived** suffix (a link’s `type`, a health file’s
basename), so adding either is a catalog edit and not a Go edit; and a
`messages/<lang>.yaml` missing a key falls back to English per key rather than
dropping the file — the whole-file skip rule above governs the other five
documents, whose unit of translation is the document.

Remaining limit: labels the bridge humanizes from a filename (`docs/*.md` in
the documentation block) are the file’s own name, so they read the same in
every language. The `docs/<lang>/` locale directories are skipped by the docs
listing (they hold localized health files, surfaced through the language bar,
not generic documentation).

## Build

This is a full m6e project (`projectfile.yaml` + `.makefile/` submodules +
`Makefile`). Bridge consumes core as a **pinned module**
(`kiota.ch/projectfile/core` in `go.mod`) — no sibling `../core` checkout. The
module is private, so a cold fetch needs `GOPRIVATE=kiota.ch/*`, an SSH
`insteadOf` rewrite, and `GOPROXY=direct`.

```sh
make help           # categorised m6e target list
make build-binaries # → dist/pf-bridge + every dist/pf-bridge-* (host-native off-matrix)
make install-binary # build-binaries + copy the whole set into ~/.local/bin
make go-test        # host Go test suite
go build ./... && go test ./...   # plain host toolchain
```

`build-binaries.sh` builds **all** binaries (dispatcher + 17 bridges +
forge/scan/init); the dispatcher finds its siblings on PATH, so the m6e-only
`install-binary` tool (which self-builds via `build-binaries.sh`) drops the whole
set together. There is no hand-written `build-local`/`install-local` — those
retired with the manifest model. `.scripts/`: `build-binaries.sh` (matrix
cross-compile on the forge, host-native when the matrix binds no cell + a Git
version fallback + an unsuffixed `dist/<name>` host copy), `install-binary.sh`
(m6e-only local install), `gen-bridge-cmds.sh` (regenerate the per-binary mains
after adding a bridge), `forgejo-release.sh` / `gh-release.sh`. Version crosses
via `-ldflags -X …/internal/buildinfo.Version` (one target for every binary).

### Container lint/build (needs validation)

Host `go build`/`go test`/`go vet`/`gosec` all pass. The old blocker — a
`replace => ../core` path invisible to the m6e tool container — is gone now that
core is pinned. What a container `make build` / `make source-passes-lint` still
needs is the private-module fetch (`GOPRIVATE`/ssh/`GOPROXY=direct`) reachable
from inside the tool container; that path is not yet validated, and no
Dockerfile ships. CI-workflow generation is gated on the same validation.

## Deferred (Bridge Revolution)

- ~~The rename (`cli → core`, `pf-cli → projectfile`).~~ DONE — module
    `…/cli → …/core`, dir `cli/ → core/`, host/release binary `pf-cli → projectfile`. The container/fleet binary (`/usr/local/bin/pf-cli`, m6e
    `compile-go`) and the forge remote repository stay `cli` until §9.
- Multi-binary release wiring: `forgejo-release.sh`/`gh-release.sh` still upload the single `pf-bridge` asset; the split ships ~20 binaries per cell that a future §7 pass should attach (or bundle) per release.
- §9 release ordering: core is tagged (`v1.0.1`) and bridge pins it — the
    `replace` is gone. Remaining: validate the container build /
    `source-passes-lint` / CI-workflow generation against the pinned private module, and land bridge’s own remote repository + Dockerfile.
