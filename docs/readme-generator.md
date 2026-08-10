<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

# Readme Generator

`pf-bridge-readme` composes a project’s `README.md` from the
`org.projectfile.readme` extension plus reserved projectfile fields
(identity, links, license). This doc covers how to configure and extend it.

> Source: `internal/bridge/readme/` — bridge (`bridge.go`), view model
> (`view.go`), embedded templates (`templates/readme.md/*.tmpl`), message
> catalogs (`messages/<lang>.yaml`).

## Quick start

Add a `readme` block under `org.projectfile` and run the bridge:

```yaml
org:
  projectfile:
    readme:
      shields:
        - name: license
          alt: license MIT
          img: https://img.shields.io/static/v1?label=license&message=MIT&color=blue
          href: https://example.com/repo/src/branch/main/LICENSE
```

```sh
pf-bridge-readme to    # write projectfile → README.md
pf-bridge-readme       # sync (newer side wins)
```

The first `to` run creates `README.md`. Subsequent runs overwrite **only**
files carrying the managed marker; pass
`--force` to overwrite a file lacking it.

## Composition model

The readme is a sequence of named **blocks**. The bridge resolves each block
in order and concatenates the non-empty results.

### Default block list

When `blocks:` is absent, the bridge uses this built-in order. Every block is
**probe-driven**: when its data source is absent it renders empty and is
silently dropped, so a bare project shows only `basics` + `license` while a
rich project fills every section.

