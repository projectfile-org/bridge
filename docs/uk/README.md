<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

<!-- textlint-disable terminology,common-misspellings -->

[English](../../README.md) · [Español](../es/README.md)

# Projectfile Bridges

pf-bridge проєктує projectfile на файли, forge-майданчики та репозиторій

[![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://damian-buho.github.io/support-ukraine/) [![Projectfile inside](https://badges.kiota.ch/static/v1?label=projectfile&message=inside&labelColor=0d0d0d&color=8c6723&style=flat-square)](https://projectfile.org) [![License](https://badges.kiota.ch/static/v1?label=license&message=MIT&color=1e5913&style=flat-square)](LICENSE) ![Commit style](https://badges.kiota.ch/static/v1?label=commits&message=conventional&color=1877aa&style=flat-square) ![Workflow](https://badges.kiota.ch/static/v1?label=workflow&message=git-flow&color=1877aa&style=flat-square) ![Versioning](https://badges.kiota.ch/static/v1?label=versioning&message=semantic&color=1877aa&style=flat-square) [![PRs welcome](https://badges.kiota.ch/static/v1?label=PRs&message=welcome&color=1e5913&style=flat-square)](CONTRIBUTING.md) [![Citation](https://badges.kiota.ch/static/v1?label=citation&message=cff&color=1877aa&style=flat-square)](CITATION.cff) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/bridge)](https://api.reuse.software/info/codeberg.org/projectfile/bridge)

![Project status](https://badges.kiota.ch/static/v1?label=status&message=maintained&color=1d63ed&style=flat-square) [![Last commit on kiota.ch](https://badges.kiota.ch/gitea/last-commit/projectfile/bridge?gitea_url=https://kiota.ch&style=flat-square)](https://kiota.ch/projectfile/bridge) [![Latest release](https://badges.kiota.ch/gitea/v/release/projectfile/bridge?gitea_url=https://codeberg.org&style=flat-square)](https://codeberg.org/projectfile/bridge/releases) [![Last commit on GitHub](https://badges.kiota.ch/github/last-commit/projectfile-org/bridge?style=flat-square)](https://github.com/projectfile-org/bridge)

[![Publish pipeline on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/published.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Vulnerability audit on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/audited.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Dependency freshness on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions) [![Analysis sweep on GitHub](https://github.com/projectfile-org/bridge/actions/workflows/analyze.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/bridge/actions)

[![Publish pipeline on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/published.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Vulnerability audit on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/audited.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Dependency freshness on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions) [![Analysis sweep on kiota.ch](https://kiota.ch/projectfile/bridge/badges/workflows/analyze.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/bridge/actions)

## Можливості

- Ідентичність форжу, сканування та каркаси
- Збирання фрагментів можливостей і дорожньої карти
- Проєкція projectfile у файли

Див. [FEATURES.md](FEATURES.md), щоб переглянути повний перелік.

## Що надає цей проєкт

- **Виконуваний файл** `pf-bridge`
- **Образ контейнера** `ghcr.io/projectfile-org/bridge:latest`

## Встановлення

Завантажте опублікований образ контейнера:

### Завантажити з GHCR

```sh
docker pull ghcr.io/projectfile-org/bridge:latest
```

Стабільні випуски також публікують теґи `X.Y.Z`, `X.Y` і `X` — завантажте той рівень точності, який хочете зафіксувати.

Якщо наведені вище реєстри недоступні, завантажте з джерела:

### Завантажити з Kiota

```sh
docker pull kiota.ch/projectfile/bridge:latest
```

## Використання

Project the projectfile onto the files a forge expects:

```sh
pf-bridge --list
pf-bridge readme --force
pf-bridge all
```

## Збирання

Виконайте `make` без аргументів для типової цілі; виконайте `make help`, щоб переглянути всі цілі.

Для локального циклу розробки `make dev-container` піднімає dev-container.

Точки входу конвеєра:

- `make analyze` — Run the heavy analysis sweep (mutation testing, benchmarks)
- `make audited` — Re-scan the pinned dependencies and published artifacts for new vulnerabilities
- `make check-outdated` — Report every pinned dependency that lags upstream
- `make ready-to-publish` — Run the pseudo-CI pipeline locally — build, test and scan, without publishing

## Політики

- [Як зробити внесок](CONTRIBUTING.md)
- [Політика безпеки](SECURITY.md)
- [Як отримати підтримку](SUPPORT.md)
- [Кодекс поведінки](CODE_OF_CONDUCT.md)
- [Політика щодо ШІ та LLM](AI_POLICY.md)

## Посилання

- [Специфікація Projectfile](https://projectfile.org)

## Ліцензія

Цей проєкт ліцензовано на умовах MIT — див. файл [LICENSE](LICENSE) для подробиць.

<!-- textlint-enable -->
