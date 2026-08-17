<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

# AI Policy Generator

`pf-bridge ai-policy` renders the project’s AI policy — `AI_POLICY.md` by default —
from the `org.projectfile.ai` extension namespace.

> Source: `internal/bridge/aipolicy/` — bridge (`bridge.go`), view model
> (`view.go`), embedded templates (`templates/AI_POLICY{,.es,.uk}.md.tmpl`).

## Why a bridge and not a hand-written file

A stance stated twice drifts. Without this, a project that changes its mind
edits prose in `CONTRIBUTING.md`, forgets `SECURITY.md`, and leaves whatever a
forge told a crawler untouched. Putting the stance in the projectfile makes it
one machine-readable declaration with one rendered face, and the drift gate
(`--check`) turns a stale policy into a warning instead of a lie left in the
repository.

## The spine: three directions, never mixed

- **Inbound** — what may come INTO the project. `attitude`, `autonomy`,
  `applies-to`, `activities`, `obligations`, the disclosure keys, the
  contribution gates, `enforcement`. Governs contributors.
- **Internal** — how the project itself uses AI. `project-use`. Governs
  maintainers.
- **Outbound** — how the project’s own content may be consumed BY AI systems.
  `content-signals`. Governs crawlers and trainers.

They are orthogonal. A project can welcome LLM-assisted patches, refuse to be
training data, and hand-run every release. Any rendering that folds them into
one “AI policy: yes/no” sentence is wrong, so the template keeps them under
separate headings and never derives one from another.

## The shape

```yaml
org:
  projectfile:
    ai:
      filename: AI_POLICY.md       # OPTIONAL basename        (default as shown)
      attitude: allowed            # REQUIRED, the default for every activity
      autonomy: assisted           # any | assisted | none            (any)
      applies-to: contributors     # everyone | contributors      (everyone)
      statement: >-                # OPTIONAL, localizable free prose
        …
      disclose-required: true
      disclose-trailer: Assisted-by
      disclose-details: [tool, extent]
      obligations: [understand, review, test, edit, human-reply]
      issue-required: true
      excluded-labels: ["good first issue"]
      enforcement: [warn, close, ban]       # declared order = escalation order
      content-signals: [search, ai-input]   # OPTIONAL, open vocabulary

      activities:                  # OPTIONAL, stance per activity
        security-reports: prohibited
        images: prohibited
        audio: allowed

      project-use:                 # OPTIONAL, autonomy per activity
        pull-requests: assisted
        releases: none

      skills: {}                   # shape TBD, preserved untouched
```

Two structural decisions carry the rest:

- **Activities live in a map.** They used to sit at the top level, which made
  the namespace’s key space open: every policy key added later would have been
  ambiguous with an activity of the same name. The map closes it.
- **`project-use` reuses the autonomy enum.** The internal direction asks the
  same question (`any | assisted | none`) pointed the other way, so it needs no
  vocabulary of its own.

## What the policy file renders

```text
# AI and LLM Policy
[cross-language bar]

<statement, verbatim>                       (omitted when unset)
Full policy: <links[type=ai-policy]>        (omitted when unset)

## Contributions made with AI assistance
<one sentence from attitude>
<one sentence from autonomy>
<maintainer-exemption sentence>             (only when applies-to: contributors)
<disclosure paragraph + detail list + trailer block>  (only when required)
<accepted-issue gate>                       (only when issue-required)
<excluded labels>                           (omitted when unset)
<obligations, one bullet each>              (omitted when unset)
<table: activity | stance>                  (only rows differing from attitude)

## When this policy is not followed         (omitted when unset)
<enforcement, one bullet each, in declared order>

## How this project uses AI                 (omitted when unset)
<table: activity | who decides>

## Using this project’s content
<content-signals, one bullet each, each with its meaning>
<or: no signals declared — absence is not permission>

## Questions
<contact from people[roles=community]>
```