| Block           | Renders from                                                                               | Template ships |
| --------------- | ------------------------------------------------------------------------------------------ | -------------- |
| `languages`     | `i18n.languages` (cross-link bar; see [Multi-language READMEs](#multi-language-readmes))   | yes            |
| `logo`          | `docs/logo.<ext>` then `assets/logo.<ext>` probe                                           | yes            |
| `basics`        | `identity.title` + `identity.summary`                                                      | yes            |
| `badges`        | `readme.shields` (see [Badges](#badges))                                                   | yes            |
| `screenshots`   | `docs/screenshots/*.<img>` probe                                                           | yes            |
| `features`      | `FEATURES.md` probe (H3 feature titles as bullets + link)                                  | yes            |
| `benchmarks`    | `BENCHMARKS.md` probe                                                                      | yes            |
| `quick-start`   | `readme.quick-start` groups, else `QUICKSTART.md` probe                                    | yes            |
| `requirements`  | `REQUIREMENTS.md` probe                                                                    | yes            |
| `artifacts`     | `org.projectfile.artifacts` — what the project ships                                       | yes            |
| `platforms`     | `operating-system` × `architecture` (spec §4.8a) — OCI platform set                        | yes            |
| `installation`  | `readme.installation` groups, else `INSTALL.md` probe                                      | yes            |
| `usage`         | `readme.usage` groups, else `USAGE.md` probe                                               | yes            |
| `configuration` | `CONFIGURATION.md` probe                                                                   | yes            |
| `building`      | `readme.building` groups, else `BUILD.md` + `docs/MAKEFILE.md` + the `ci` goal nodes       | yes            |
| `documentation` | `docs/*.md` probe (excludes `readme-generator.md` and `MAKEFILE.md`)                       | yes            |
| `faq`           | `FAQ.md` probe                                                                             | yes            |
| `roadmap`       | `ROADMAP.md` probe                                                                         | yes            |
| `policies`      | CONTRIBUTING / SECURITY / SUPPORT / CODE_OF_CONDUCT `.md` probe (human-readable labels)    | yes            |
| `related`       | `links[]` tagged `tags: [related]` — “Related projects” bar (fixed after `badges`)         | yes            |
| `links`         | top-level `links[]`, categorized                                                           | yes            |
| `funding`       | `FUNDING.md` probe                                                                         | yes            |
| `license`       | `license.spdx`                                                                             | yes            |

Override the list to reorder, drop, or add blocks:

```yaml
org:
  projectfile:
    readme:
      blocks: [basics, badges, links, license]
```

### Block resolution (4-tier, first match wins)

For each block name, the bridge looks in this order:

1. **Project-local template** —
    `.projectfile/templates/readme.md/<name>.tmpl`. Takes precedence over
    everything. Use this to customise a built-in block or add a new one.
1. **Embedded built-in** — the template shipped with the bridge
    (`templates/readme.md/<name>.tmpl`).
1. **Extras entry** — a `{name, content}` pair in `readme.extras` whose
    `name` matches. Content is a localized-string, rendered **as-is**
    (no templating).
1. **Skip silently** — no match; the block is omitted. This lets a project
    declare a block that exists only as a local template.

A block that renders to only whitespace is dropped, and runs of empty lines
are collapsed — so empty templates never produce stray blank sections.

Inside each **template** tier the language variant is tried first:
`<name>.<lang>.tmpl`, then `<name>.tmpl`. Translating a block does not need
one — the shipped templates localize their own strings from the [message
catalog](#message-catalogs). Reach for a variant only when a block has to be
*shaped* differently in some language.

## Command sections

Four blocks — `installation`, `quick-start`, `usage`, `building` — render inline
when the projectfile declares them, and fall back to their companion-file link
(`INSTALL.md`, `QUICKSTART.md`, `USAGE.md`, `BUILD.md`) when it does not.

A section is a **list of groups**, because a project ships several things and the
ways to install them are *alternatives, not steps* — `npm install foo` and
`docker pull foo` inside one fenced block invites a reader to run both. Each
group gets its own lead-in sentence and its own fence:

```yaml
org:
  projectfile:
    readme:
      installation:            # same shape for quick-start, usage, building
        - name: image          # identity for override; never displayed
          prefix: "Pull the published container image:"    # optional prose
          commands:
            - docker pull ${org.projectfile.artifacts{kind=image}.ref}
          postfix: See <docs/TAGS.md> for available tags.  # optional prose
        - name: npm
          prefix: "Or add the package to your project:"
          commands:
            - npm install ${org.projectfile.artifacts{kind=package,registry=npm}.name}
```

- `prefix` and `postfix` are localized-strings (bare string or a lang→text
    map, like `identity.summary`). They frame the code block with prose.
- `commands` is a list of lines rendered as one ` ```sh ` fenced block.
    Commands are **code, not localized** — a shell command reads the same in
    every language.
- The heading comes from the message catalog (`installation.title`, …), so it
    localizes with the rest of the readme.
- `name` is the group’s identity for override, never display text.
- A group with no `commands`, `prefix`, or `postfix` is dropped; a section left
    with no group falls back to its file probe — declaring the key never
    produces an empty section.

> YAML note: quote any `prefix`/`postfix` that ends in a colon
> (`"…container image:"`), or the parser reads the trailing `:` as a mapping
> key.

### Sections are driven by artifacts

The commands above name no project. They name what the project **ships** —
[`org.projectfile.artifacts`](#artifacts) — and that is the whole mechanism:

```yaml
# the project says only WHAT it produces …
org:
  projectfile:
    artifacts:
      npm-package: { kind: package, registry: npm, name: textlint-rule-x }
```

… and the shared fragment carrying the npm recipe renders, while the fragments
carrying the Docker, PyPI and Go recipes drop themselves. Nothing is configured
twice, and nothing has to ask what kind of project this is.

What makes that safe is the **drop rule**, the same one badges already run on:

- A command line whose references cannot all be resolved is **dropped**.
- A group whose declared commands *all* dropped is dropped whole — lead-in
    sentence included, because the prose exists to introduce those commands.
- A group that declared **no** commands is intentional prose and is kept.
- A section left with no group falls back to its companion-file probe.

So a fragment may declare a recipe for every ecosystem in the fleet,
unconditionally, with no `when:` and no per-namespace variant. Only a project
that declares the matching artifact can answer the reference.

Before this, install instructions were chosen by *which namespace a project
lived in* — the `d9t/*` metadata include wired the "run a tool from the image"
recipe, the `b19/*` one wired `FROM`. A project’s readme described the folder it
was filed under rather than the thing it produces, and a published npm library
could show `docker pull` above its npm version badge.

### Interpolation

A command string, a `prefix`/`postfix`, and every badge URL may reference **any
field of the merged document** as `${<fieldpath>}`, using the same address
grammar `pf-cli get` takes. This is spec §3.8; there is no token vocabulary to
memorize and no list to extend.

```yaml
commands:
  - uv add ${identity.name}
  - curl ${links[type=source-code].url}/releases
  - docker pull kiota.ch/${image.basename}:latest
img: https://img.shields.io/static/v1?label=license&message=${license.spdx}&color=4c1
```

Anything that is **not** a resolvable field address is left **verbatim**: a make
variable (`${B19_DOCKER_REGISTRY}`), a shell variable (`$PWD`), a typo, or a path
that resolves to a mapping. `$$` escapes a literal `$`. That is what makes the
mechanism safe to evaluate over shared and remote include content — every other
actor’s `${…}` survives for that actor to resolve later.

Badges take this one step further: a shield whose `img` or `href` still carries
an unresolved reference is **dropped** rather than published broken, which is how
one shared badge row serves projects with different sets of forge mirrors.

Two addressing forms matter for artifacts, and they differ in how many values
they yield:

| Form                             | Container      | Yields               |
| -------------------------------- | -------------- | -------------------- |
| `artifacts.<name>.path`          | map, by key    | that one artifact    |
| `artifacts{kind=image}.ref`      | map, by filter | **every** match      |
| `links[type=source-code].url`    | list, by match | the **first** match  |
| `keywords[]`                     | list           | **every** entry      |

The map filter uses **curly** braces because `artifacts` is a mapping of named
keys — `artifacts[kind=image]` is a malformed address and `pf-cli` refuses it
loudly. It yields every match rather than the first because a mapping has no
order in which "first" would mean anything.

### Fan-out

A command whose reference resolves to **several** values is emitted once per
value. That is how a base image built once per Ubuntu series documents every
series without naming any of them:

```sh
docker pull kiota.ch/b19/ubuntu/resolute:latest
docker pull kiota.ch/b19/ubuntu/noble:latest
```

When a group fans out to more than one command, the bridge renders the **first**
cell as a showcased default block, then a one-line variant summary
(`B19_LLVM_SERIES: 22, 21`), then the remaining cells — so a reader sees the
primary image to grab and every other variant, instead of an undifferentiated
wall of pulls. A single-cell group renders one block with no summary.

Two sources of several values, either of which works:

1. **Several matching artifacts** — `{kind=image}` on a project declaring two
    image artifacts.
1. **A `{AXIS}` matrix placeholder** inside a resolved value, expanded against
    `org.projectfile.ci.matrix.axes` — the same substitution m6e and ci-resolver
    perform when building. Only axes the document *declares* are substituted, so
    a shell brace (`docker inspect --format '{{.Id}}'`) is left alone. YAML
    scalar axis values are coerced to their string form, so an integer axis
    (`B19_LLVM_SERIES: [22, 21]`) substitutes as `22`/`21`, not silently drops.

Prose (`prefix`/`postfix`) cannot fan out — a sentence has no per-value form — so
a multi-valued reference there leaves the sentence unresolved, and it is dropped
rather than published with a literal `${…}` in it.

### `syntax`

A command group renders as a fenced block tagged `sh`. Set `syntax` to change
it — a base image’s usage snippet is a Dockerfile, a library’s is source code:

```yaml
usage:
  - name: base-image
    prefix: "Build on top of this image:"
    syntax: dockerfile
    commands:
      - FROM ${org.projectfile.artifacts{kind=image}.ref}
```

### Building block: which goals are highlighted

When the `building` block derives its entry points from the CI DAG (no
`readme.building` section declared), it lists one `make <name>` line per
`goal: true` node. Two refinements narrow that list and fill in the local-dev
story:

- **`tags: [readme]`** on a goal node opts it into the highlighted set. The
    block prefers tagged goals and falls back to **every** goal when none are
    tagged, so a project that never opts in keeps the full list it always had.
    `m6e/core/goals/publish.yaml` tags `ready-to-publish` by default (the local
    pseudo-CI entry point); a project tags further goals to surface them.
- **The intro explains `make` and the dev loop.** A line notes that bare `make`
    runs the default target (`make help` lists them all); when the DAG declares
    a `dev-container` node (the container plane’s selectable dev node), a second
    line advertises `make dev-container` as the local dev loop — the node
    launcher m6e derives for that node.

```yaml
# m6e/core/goals/publish.yaml
org:
  projectfile:
    ci:
      nodes:
        ready-to-publish:
          goal: true
          description: Run the pseudo-CI pipeline locally — build, test and scan, without publishing
          tags: [readme]   # highlighted in the README
```

### Inheritable / traitable sections

Because [includes union sequences](#composition-model), a section declared in a
shared include fragment is inherited by every project that includes it. One
fragment gives an entire family of container projects a correct install section
with **zero per-project text**:

```yaml
# m6e/container/traits/oci-image.yaml — the shared "trait"
org:
  projectfile:
    artifacts:
      image:
        kind: image
        ref: ${org.projectfile.readme.registry}/${image.basename}:${org.projectfile.readme.tag}
    readme:
      registry: kiota.ch
      tag: latest
      installation:
        - name: image
          prefix: "Pull the published container image:"
          commands:
            - docker pull ${org.projectfile.artifacts{kind=image}.ref}
```

`${image.basename}` is a **synthetic** field: an explicit `ci.image` when the
project declares one, else `<last-label(identity.namespace)>/<identity.name>`.
Core computes it for every consumer of the image-naming rule, so the readme
documents the same path the build actually pushes to.

To **replace** an inherited group rather than extend the section, redeclare its
`name` in your own projectfile: last wins, and the position is kept so an
override never reorders the section. Union has no delete, so keep each recipe
family in exactly one owning fragment.

## Artifacts

The `artifacts` block lists what the project ships, from
`org.projectfile.artifacts`. It is the "what IS this" that the installation and
usage sections then answer "how do I get it" and "how do I call it" for:

```yaml
org:
  projectfile:
    artifacts:
      cli:
        kind: binary
        command: pf-bridge            # what a user types
        module: kiota.ch/x/bridge     # what `go install` fetches
        path: dist/pf-bridge          # what a size analyzer measures
      db:
        kind: service
        summary: MySQL-compatible database
        ports:
          - 3306
          - { port: 9104, name: metrics }
```

renders:

```markdown
## What this provides

- **Executable** `pf-bridge`
- **Service** `db` — listens on `3306`, `9104 (metrics)` — MySQL-compatible database
```

Each artifact is shown by the one string that identifies it to a reader: `ref`
for an image, `command` for a binary, `name` for a package, `module` for a
library, `url` for a site — falling back to the declared key, so a service
identified only by its ports still has a label. The kind’s display text comes
from the message catalog under `artifact.kind.<kind>`, so a project inventing a
kind gets a correct line immediately and only its label needs a catalog entry.

## Template data & functions

Every template (tiers 1 and 2) executes against a tiny view model with Go
`text/template` syntax (`missingkey=zero`, so missing fields render empty).
The view is rebuilt per language in multi-language renders (see
[Multi-language READMEs](#multi-language-readmes)).

The view holds only render-state that is a function of the *render*, not of
the source document:

| Field        | Type                         | Source                                              |
| ------------ | ---------------------------- | --------------------------------------------------- |
| `.Doc`       | `*projectfile.Document`      | raw document; reach any field via `.Doc.*`          |
| `.Lang`      | `string`                     | active language code (empty for the default render) |
| `.Languages` | `[]{.Code .Label .Filename}` | cross-link bar; every lang except the active one    |

Everything else a template might want — the project name, the probes, the
link buckets — is reached through **template functions**. Each function
calls the same helper the bridge uses internally, so the values you see in
`-v` decision trace match what the template gets. Probes run lazily: a
block that does not call `logo` never stats the logo candidates, so adding
a block to the list never pays for probes it does not use.

### Template functions

| Function                              | Returns                       | Source                                                  |
| ------------------------------------- | ----------------------------- | ------------------------------------------------------- |
| `pf "a.b.c"`                          | `any` (empty on miss)         | dotted reverse-DNS walk of `Doc.Extensions`             |
| `t "key"`                             | `string` (key itself on miss) | message catalog, resolved in the active language        |
| `ls .LocalizedString`                 | `string`                      | localized string resolved in the active language        |
| `projectName`                         | `string`                      | `identity.title.en`, else namespace/name                |
| `license`                             | `string` (empty when unset)   | `license.spdx`                                          |
| `badges`                              | `[]{.Alt .Img .Href}`         | `readme.shields[]` (alt falls back to name)             |
| `readmeSection "name"`                | {.Title …} or nil             | `readme.<name>` section, `${…}` expanded in commands    |
| `linkGroups`                          | `[]{.Key .Heading .Links}`    | top-level `links[]`, bucketed by category               |
| `relatedLinks`                        | `[]{.Label .URL}`             | `links[]` tagged `tags: [related]` — the related bar    |
| `staticLinks`                         | `[]{.Filename .Label}`        | health-file probe; label is human-readable              |
| `docLink "FILE" "Label"`              | `{.Filename .Label}` or nil   | single companion-file probe; `{{with}}` drops on nil    |
| `logo`                                | `[]string`                    | `docs/logo.<ext>` then `assets/logo.<ext>`              |
| `screenshots`                         | `[]{.Path .Name}`             | `docs/screenshots/*.<img>`                              |
| `docLinks`                            | `[]{.Filename .Label}`        | `docs/*.md`; label is the file’s first heading          |
| `buildLinks`                          | `[]{.Filename .Label}`        | `BUILD.md` + `docs/MAKEFILE.md`                         |
| `featureHeadings`                     | `[]string`                    | `FEATURES.md` level-3 titles (feature bullets)          |

Call a function with no arguments by name (`{{projectName}}`,
`{{with badges}}…{{end}}`). `docLink` is the one two-argument function:
`{{with docLink "FEATURES.md" "Features"}}…{{.Filename}}…{{end}}`.

`links[]` entries are bucketed into the groups `project`, `community`,
`security`, then `other` (in that order). `.Key` is that stable identity;
`.Heading` is the same key resolved through the message catalog, which is what
the template prints. The category comes from the link’s `type`. Within a
category, links order by [priority](#priority) (higher first) and fall back to
declaration order:

- **Project** — `homepage`, `source-code`, `documentation`, `changelog`,
    `wiki`, `faq`, `package-registry`
- **Community** — `chat`, `forum`, `contact`, `enforcement`,
    `first-contribution`, `donation`, `translate`
- **Security** — `security-policy`, `security-report`, `bug-bounty`
- **Other** — any unrecognized `type`

When a project’s links all fall in a single group (the near-universal case —
source-code + bugs), the `### <group>` subheading is omitted: it would only
repeat the `## Links` title. A project mixing groups keeps the subheadings,
since they then carry information the title does not.

Each link’s label falls back from `link.label` to the catalog entry for its
`type` (`link.type.source-code` → “Source Code”), then to the raw `type`.

A link tagged `tags: [related]` is shown in the [Related projects](#related-projects)
bar under the badges, not in this section — a sibling appears once, where a
reader looks for neighbours.

## Custom projectfile fields

`pf` and `ls` are the escape hatch for fields the bridge does not surface
as a dedicated function. Custom extensions under `[org.<ns>]` are one
template expression away — no bridge fork needed.

- `.Doc` is the raw `*projectfile.Document`; typed fields are reachable
    directly, e.g. `{{.Doc.Identity.Version}}`.
- `pf "a.b.c"` walks `Doc.Extensions` by dotted reverse-DNS path
    (`"org.acme.todo"`, `"org.projectfile.i18n.languages"`, …) and returns
    the subtree — a map, slice, string, or number. A miss renders empty
    rather than `<no value>`, so `{{with pf "x"}}…{{end}}` drops cleanly.
- `ls .LocalizedString` resolves a `*LocalizedString` in the active render
    language — useful because localization is a function, not a field, so
    a template can’t otherwise call the resolver inline.

Worked example — surface the project status stored under a custom
`[org.projectfile.status]` extension:

```gotemplate
{{- with pf "org.projectfile.status"}}
{{- if eq . "archived"}}
> ⚠️ This project is archived — no new development.
{{end -}}
{{end -}}
```

Both encodings the core supports work transparently: the literal dotted
key (`org.acme.todo` under YAML/JSON or TOML quoted) and the exploded
dotted-table header (`[org.acme.todo]` in TOML).

## Badges

These are Markdown image links of the form `[![alt](img)](href)`, declared
under `readme.shields`. The built-in `badges` block renders them space-separated,
one line per `row`.

```yaml
org:
  projectfile:
    readme:
      shields:
        - name: license          # REQUIRED — identity, and the alt-text fallback
          img: https://img.shields.io/static/v1?label=license&message=${license.spdx}&color=4c1  # REQUIRED
          href: LICENSE          # OPTIONAL — omit for an unlinked indicator
          alt: License           # OPTIONAL — alt text; defaults to name
          row: static            # OPTIONAL — the line this badge joins
          priority: 300          # OPTIONAL — render order within the row (higher first)
```

Every field is [interpolated](#interpolation), which is what makes a badge pure
data — the cost of a new badge, for a whole fleet, is one YAML entry and no code.

- `name` is the badge’s identity and the **alt-text fallback**. A missing
    `alt` never yields an empty image alt (which would render as a broken
    badge), because `alt` falls back to `name`.
- A shield whose `img` or `href` still carries an **unresolved** `${…}` after
    interpolation is **dropped**. This is what lets one shared fragment declare
    a badge per forge: the Codeberg badge simply does not render for a project
    with no Codeberg mirror. A shield with **no** `href` renders unlinked.
- Shields are **deduplicated by `name`, last wins**, keeping the first
    position. Include entries merge before the base document (spec §4.9a), so
    redeclaring a name in your own projectfile replaces the inherited badge —
    the only override available, since includes union sequences and cannot
    delete. A redeclaration also adopts the new `priority` and the new `row`.
- `priority` orders a badge **within its row** ([see Priority](#priority)):
    higher renders first, default `50`, ties keep declaration order.
- With no `shields` entries, the `badges` block renders empty and is skipped
  automatically.

> **Prefer shields.io’s `/static/v1?label=…&message=…&color=…` query form over
> the `/badge/LABEL-VALUE-COLOR` path form.** The path form splits label and
> value on `-`, so any value containing a dash (`git-flow`, `Apache-2.0`, …)
> parses as extra segments and 404s. Query params carry the value verbatim.

### Rows

`row` groups badges into rendered lines, each its own Markdown paragraph.
Badges sharing a row name render on one line ordered by
[priority](#priority) (higher first, declaration order as the tiebreak), and the
**rows themselves appear in the order each name is first seen** — no second key
declares that order, because include order already is it: a fragment merged
earlier opens its row higher up.

That is what lets independent fragments compose a layout none of them can see.
The m6e fleet uses three names:

| Row         | Holds                                                           | Owned by                                    |
| ----------- | --------------------------------------------------------------- | ------------------------------------------- |
| `static`    | facts that follow from the source (licence, REUSE, PRs welcome) | `core/traits/{badges,reuse,community}.yaml` |
| `dynamic`   | live project signals (status, last commit, latest release, CI)  | `core/traits/badges{,-ci}.yaml`             |
| `ecosystem` | registry and dependency signals (npm, PyPI, libraries.io)       | the `library/traits/*-publish` fragments    |

A badge with no `row` joins the unnamed row, so a document that never heard of
rows renders exactly one line. Grouping runs **after** the name dedup, so
redeclaring a name also re-files that badge into the row the redeclaration
names — the only way to move an inherited badge.

```markdown
[![License](…)](LICENSE) [![REUSE compliance](…)](…)

![Project status](…) [![Last commit](…)](…)

[![npm version](…)](…) [![Dependency freshness](…)](…)
```

### Priority

A `priority` field orders peers **within** a list: badges inside a row, links
inside a category, related links inside the bar, the forge-star and support
rows inside a kind. **Higher renders first; the default is `50`.** The sort is
stable, so equal priorities — and every entry that declares none — keep the
order they were declared in. A project that never sets `priority` renders
byte-identical to one with no priority support at all.

It applies uniformly to badges and to links, so one field pins the support
badge to the head of the static row and one field surfaces a sibling first in
the related bar:

```yaml
org:
  projectfile:
    readme:
      shields:
        - name: support-ukraine
          img: https://img.shields.io/badge/Support-Ukraine-0057B7
          priority: 300          # head of the static row
        - name: license
          img: https://img.shields.io/static/v1?label=license&message=MIT
          # no priority — stays at the default 50, after support-ukraine
```

```yaml
links:
  - type: source-code
    url: https://codeberg.org/projectfile/cli
    label: Projectfile CLI
    priority: 100            # first in its category / first forge star
  - type: source-code
    url: https://github.com/projectfile/cli
    label: GitHub mirror
```

`priority` on a link rides the spec’s §139 additional-key channel (`tags` uses
the same path), so it crosses the document unchanged with no schema change and
propagates to every document that consumes `links[]` — the readme links and
related bar, the SUPPORT “Before You Ask” and “Where to Ask” lists, and the
CONTRIBUTING forge-star and project-socials lines.

Priority never reorders the **semantic groupings** themselves — the badge rows,
the link categories (`project → community → security → other`), the where-to-ask
kind ladder. It only reorders peers inside one.

### Forge coordinates

Badge endpoints want host, owner and repository name separately, and neither templates nor
the address grammar can slice a URL. The bridge therefore derives, per
`links[type=source-code]` mirror and **in memory only**:

```text
org.projectfile.forge.remotes.<slug>.{host,owner,repo,url,kind}
```

The slug is the first domain label (`codeberg.org` → `codeberg`, `kiota.ch` →
`kiota`), so nothing has to be declared. `kind` honours
`org.projectfile.forge.kinds`, which is how a self-hosted instance on a bare
hostname is classified at all. Nothing is written to the projectfile — the
values are a function of links already in it.

```yaml
img: https://api.reuse.software/badge/${org.projectfile.forge.remotes.codeberg.host}/${org.projectfile.forge.remotes.codeberg.owner}/${org.projectfile.forge.remotes.codeberg.repo}
```

#### Capability aliases

A slug names a forge, so a fragment written against one **decides the topology
of every project that includes it**. `remotes.codeberg` in a shared badge is a
Codeberg mirror the consumer never chose, and it renders nothing at all for a
consumer who does not have one.

So each mirror is also reachable by what it **affords**. A source-code link
declares its capabilities through the advisory `tags` list — the same §139
channel `related` and `priority` ride, so there is no schema change:

```yaml
links:
  - type: source-code
    url: https://codeberg.org/b19/ubuntu
    tags: [public, badges]     # a third-party badge service can crawl this one
  - type: source-code
    url: https://github.com/damian-buho/b19-ubuntu
    tags: [public, ci]
  - type: source-code
    url: https://kiota.ch/b19/ubuntu
    preferred: true
    tags: [ci]
```

Every tag becomes an alias of that mirror’s coordinate map, alongside its slug:

```yaml
img: https://api.reuse.software/badge/${org.projectfile.forge.remotes.badges.host}/…
```

The vocabulary is **open** — the deriver aliases whatever tag it finds, so a
fragment and a project agree on a word without either registering it. Two
aliases need no tag at all: `preferred` comes from `links[].preferred`, and
`issues` from `repositories[issues=true]` (spec §4.3a) matched to the link
naming the same repository, so the bug tracker stays declared exactly once even
though the repository entry holds an SSH clone URL a badge cannot use.

Two rules keep the namespace honest, and both keep the slug:

- **An alias never shadows a slug.** A project that tags a mirror `gitea` keeps
    `remotes.gitea` meaning gitea.com. The claim is refused with a warning.
- **The first claimant wins**, in document order — the rule slug collisions
    already use. Later claimants are traced, not dropped silently.

An alias no mirror claims stays unresolved, so the badge referencing it is
dropped rather than published broken. That is what lets one fragment serve a
fleet whose projects each sit on a different set of mirrors.

`pf-bridge scan git-remotes` **proposes** the tags a hostname settles, from the
same rule table that already classifies a forge kind:

| Host                        | Proposed                              |
| --------------------------- | ------------------------------------- |
| `github.com`                | `public`, `badges`, `badges-github`   |
| `gitlab.com`                | `public`, `badges`, `badges-gitlab`   |
| `codeberg.org`, `gitea.com` | `public`, `badges`, `badges-gitea`    |
| `git.sr.ht`                 | `public`                              |
| self-hosted, unknown        | nothing                               |

It proposes nothing else — `ci` and `releases` say what a project decided to
run where, which no hostname reveals — and nothing at all for a self-hosted
instance (`gitlab.example.com`) or an unknown host, because a wrong `public`
tag persists in the projectfile and renders a broken badge forever, while a
missing one only drops a badge and stays visible for the user to fix.

`badges` is host-agnostic: `api.reuse.software` takes any host it can clone
from. The `badges-*` three name a **shields.io route family**, because
`/github/last-commit`, `/gitlab/last-commit` and `/gitea/last-commit` are three
endpoints and no template chooses between them. A fragment declares one badge
per route, and the drop rule picks: the routes no mirror claims do not resolve,
so they render nothing. That is the conditional this grammar does not otherwise
have. Note the route family is **not** the forge kind — Codeberg is kind
`forgejo` and shields calls its route `gitea`.

Most projects have both a Codeberg and a GitHub mirror, so both claim `badges`
and document order decides which one `remotes.badges` resolves to. That is fine
for a host-agnostic badge and wrong for a route-specific one, which is exactly
why the route tags exist.

The proposal is a **gap-fill on key presence**: a link with no `tags` key gets
one, and a link that already declares `tags` keeps exactly what it declares —
including `tags: []`, which states that this mirror affords nothing. So a
rescan never argues with a curated list, and repeated scans are byte-identical.

`--force` (`-f`) overwrites a declared list with the proposal. Without it the
first fleet-wide scan is irreversible: every projectfile would hold whatever
vocabulary shipped first and no later run could correct it. `--force` applies
to `tags` only; `label` and `preferred` stay gap-fill, because nothing
generates a better label than the user.

Example render:

```markdown
[![license MIT](https://img.shields.io/static/v1?label=license&message=MIT&color=blue)](https://example.com/LICENSE)
```

## Related projects

The `related` block renders a headingless bar of sibling-project links right
after the badges — the navigational slot where a reader looks for neighbours.
It is driven entirely by the spec’s top-level `links[]`: any entry that carries
the advisory `tags: [related]` joins the bar. There is no separate list and no
config block under `org.projectfile.readme`. Entries order by
[priority](#priority) within the bar (higher first), falling back to document
order.

```yaml
links:
  - type: source-code                 # keeps its real type
    url: https://codeberg.org/projectfile/cli
    label: Projectfile CLI
    tags: [related]                   # opts into the bar
  - type: source-code
    url: https://codeberg.org/projectfile/core
    label: Projectfile Core
    tags: [related]
```

renders, immediately after the badges:

```markdown
[Projectfile CLI](https://codeberg.org/projectfile/cli) | [Projectfile Core](https://codeberg.org/projectfile/core)
```

- A tagged link keeps its real `type` — `tags` is an additional key the spec
    preserves on round-trip (§139), so the link stays discoverable by type
    elsewhere (a forge bridge, CITATION) while also surfacing as a sibling.
- A tagged link is **excluded** from the regular Links section, so a sibling
    appears once, not twice. An untagged `source-code` mirror still lists under
    Links as usual.
- Each entry’s label falls back from `link.label` to the catalog entry for its
    `type`, then to the raw `type` — the same chain the Links section uses, so a
    sibling reads identically in both places.
- With no tagged link the block renders empty and is silently dropped, like
    every probe-driven block.

## Multi-language readmes

Declaring `i18n.languages` produces one readme per language, each fully
resolved in its own language with a cross-link bar to the others at the top.
The default `README.md` is always rendered (it carries the canonical English
content and is the file forges link to); `README.<lang>.md` is rendered for
every declared language.

The list is **document-level**, not readme-local: the same
`org.projectfile.i18n.languages` drives `CONTRIBUTING.<lang>.md`,
`CODE_OF_CONDUCT.<lang>.md`, `SECURITY.<lang>.md` and `SUPPORT.<lang>.md`.
See [Localized community health files](#localized-community-health-files).

```yaml
identity:
  name: demo
  summary:
    en: A demo project
    es: Un proyecto demo
    uk: Демонстраційний проєкт
org:
  projectfile:
    i18n:
      languages: [es, uk]
```

Output: `README.md`, `README.es.md`, `README.uk.md`. Each file’s
`languages` block renders a bar of every *other* language — never a link to
itself. For example, `README.es.md` starts with:

```markdown
[EN](README.md) · [UK](README.uk.md)
```

Localization scope:

- **Resolved per language** — `identity.summary`, `identity.description`,
    every `links[].label`, and the `content` of any `extras` entry declared
    as a language map.
- **Per-language link targets** — the `policies` block links the
    same-language variant of each community health file when it exists on
    disk (`README.es.md` → `CONTRIBUTING.es.md`), falling back to the
    canonical file.
- **Resolved from the catalog** — every section heading and every label the
    bridge derives rather than reads: the `policies` link text, the
    link-group headings, the link-type names, the boilerplate sentence in
    each probe block. See [Message catalogs](#message-catalogs).
- **Untranslatable** — labels taken from a document’s first heading
    (`docs/deployment-guide.md` whose `# Deploying the App` heading becomes
    “Deploying the App” in the `documentation` block; a heading-less file falls
    back to its humanized filename). The heading on disk is the same in every
    language; give the file a localized name, or override the block. The
    `features` bullets likewise come from `FEATURES.md`’s H3 titles verbatim.

Per-language extras content: declare the `content` as a language map and the
bridge picks the right variant for each render. A bare-string `content` is
used verbatim across all languages.

```yaml
org:
  projectfile:
    readme:
      extras:
        - name: notice
          content:
            en: |
              ## Important notice
              Hello world.
            es: |
              ## Aviso importante
              Hola mundo.
      blocks: [languages, notice, license]
```

## Message catalogs

Readme blocks carry no English of their own. Every fixed string — the heading,
the boilerplate sentence, the derived labels — comes from a flat catalog, one
YAML file per language, embedded in the bridge:

```text
internal/bridge/readme/messages/
├── en.yaml   ← canonical; every key lives here first
├── es.yaml
└── uk.yaml
```

```yaml
installation.title: Instalación
installation.body: Consulta [Instalación](%s) para ver los pasos de instalación.
```

- Templates read it with `{{t "installation.title"}}`; a body string is
    substituted with `{{printf (t "installation.body") .Filename}}`, so the
    `%s` link target can sit anywhere the language wants it.
- A key missing from a language falls back to `en.yaml`, then to the key
    itself — an unknown key renders visibly rather than leaving a bare `##`.
- Keys whose suffix is *derived* need no code change to extend:
    `link.type.<type>` for any link type, `policy.<basename>` for any
    community health file (`CODE_OF_CONDUCT.md` → `policy.code-of-conduct`).

This is why the readme localizes differently from the four health files: those
are prose documents, translated whole; readme blocks are eighteen three-line
fragments whose only translatable content is a heading and a sentence. One
catalog per language beats eighteen near-identical templates per language.

### Adding a readme language

Copy `en.yaml`, translate the values, name it `<tag>.yaml`. Keep exactly one
`%s` in every `*.body` string — a test enforces both that and full key
coverage, so a half-finished catalog fails the build rather than shipping a
half-English readme.

## Localized community health files

`org.projectfile.i18n.languages` is not a readme feature — it is the
document-level locale list, and the same declaration localizes every
community health file the bridges generate:

| Canonical | Variant | Bridge |
| --- | --- | --- |
| `README.md` | `README.<lang>.md` | `readme` |
| `CONTRIBUTING.md` | `CONTRIBUTING.<lang>.md` | `contributing` |
| `CODE_OF_CONDUCT.md` | `CODE_OF_CONDUCT.<lang>.md` | `coc` |
| `SECURITY.md` | `SECURITY.<lang>.md` | `security` |
| `SUPPORT.md` | `SUPPORT.<lang>.md` | `support` |

`LICENSE` is deliberately absent: a licence’s legal force lives in its
canonical text, so the `license` bridge never localizes.

The four non-readme files render from a single per-language template rather
than a block tree. Resolution mirrors the canonical tier order, with the
language tag in the same position as it sits in the filename:

1. `.projectfile/templates/CONTRIBUTING.es.md.tmpl` — project-local override
2. the embedded `CONTRIBUTING.es.md.tmpl` shipped in `pf-bridge-contributing`

`es` and `uk` ship embedded for all four files. Any other language works the
moment the project drops a template in tier 1.

**A language with no template is skipped, not faked.** `pf-bridge` warns with
the exact path to add and moves on:

```text
WARN no template for language — file skipped file=SUPPORT.fr.md lang=fr \
     hint="add .projectfile/templates/SUPPORT.fr.md.tmpl"
```

A skipped language is also dropped from the cross-language bar of the files
that *did* render, so no variant ever advertises a translation that was never
written.

Cross-references between health files stay in-language: `SUPPORT.es.md` links
`SECURITY.es.md` and `CONTRIBUTING.es.md`, not their canonical siblings.

### Adding a language to the shipped set

Adding a locale is one file per document — no Go edit. Templates register by
walking each bridge’s embedded `templates/` directory, so the installed
translation set is exactly the set of files on disk:

```text
internal/bridge/support/templates/
├── SUPPORT.md.tmpl
├── SUPPORT.es.md.tmpl
└── SUPPORT.fr.md.tmpl   ← drop it in, rebuild, done
```

Translate prose only — every template action must survive verbatim, and any
in-page anchor in a table of contents has to match its translated heading.

## Extras — inline content blocks

For one-off content that doesn’t warrant a template file, add it under
`extras` and reference it from `blocks`:

```yaml
org:
  projectfile:
    readme:
      blocks: [basics, special-psa, links, license]
      extras:
        - name: special-psa
          content:
            en: |
              ## Important Notice

              This project is in early development.
            es: |
              ## Aviso Importante

              Este proyecto está en desarrollo temprano.
```

`content` is a localized-string: either a bare string or a map of lang → text.
For a single-language render the bridge resolves it with the default precedence
(`en`, then first non-empty). For a multi-language render (see
[Multi-language READMEs](#multi-language-readmes)) each `README.<lang>.md`
gets its own variant from the map, falling back to the default when the
requested lang is missing. Extras content is rendered **as-is** — it is not
passed through the template engine, so a template expression like
`{{projectName}}` in extras is emitted literally, not evaluated.

## Local template overrides

Drop a `.tmpl` file at `.projectfile/templates/readme.md/<block>.tmpl` to
override a built-in or define a new block. The template sees the same view
model and functions as built-ins — `t` included, so one override can serve
every language. Name it `<block>.<lang>.tmpl` to override a single language;
it wins over the neutral override for that language only.

For example, to write a project-specific installation block:

`.projectfile/templates/readme.md/installation.tmpl`:

````gotemplate
{{- with projectName}}
## Installation

Install `{{.}}` with:

```sh
go install projectfile.org/projectfile/bridge/cmd/pf-bridge@latest
```
{{end -}}
````

## CLI

```text
pf-bridge-readme                 # sync — newer side wins
pf-bridge-readme to              # write — projectfile → README.md
pf-bridge-readme from            # read  — README.md → projectfile (no-op; write-only bridge)
pf-bridge-readme to --dry-run    # preview the rendered output without writing
pf-bridge-readme to --force      # overwrite even without the managed marker
pf-bridge-readme to --check      # report drift, write nothing, exit non-zero
pf-bridge-readme to --offline    # refuse network fetches (embedded data only)
pf-bridge-readme --list          # list registered bridge filenames
```

Flags: `--dry-run` (`-n`), `--force` (`-f`), `--check`, `--offline`,
`--quiet` (`-q`), `--verbose` (`-v`), `--no-create`, `--ignore-user-config`,
`--sorted`, `--fail-on {error|warning}`.

### `--check`, the drift gate

`--force` is not a gate. It always succeeds, and it succeeds by overwriting
whatever a human wrote — so a CI step that runs it reports green while
destroying the evidence. `--check` renders in memory, compares byte-for-byte
against the file on disk, writes nothing, and exits non-zero on any difference
(a **missing** generated file counts as drift too).

That splits the concern in two, which is how `m6e/core` wires it: the local
plane runs `pf-bridge readme --force` to generate, the forges run
`pf-bridge readme --check` to verify what was committed. On a two-way bridge
`--check` means the same thing — any field the sync *would* have moved is drift.

The bridge is **write-only** (`from` is a no-op): the readme is a projection
of the projectfile, not a source of truth. Generation prepends a single HTML
comment holding both the REUSE SPDX header and the managed marker
(`pf-cli-managed: yes` folded in before the closing `-->`); everything between
is block output. `core.HasMarker` recognises all three sentinel forms — the
hash line, the standalone HTML comment, and the folded inner line — so the
merged header still reads as managed.
