<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

<!-- textlint-disable terminology -->

[English](README.md) · [Українська](docs/uk/README.md)

# projectfile/bridge

pf-bridge projects the projectfile onto files, forges, and the repository

[![License](https://img.shields.io/badge/license-MIT-4c1?style=flat-square)](LICENSE) [![PRs welcome](https://img.shields.io/badge/PRs-welcome-4c1?style=flat-square)](CONTRIBUTING.md) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/bridge)](https://api.reuse.software/info/codeberg.org/projectfile/bridge)

![Project status](https://img.shields.io/badge/status-maintained-1d63ed?style=flat-square) [![Last commit](https://img.shields.io/gitea/last-commit/projectfile/bridge?gitea_url=https://codeberg.org&style=flat-square)](https://codeberg.org/projectfile/bridge)

[![Build status on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/published.yaml/badge.svg)](https://kiota.ch/projectfile/bridge/actions)

## Características

- Forge identity, scanning and scaffolding
- Feature and roadmap fragment assembly
- Projectfile-to-file projection

Consulta [Características](FEATURES.md) para ver la lista completa.

## Qué entrega este proyecto

- **Ejecutable** `pf-bridge`
- **Imagen de contenedor** `kiota.ch/projectfile/bridge:latest`

## Instalación

Pull the published container image:

```sh
docker pull kiota.ch/projectfile/bridge:latest
```

## Uso

Project the projectfile onto the files a forge expects:

```sh
pf-bridge --list
pf-bridge readme --force
pf-bridge all
```

## Compilación

- [Referencia del Makefile](docs/MAKEFILE.md)

Puntos de entrada de la canalización:

- `make analyze` — Run the heavy analysis sweep (mutation testing, benchmarks)
- `make audited` — Re-scan the pinned dependencies and published artifacts for new vulnerabilities
- `make check-outdated` — Report every pinned dependency that lags upstream
- `make published` — Build, test, scan and publish the release artifacts

Ejecuta `make` sin argumentos para el destino predeterminado; ejecuta `make help` para listar todos los destinos.

Para el bucle de desarrollo local, `make ci-dag M6E_CI_TARGETS=dev` levanta el dev-container.

## Políticas

- [Cómo contribuir](docs/es/CONTRIBUTING.md)
- [Política de seguridad](docs/es/SECURITY.md)
- [Cómo obtener ayuda](docs/es/SUPPORT.md)
- [Código de conducta](docs/es/CODE_OF_CONDUCT.md)

## Enlaces

- [Source Code on Codeberg](https://codeberg.org/projectfile/bridge)
- [Source Code on GitHub](https://github.com/damian-buho/projectfile-bridge)
- [Source Code on kiota.ch](https://kiota.ch/projectfile/bridge)
- [Issues on Codeberg](https://codeberg.org/projectfile/bridge/issues)
- [Issues on GitHub](https://github.com/damian-buho/projectfile-bridge/issues)

## Licencia

Este proyecto se publica bajo la licencia MIT — consulta el archivo [LICENSE](LICENSE) para más detalles.

<!-- textlint-enable -->