Three rendering rules carry the design:

- **Only overrides reach the activity table.** A row is rendered when its value
  differs from `attitude`, not when it is merely declared. A project that
  spelled out nine activities identical to its attitude gets a
  three-paragraph file, and a project with two real exceptions gets exactly two
  rows. A table of nine rows all saying the same word is noise a reader learns
  to skip.
- **Every `project-use` entry renders.** There is no document-level default for
  the internal direction, so an entry there can never be repeating one.
- **Absence of a signal is stated, never implied.** The content section always
  renders. When `content-signals` is unset it says so, and says that silence is
  not consent and that the licence governs reuse independently — a reader
  arriving from a crawler needs the sentence more than a permissive project
  does.

## Bridge contract

| Concern | Decision |
| --- | --- |
| Kind | `core.Renderer` (derive-only; no read-back path) |
| Filename | `filename` from the document; default `core.FileAIPolicy` |
| Aliases | `AI.md`, `AI-POLICY.md`, `LLM.md` for `pf-bridge` lookup |
| Policy | `core.Policy{Marker: true}` |
| Gate | namespace absent → empty `Output`, existing file never deleted |
| Localization | `core.RenderLocalized` + `LocalizedSpec`, `docs/<lang>/<filename>` |

`Marker`, not `ScaffoldOnce`: the file is a projection of declared fields, so
flipping `attitude` MUST change the file on the next run. `CONTRIBUTING.md` is
`ScaffoldOnce` because it becomes the maintainer’s document; a policy that
became the maintainer’s document would be the drift this bridge exists to kill.

### The filename is data, the prose is not

`LocalizedSpec` carries `Template` beside `Filename`: the output name comes
from the document, the template name stays canonical. A project renaming its
file to `AI.md` still renders from `AI_POLICY.md.tmpl`, because the name a
project chose says nothing about which prose belongs in it.

`Bridge.Filename()` stays the DEFAULT name — the dispatcher resolves a bridge
before it has read a projectfile, so the registry identity cannot depend on
one. A project with an unusual name is still reachable as `pf-bridge ai-policy`.

The value is validated as a bare basename and REJECTED otherwise — never
sanitized. This field names a file a tool writes, so a separator turns a
metadata field into a write primitive.

### Prose in templates, data in Go

Every enum token is data; its meaning is prose. Go emits stable tokens —
`{Activity: "pull-requests", Stance: "prohibited"}` — and the template maps
both halves through `{{define}}` blocks, exactly as `SUPPORT.md` maps its row
`Kind` to a localized question. Consequences:

- A new activity key needs no Go edit. The template’s fallback prints the raw
  key, so an unknown activity renders as itself rather than vanishing.
- A new locale is one template file, as everywhere else in this bridge.

### Determinism

Activities and `project-use` are maps, and a map-ordered render fails `--check`
at random. Sort: known activities in the canonical declared order first,
unknown keys alphabetically after. Sequence fields keep their declared order
(that order is the meaning for `enforcement`) with duplicates dropped.

An unrecognised enum value warns and renders raw. Refusing to render is the
wrong failure: the reader loses the whole policy over one typo. Open-vocabulary
fields are not checked at all — an unknown value there is the spec working.

## Integration

- **readme** — `healthFiles(pf)` appends the document’s own policy name, so a
  renamed file is still discovered and linked. The catalog key is derived from
  the basename (`AI_POLICY.md` → `policy.ai-policy`).
- **contributing** — a one-line pointer to the policy file, NOT a copy of the
  policy. Two documents restating one policy is a drift source, and the bridge
  that owns the policy should be the only one stating it.
- **security** — no change. An LLM-authored vulnerability report is governed by
  the `security-reports` activity, which renders with the rest of the stance.

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
- **Per-activity autonomy.** `obligations: [human-reply]` covers the case that
  motivated it (a human answers human feedback, whatever opened the thread).
  Add it when a project needs more than that.
