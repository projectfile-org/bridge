<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# One place for your package metadata

- Your JavaScript or TypeScript `package.json` follows the projectfile, and edits there flow back — the version is never newer in one place than in the other.
- Your PHP `composer.json` stays in the same sync, so Packagist sees what the projectfile says.
- Your Python `pyproject.toml` stays in sync too, while the sections you hand-edit remain exactly as you left them.
- Your Crystal `shard.yml` is covered as well: a shard release starts from the projectfile, not from a second copy of the same facts.
