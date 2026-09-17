<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

<!-- textlint-disable terminology,common-misspellings -->

[English](../../README.md) · [Українська](../uk/README.md)

# Projectfile Bridges

pf-bridge proyecta el projectfile sobre archivos, forges y el repositorio

[![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://damian-buho.github.io/support-ukraine/) [![Projectfile inside](https://badges.kiota.ch/static/v1?label=projectfile&message=inside&labelColor=0d0d0d&color=8c6723&style=flat-square)](https://projectfile.org) [![License](https://badges.kiota.ch/static/v1?label=license&message=MIT&color=1e5913&style=flat-square)](LICENSE) ![Commit style](https://badges.kiota.ch/static/v1?label=commits&message=conventional%20v1.0.0&color=1877aa&style=flat-square) ![Workflow](https://badges.kiota.ch/static/v1?label=workflow&message=git-flow&color=1877aa&style=flat-square) ![Versioning](https://badges.kiota.ch/static/v1?label=versioning&message=semantic%20v2.0.0&color=1877aa&style=flat-square) [![PRs welcome](https://badges.kiota.ch/static/v1?label=PRs&message=welcome&color=1e5913&style=flat-square)](CONTRIBUTING.md) [![Cosign](https://badges.kiota.ch/static/v1?label=cosign&message=enabled&color=1e5913&style=flat-square)](https://docs.sigstore.dev/cosign/verifying/verify/) [![Citation](https://badges.kiota.ch/static/v1?label=citation&message=cff&color=1877aa&style=flat-square)](CITATION.cff) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/bridge)](https://api.reuse.software/info/codeberg.org/projectfile/bridge)

![Project status](https://badges.kiota.ch/static/v1?label=status&message=maintained&color=1d63ed&style=flat-square) [![Last commit on kiota.ch](https://badges.kiota.ch/gitea/last-commit/projectfile/bridge?gitea_url=https://kiota.ch&label=last%20commit%20on%20kiota.ch&style=flat-square)](https://kiota.ch/projectfile/bridge) [![Latest release](https://badges.kiota.ch/gitea/v/release/projectfile/bridge?gitea_url=https://codeberg.org&style=flat-square)](https://codeberg.org/projectfile/bridge/releases) [![Last commit on GitHub](https://badges.kiota.ch/github/last-commit/projectfile-org/bridge?label=last%20commit%20on%20GitHub&style=flat-square)](https://github.com/projectfile-org/bridge)

[![Publish pipeline on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/published.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Vulnerability audit on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/audited.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Dependency freshness on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Analysis sweep on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/analyze.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions)

[![Publish pipeline on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/published.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Vulnerability audit on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/audited.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Dependency freshness on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Analysis sweep on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/analyze.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions)

## Características

- Documentos comunitarios sin copiar y pegar
- Documentos de características y de ruta que incluyen a tus padres
- Cada espejo de forja cuenta la misma historia
- Licenciado y citable
- Un solo lugar para los metadatos de tu paquete
- Un readme que sigue el ritmo del proyecto
- Empieza en minutos, mantén la sincronía después
- Cada herramienta lee las mismas listas

Consulta [FEATURES.md](FEATURES.md) para ver la lista completa.

## Qué entrega este proyecto

- **Ejecutable** `pf-bridge`
- **Imagen de contenedor** `ghcr.io/projectfile-org/bridge:latest`

## Plataformas admitidas

- `linux/amd64`
- `linux/arm64`
- `linux/riscv64`

## Instalación

Descarga la imagen de contenedor publicada:

### Descargar de GHCR

```sh
docker pull ghcr.io/projectfile-org/bridge:latest
```

Las versiones estables también publican las etiquetas `X.Y.Z`, `X.Y` y `X`: descarga el nivel de precisión que quieras fijar.

Si los registros anteriores no están disponibles, descarga desde el origen:

### Descargar de Kiota

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

Ejecuta `make` sin argumentos para el destino predeterminado; ejecuta `make help` para listar todos los destinos.

Para el bucle de desarrollo local, `make dev-container` levanta el dev-container.

Puntos de entrada de la canalización:

- `make analyze` — Run the heavy analysis sweep (mutation testing, benchmarks)
- `make audited` — Re-scan the pinned dependencies and published artifacts for new vulnerabilities
- `make check-outdated` — Report every pinned dependency that lags upstream
- `make ready-to-publish` — Run the pseudo-CI pipeline locally — build, test and scan, without publishing

## Políticas

- [Cómo contribuir](CONTRIBUTING.md)
- [Política de seguridad](SECURITY.md)
- [Cómo obtener ayuda](SUPPORT.md)
- [Código de conducta](CODE_OF_CONDUCT.md)
- [Política sobre IA y LLM](AI_POLICY.md)

## Enlaces

- [Especificación de Projectfile](https://projectfile.org)

## Licencia

Este proyecto se publica bajo la licencia MIT — consulta el archivo [LICENSE](LICENSE) para más detalles.

<!-- textlint-enable -->
