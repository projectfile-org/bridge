<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

[Español](docs/es/README.md) · [Українська](docs/uk/README.md)

# projectfile/bridge

pf-bridge projects the projectfile onto files, forges, and the repository

[![License](https://img.shields.io/badge/license-MIT-4c1?style=flat-square)](LICENSE) [![PRs welcome](https://img.shields.io/badge/PRs-welcome-4c1?style=flat-square)](CONTRIBUTING.md) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/bridge)](https://api.reuse.software/info/codeberg.org/projectfile/bridge)

![Project status](https://img.shields.io/badge/status-maintained-1d63ed?style=flat-square) [![Last commit](https://img.shields.io/gitea/last-commit/projectfile/bridge?gitea_url=https://codeberg.org&style=flat-square)](https://codeberg.org/projectfile/bridge)

[![Build status on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/published.yaml/badge.svg)](https://kiota.ch/projectfile/bridge/actions)

## Features

- Forge identity, scanning and scaffolding
- Feature and roadmap fragment assembly
- Projectfile-to-file projection

See [Features](FEATURES.md) for the full list.

## What this provides

- **Executable** `pf-bridge`
- **Container image** `kiota.ch/projectfile/bridge:latest`

## Installation

Pull the published container image:

```sh
docker pull kiota.ch/projectfile/bridge:latest
```

## Usage

Project the projectfile onto the files a forge expects:

```sh
pf-bridge --list
pf-bridge readme --force
pf-bridge all
```

## Building

- [Makefile reference](docs/MAKEFILE.md)

Pipeline entry points:

- `make analyze` — Run the heavy analysis sweep (mutation testing, benchmarks)
- `make audited` — Re-scan the pinned dependencies and published artifacts for new vulnerabilities
- `make check-outdated` — Report every pinned dependency that lags upstream
- `make published` — Build, test, scan and publish the release artifacts

Run `make` with no arguments for the default target; run `make help` to list every target.

For the local dev loop, `make ci-dag M6E_CI_TARGETS=dev` brings up the dev-container.

## Policies

- [How to contribute](CONTRIBUTING.md)
- [Security policy](SECURITY.md)
- [Getting support](SUPPORT.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)

## Links

- [Source Code on Codeberg](https://codeberg.org/projectfile/bridge)
- [Source Code on GitHub](https://github.com/damian-buho/projectfile-bridge)
- [Source Code on kiota.ch](https://kiota.ch/projectfile/bridge)
- [Issues on Codeberg](https://codeberg.org/projectfile/bridge/issues)
- [Issues on GitHub](https://github.com/damian-buho/projectfile-bridge/issues)

## License

This project is licensed under MIT — see the [LICENSE](LICENSE) file for details.
