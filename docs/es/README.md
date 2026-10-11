<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

<!-- textlint-disable terminology,common-misspellings -->

[English](../../README.md) · [Українська](../uk/README.md)

# Projectfile Bridges

pf-bridge es la herramienta de proyección del projectfile: deriva los archivos de identidad del forge, ensambla los fragmentos de características y hoja de ruta, y proyecta el projectfile sobre los archivos que un forge y un repositorio esperan. Binario compañero de pf-cli para las herramientas de projectfile.org.

[![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://damian-buho.github.io/support-ukraine/) [![Projectfile inside](https://badges.kiota.ch/static/v1?label=projectfile&message=inside&labelColor=0d0d0d&color=8c6723&style=flat-square)](https://projectfile.org) [![License](https://badges.kiota.ch/static/v1?label=license&message=MIT&color=1e5913&style=flat-square)](LICENSE) [![Cosign](https://badges.kiota.ch/static/v1?label=cosign&message=enabled&color=1e5913&style=flat-square)](https://docs.sigstore.dev/cosign/verifying/verify/) [![PRs welcome](https://badges.kiota.ch/static/v1?label=PRs&message=welcome&color=1e5913&style=flat-square)](CONTRIBUTING.md) [![REUSE compliance](https://api.reuse.software/badge/github.com/projectfile-org/bridge)](https://api.reuse.software/info/github.com/projectfile-org/bridge)

![Project status](https://badges.kiota.ch/static/v1?label=status&message=maintained&color=1d63ed&style=flat-square) [![Last commit on GitHub](https://badges.kiota.ch/github/last-commit/projectfile-org/bridge?label=last%20commit%20on%20GitHub&style=flat-square)](https://github.com/projectfile-org/bridge) [![Last commit on kiota.ch](https://badges.kiota.ch/gitea/last-commit/projectfile/bridge?gitea_url=https://kiota.ch&label=last%20commit%20on%20kiota.ch&style=flat-square)](https://kiota.ch/projectfile/bridge)

[![Publish pipeline on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/published.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Vulnerability audit on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/audited.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Dependency freshness on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Analysis sweep on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/analyzed.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions)

[![Publish pipeline on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/published.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Vulnerability audit on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/audited.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Dependency freshness on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Analysis sweep on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/analyzed.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions)

## Características

- Documentos comunitarios sin copiar y pegar
- Documentos de características y de ruta que incluyen a tus padres
- Cada espejo de forja cuenta la misma historia
- Licenciado y citable
- Un solo lugar para los metadatos de tu paquete
- Un readme que sigue el ritmo del proyecto
- Notas de versión escritas desde tu historial
- Empieza en minutos, mantén la sincronía después
- Cada herramienta lee las mismas listas

Consulta [Características](FEATURES.md) para ver la lista completa.

## Qué entrega este proyecto

- **Ejecutable** `pf-bridge` — comando `pf-bridge`
- **Imagen de contenedor** `ghcr.io/projectfile-org/bridge:latest`
- **Imagen de contenedor** `damianbuho/projectfile-bridge:latest`

## Instalación

### Imagen de contenedor

Descarga la imagen de contenedor publicada:

#### Descargar de GHCR — linux/amd64, linux/arm64, linux/riscv64

```sh
docker pull ghcr.io/projectfile-org/bridge:latest
```

#### Descargar de DockerHub — linux/amd64

```sh
docker pull damianbuho/projectfile-bridge:latest
```

Las versiones estables también publican las etiquetas `X.Y.Z`, `X.Y` y `X`: descarga el nivel de precisión que quieras fijar.

Si los registros anteriores no están disponibles, descarga desde el origen:

#### Descargar de Kiota — linux/amd64

```sh
docker pull kiota.ch/projectfile/bridge:latest
```

### Binario precompilado

Descarga el binario precompilado para tu plataforma desde las versiones de GitHub:

```sh
mkdir -p ~/.local/bin
curl --fail --location --output ~/.local/bin/pf-bridge https://github.com/projectfile-org/bridge/releases/latest/download/pf-bridge-$(uname -s | tr A-Z a-z)-$(uname -m)
chmod +x ~/.local/bin/pf-bridge
~/.local/bin/pf-bridge --help
```

Publicado para: `linux/amd64`, `linux/arm64`, `linux/riscv64`, `darwin/amd64`, `darwin/arm64`

## Uso

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

Los ejemplos y la ayuda de cada comando están en [Uso](USAGE.md).

## Compilación

Clona el repositorio con sus submódulos:

```sh
git clone --recurse-submodules https://github.com/projectfile-org/bridge bridge && cd bridge
```

Construye la imagen de contenedor en local:

```sh
make container-build
```

- [Referencia del Makefile](../how-to/MAKEFILE.md)

Ejecuta `make` sin argumentos para el destino predeterminado; ejecuta `make help` para listar todos los destinos.

Para el bucle de desarrollo local, `make dev-container` levanta el dev-container.

Puntos de entrada de la canalización:

- `make analyzed` — Ejecuta el análisis pesado (pruebas de mutación, benchmarks)
- `make audited` — Vuelve a escanear las dependencias fijadas y los artefactos publicados en busca de vulnerabilidades nuevas
- `make check-outdated` — Informa de cada dependencia fijada que va por detrás de su versión upstream
- `make ready-to-publish` — Ejecuta localmente el pipeline pseudo-CI — compila, prueba y escanea, sin publicar

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
