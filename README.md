<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

[Español](docs/es/README.md) · [Українська](docs/uk/README.md)

# Projectfile Bridges

pf-bridge is the projection tool of the projectfile: it derives forge identity files, assembles feature and roadmap fragments, and projects the projectfile onto the files a forge and repository expect. Companion binary to pf-cli for the projectfile.org tooling.

[![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://damian-buho.github.io/support-ukraine/) [![Projectfile inside](https://badges.kiota.ch/static/v1?label=projectfile&message=inside&labelColor=0d0d0d&color=8c6723&style=flat-square)](https://projectfile.org) [![License](https://badges.kiota.ch/static/v1?label=license&message=MIT&color=1e5913&style=flat-square)](LICENSE) [![Cosign](https://badges.kiota.ch/static/v1?label=cosign&message=enabled&color=1e5913&style=flat-square)](https://docs.sigstore.dev/cosign/verifying/verify/) [![PRs welcome](https://badges.kiota.ch/static/v1?label=PRs&message=welcome&color=1e5913&style=flat-square)](CONTRIBUTING.md) [![REUSE compliance](https://api.reuse.software/badge/github.com/projectfile-org/bridge)](https://api.reuse.software/info/github.com/projectfile-org/bridge)

![Project status](https://badges.kiota.ch/static/v1?label=status&message=maintained&color=1d63ed&style=flat-square) [![Last commit on GitHub](https://badges.kiota.ch/github/last-commit/projectfile-org/bridge?label=last%20commit%20on%20GitHub&style=flat-square)](https://github.com/projectfile-org/bridge) [![Last commit on kiota.ch](https://badges.kiota.ch/gitea/last-commit/projectfile/bridge?gitea_url=https://kiota.ch&label=last%20commit%20on%20kiota.ch&style=flat-square)](https://kiota.ch/projectfile/bridge)

[![Publish pipeline on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/published.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Vulnerability audit on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/audited.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Dependency freshness on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Analysis sweep on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/analyzed.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions)

[![Publish pipeline on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/published.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Vulnerability audit on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/audited.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Dependency freshness on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Analysis sweep on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/analyze.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions)

## Features

- Community files without the copypaste
- Feature and roadmap docs that include your parents
- Every forge mirror tells the same story
- Licensed and citable
- One place for your package metadata
- A readme that keeps up with the project
- Start in minutes, stay in sync afterwards
- Every tool reads the same lists

See [Features](docs/FEATURES.md) for the full list.

## What this provides

- **Executable** `pf-bridge` — command `pf-bridge`
- **Container image** `ghcr.io/projectfile-org/bridge:latest`
- **Container image** `damianbuho/projectfile-bridge:latest`

## Installation

### Container image

Pull the published container image:

#### Pull from GHCR — linux/amd64, linux/arm64, linux/riscv64

```sh
docker pull ghcr.io/projectfile-org/bridge:latest
```

#### Pull from DockerHub — linux/amd64

```sh
docker pull damianbuho/projectfile-bridge:latest
```

Stable releases also publish `X.Y.Z`, `X.Y` and `X` tags — pull the precision you want to pin.

If the registries above are unreachable, pull from the origin instead:

#### Pull from Kiota — linux/amd64

```sh
docker pull kiota.ch/projectfile/bridge:latest
```

### Prebuilt binary

Download the prebuilt binary for your platform from the latest GitHub release:

```sh
curl --fail --location --output pf-bridge https://github.com/projectfile-org/bridge/releases/latest/download/pf-bridge-$(uname -s | tr A-Z a-z)-$(uname -m | sed -e s/x86_64/amd64/ -e s/aarch64/arm64/) && chmod +x pf-bridge
./pf-bridge --help
```

Published for: `linux/amd64`, `linux/arm64`, `linux/riscv64`, `darwin/amd64`, `darwin/arm64`

## Usage

### pf-bridge

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
  cache        Manage the local SPDX and include cache
  forge        Push projectfile metadata out to the repository forge
  init         Scaffold a new projectfile document (prefer pf-cli init)
  scan         Run scanners to refresh projectfile metadata from filesystem signals
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

Examples and every command’s help are in [Usage](docs/USAGE.md).

## Building

Clone the repository with its submodules:

```sh
git clone --recurse-submodules https://github.com/projectfile-org/bridge bridge && cd bridge
```

Build the container image locally:

```sh
make container-build
```

- [Makefile reference](docs/how-to/MAKEFILE.md)

Run `make` with no arguments for the default target; run `make help` to list every target.

For the local dev loop, `make dev-container` brings up the dev-container.

Pipeline entry points:

- `make analyzed` — Run the heavy analysis sweep (mutation testing, benchmarks)
- `make audited` — Re-scan the pinned dependencies and published artifacts for new vulnerabilities
- `make check-outdated` — Report every pinned dependency that lags upstream
- `make ready-to-publish` — Run the pseudo-CI pipeline locally — build, test and scan, without publishing

## Policies

- [How to contribute](CONTRIBUTING.md)
- [Security policy](SECURITY.md)
- [Getting support](SUPPORT.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [AI and LLM Policy](AI_POLICY.md)

## Links

- [Projectfile Specification](https://projectfile.org)

## License

This project is licensed under MIT — see the [LICENSE](LICENSE) file for details.
