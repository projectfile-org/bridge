<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

# LLM Policy Generator

`pf-bridge-llm` renders `LLM.md` — a prose statement of the project’s stance on
AI and LLM use — from the `org.projectfile.llm` extension namespace.

> Source (planned): `internal/bridge/llm/` — bridge (`bridge.go`), view model
> (`view.go`), embedded templates (`templates/LLM{,.es,.uk}.md.tmpl`).

## Why a bridge and not a hand-written file

A stance stated twice drifts. Today a project that changes its mind edits prose
in `CONTRIBUTING.md`, forgets `SECURITY.md`, and leaves whatever a forge told a
crawler untouched. Putting the stance in the projectfile makes it one machine-
readable declaration with one rendered face, and the drift gate (`--check`)
turns a stale policy into a warning instead of a lie left in the repository.

## The spine: two directions, never mixed

The namespace already encodes the only split that matters, and the document is
built on it:

- **Inbound** — what may come INTO the project. `attitude`, the per-activity
  keys, `disclose-required`. Governs contributors.
- **Outbound** — how the project’s own content may be consumed BY AI systems.
  `content-signals`. Governs crawlers and trainers.

They are orthogonal. A project can welcome LLM-assisted patches and refuse to
be training data, or the reverse. Any rendering that folds them into one
“AI policy: yes/no” sentence is wrong, so the template keeps them under
separate headings and never derives one from the other.

## Spec changes

Landed in `projectfile/specification` (`spec/shapes/org.projectfile.llm.yaml`
and `spec/registry.yaml`). The shape is `status: proposed`, so these were
changes, not migrations. Each removes a mechanism rather than adding one.

### 1. One vocabulary for `attitude` and every activity

Today `attitude` is `encouraged | allowed | neutral | discouraged | prohibited`
and activities are `allowed | neutral | banned`, joined by a derivation table.
The table distorts:

- `neutral` (“we have no opinion”) derives to `assist-allowed` for pull
  requests, which is an opinion — it bans agent-opened PRs.
- `discouraged` derives to `banned`. A project that discourages has not
  prohibited, and the derived value says it did.
- `prohibited` and `banned` are two words for one state across two enums.

Use ONE enum in both places: `encouraged | allowed | neutral | discouraged |
prohibited`. Derivation becomes the identity function, the table disappears
from the spec, and no attitude is silently strengthened.

### 2. Lift the agent-versus-assist axis out of `pull-requests`

`pull-requests: agents-allowed | assist-allowed | banned` makes one field carry
two questions — how much is permitted, and by what kind of actor. That is why
it needed its own enum.

Split the axis into a document-level key:

```yaml
autonomy: any        # any | assisted | none
```

`assisted` means a human drives and submits; `none` means no LLM in the loop at
all; `any` permits autonomous agents. It composes with EVERY activity, not just
pull requests — autonomous bug reports are the spam problem projects actually
have — and `pull-requests` collapses to the common enum.

Per-activity autonomy overrides are deliberately out of v1. Add them only when
a real project needs one.

### 3. `statement` — the paragraph enums cannot carry

```yaml
statement: >-
  We accept LLM-assisted contributions because reviewing a patch is our job
  either way. What we will not accept is a patch the submitter cannot explain.
```

A localizable free-prose paragraph rendered verbatim under the H1. Without it,
`LLM.md` is a filled-in form; with it, it is a stance. Precedent exists in the
namespaces already shipping: `support.response-time` and `dei.scope` are the
same idea — prose that overrides a template fallback.

### 4. `disclose-trailer` — make disclosure copyable

`disclose-required: true` tells a contributor to disclose and not how. Add:

```yaml
disclose-required: true
disclose-trailer: Assisted-by
```

The template then renders the exact line to paste into a commit message. With
no trailer declared, the prose stays generic — the boolean alone still works.

### 5. Absence is not permission

State normatively: an absent `org.projectfile.llm` means “no declared policy”.
Consumers MUST NOT render a permissive default, and the bridge MUST emit
nothing. The namespace being present IS the opt-in — no `enabled` key, unlike
`dei`, because a policy namespace with no policy in it has no other meaning.

This is what keeps a fleet-wide `pf-bridge all` from publishing an invented
“AI is allowed” into every project that never said so.

### Kept as-is

- `content-signals` stays an open vocabulary; unknown values render.
- `skills` stays untouched and unread — the bridge is a Renderer and never
  writes the projectfile, so round-trip preservation is free.
- `links[type=ai-policy]` already exists in §4 and is the escape hatch for
  projects whose real policy is a governance page.

### Known limits, not proposed for v1

- `content-signals` is flat. A project whose code and prose deserve different
  signals cannot say so. Scoping it (`{code: […], docs: […]}`) waits for a real
  request.
- Model provenance constraints (“no output from models trained on GPL code”)
  are the legal concern behind most bans. They belong in `statement` until the
  vocabulary for them settles.

## Shape after the changes

```yaml
org:
  projectfile:
    llm:
      attitude: allowed            # REQUIRED, the default for every activity
      autonomy: assisted           # any | assisted | none        (default: any)
      statement: >-                # OPTIONAL, localizable free prose
        …
      disclose-required: true      # OPTIONAL (default: false)
      disclose-trailer: Assisted-by
      content-signals: [search, ai-input]   # OPTIONAL, open vocabulary

      # OPTIONAL per-activity overrides; same enum as attitude
      translations: encouraged
      security-reports: prohibited

      skills: {}                   # shape TBD, preserved untouched
```

