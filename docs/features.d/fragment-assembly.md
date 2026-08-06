<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# Feature and roadmap fragment assembly

- Assembles `docs/*.d/*.md` fragment files into composite documents such as FEATURES.md and ROADMAP.md.
- Nests content inherited from upstream parent repositories by release tag, with the vendored copies committed for offline reproducibility.
- Refreshing inherited copies needs only the parent’s newest tag and one archive fetch — no forge API or per-forge URL table.
