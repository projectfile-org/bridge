<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

# pf-bridge

```console
$ pf-bridge --help
pf-bridge — keep every project file in sync with your projectfile document

A projectfile (projectfile.yaml) is the single source of truth for a project:
identity, people, links, and tool config. pf-bridge copies that data out
into the files each tool already reads — README.md, package.json,
CITATION.cff, forge settings — and reads two-way files back.
Spec: https://projectfile.org/specification/

Usage:
  pf-bridge <bridge> [args]   run one bridge (e.g. readme, npm, forge)
  pf-bridge all [flags]       update every file from the projectfile [rw]
  pf-bridge check [flags]     verify declared files match, write nothing [ro]
  pf-bridge to all            write projectfile out to every file [rw]
  pf-bridge from all          read every two-way file back in [rw]
  pf-bridge --list            list installed bridges
  pf-bridge completion SHELL  print completion script (bash|zsh|fish)

[rw] updates files, [ro] only reads or compares. `check` without names
covers the bridges this project declares; `check <name>…` narrows it,
`check --all` covers every installed bridge instead.

Shared flags (also accepted after a bridge name):
  --check, --dry-run, --force, --create-all, --fail-on-drift, --offline,
  --quiet, --verbose. Each has a PF_BRIDGE_* env equivalent.
  Drift warns by default; --fail-on-drift makes it fatal.

Tools:
  cache                Manage the local SPDX and include cache
  forge                Push projectfile metadata out to the repository forge
  init                 Scaffold a new projectfile document (prefer pf-cli init)
  release-notes        Render release notes from git
  scan                 Run scanners to refresh projectfile metadata from filesystem signals
Package manifests:
  composer   [rw]  composer.json — two-way sync
  npm        [rw]  package.json — two-way sync
  pyproject  [rw]  pyproject.toml — two-way sync
  shard      [rw]  shard.yml — two-way sync
Citation and ownership:
  cff          [rw]  CITATION.cff — two-way sync
  codeowners   [rw]  CODEOWNERS — two-way sync
  funding      [ro]  FUNDING.yml — one-way render
  fundingjson  [ro]  funding.json — one-way render
Documentation:
  ai-policy     [ro]  AI_POLICY.md — one-way render
  coc           [ro]  CODE_OF_CONDUCT.md — one-way render
  contributing  [ro]  CONTRIBUTING.md — one-way render
  dei           [ro]  DEI.md — one-way render
  fragments     [ro]  FEATURES.md, ROADMAP.md, … assembled from docs/<name>.d fragments (one-way render)
  llm           [ro]  LLM.md — one-way render
  readme        [ro]  README.md — one-way render
  security      [ro]  SECURITY.md — one-way render
  support       [ro]  SUPPORT.md — one-way render
Repository setup:
  browserslist     [ro]  .browserslistrc — one-way render
  gitattributes    [ro]  .gitattributes — one-way render
  ignore           [ro]  .claudeignore, .containerignore, .dockerignore, .fdignore, .gitignore, .npmignore, .textlintignore — one-way render
  license          [ro]  LICENSE + LICENSES/<spdx>.txt (one-way render, always overwrite)
  releaserc        [ro]  .releaserc.yaml — one-way render
  vulnerabilities  [ro]  .grype.yaml, .trivyignore, audit-ci.jsonc, osv-scanner.toml — one-way render
  yamllint         [ro]  .yamllint — one-way render

Help per bridge: pf-bridge <bridge> --help
```