## What `LLM.md` renders

```text
# AI and LLM Policy
[cross-language bar]

<statement, verbatim>                       (omitted when unset)
Full policy: <links[type=ai-policy]>        (omitted when unset)

## Contributions made with AI assistance
<one sentence from attitude>
<one sentence from autonomy>
<disclosure paragraph + trailer block>      (omitted unless disclose-required)
<table: activity | stance>                  (only rows differing from attitude)

## Using this project’s content
<content-signals, one bullet each, each with its meaning>
<or: no signals declared — absence is not permission>

## Questions
<contact from people[roles=community]>
```

Two rendering rules carry the design:

- **Only overrides reach the table.** A row is rendered when its value differs
  from `attitude`, not when it is merely declared. A project that spelled out
  nine activities identical to its attitude gets a three-paragraph file, and a
  project with two real exceptions gets exactly two rows. A table of nine rows
  all saying the same word is noise a reader learns to skip.
- **Absence of a signal is stated, never implied.** The content section always
  renders. When `content-signals` is unset it says so, and says that silence is
  not consent and that the licence governs reuse independently — a reader
  arriving from a crawler needs the sentence more than a permissive project
  does.

## Bridge contract

| Concern | Decision |
| --- | --- |
| Kind | `core.Renderer` (derive-only; no read-back path) |
| Filename | `LLM.md` — `core.FileLLM`, alias `AI.md` for `pf-bridge` lookup |
| Policy | `core.Policy{Marker: true}` |
| Gate | namespace absent → empty `Output`, existing file never deleted |
| Localization | `core.RenderLocalized` + `LocalizedSpec`, `docs/<lang>/LLM.md` |

`Marker`, not `ScaffoldOnce`: the file is a projection of declared fields, so
flipping `attitude` MUST change the file on the next run. `CONTRIBUTING.md` is
`ScaffoldOnce` because it becomes the maintainer’s document; a policy that
became the maintainer’s document would be the drift this bridge exists to kill.

### Prose in templates, data in Go

Every enum token is data; its meaning is prose. Go emits stable tokens —
`{Activity: "pull-requests", Stance: "prohibited"}` — and the template maps
both halves through `{{define}}` blocks, exactly as `SUPPORT.md` maps its row
`Kind` to a localized question. Consequences:

- A new activity key needs no Go edit. The template’s fallback prints the raw
  key, so an unknown activity renders as itself rather than vanishing.
- A new locale is one template file, as everywhere else in this bridge.

### Determinism

The activity set is a map, and a map-ordered render fails `--check` at random.
Sort: known activities in a canonical declared order first, unknown keys
alphabetically after. `content-signals` keeps its declared order (it is a
sequence) with duplicates dropped.

An unrecognised enum value warns and renders raw. Refusing to render is the
wrong failure: the reader loses the whole policy over one typo.

## Integration

- **readme** — add `core.FileLLM` to `healthFiles` in `internal/bridge/readme/
  view.go` and a `policy.llm` key to each `messages/<lang>.yaml`. The label key
  is derived from the basename, so nothing else changes.
- **contributing** — a one-line pointer to `LLM.md`, NOT a copy of the policy.
  The shape’s note about generating “CONTRIBUTING.md sections” predates this
  file existing; two documents restating one policy is a drift source, and the
  bridge that owns the policy should be the only one stating it.
- **security** — no change. An LLM-authored vulnerability report is governed by
  `security-reports`, which renders in `LLM.md` where the rest of the stance is.

## Implementation checklist

Specification:

- `spec/shapes/org.projectfile.llm.yaml` — rewrite per the five changes.
- `spec/registry.yaml` — update the summary (drop the derivation table
  description, drop `llms.txt` from the consumer list).

Bridge:

- `internal/pfmodel/helpers.go` — `LLMExtensionNS`.
- `internal/pfmodel/types.go` — `LLMExtension`.
- `internal/pfmodel/extensions.go` — `GetLLMExtension`, returning `(nil, nil)`
  when absent.
- `internal/bridge/core/localize.go` — `FileLLM = "LLM.md"`.
- `internal/bridge/llm/` — `bridge.go`, `view.go`, `register.go`,
  `templates/LLM{,.es,.uk}.md.tmpl` with `.license` sidecars.
- `internal/bridge/readme/` — health-file list plus three catalog keys.
- `.scripts/gen-bridge-cmds.sh` — re-run; it writes `cmd/pf-bridge-llm/`.

Tests worth writing first, because each guards a decision above:

- absent namespace renders nothing and deletes nothing;
- an activity equal to `attitude` produces no table row;
- unknown activity keys survive to the output;
- two renders of a multi-activity document are byte-identical.

## Non-goals

- **`llms.txt`.** A different artefact for a different audience: an index of
  the project’s documentation for retrieval, not a statement of stance. It has
  a different lifetime and a different write policy (pure derived data, always
  overwritten), so it belongs to a sibling bridge if it is ever wanted.
- **`AGENTS.md`.** Instructions TO an agent working in the repository. It is
  hand-written knowledge, not a projection of declared fields, and nothing in
  the namespace could derive it.
- **`robots.txt` and `/.well-known/`.** Deployment artefacts of a site, not
  of a source repository. `content-signals` renders as prose here; a project
  serving a site can lower the same field itself.
