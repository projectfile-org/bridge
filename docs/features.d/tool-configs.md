<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# Every tool reads the same lists

- Ignore files for Git, containers, npm, and AI assistants all come from one include and exclude list — a path ignored in one place is ignored everywhere it matters.
- Lint and editor configs such as yamllint, browserslist, and Git attributes derive from the same declaration.
- A vulnerability you suppress once stays suppressed in Trivy, Grype, OSV-Scanner, and audit-ci alike.
- Release automation keeps one shape too, so version bumps and release notes behave the same on every release.
