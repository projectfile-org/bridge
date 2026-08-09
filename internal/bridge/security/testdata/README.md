<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

# SECURITY.md test fixtures

Test inputs the security bridge `pgp_test.go` loads. The toolchain ignores
`testdata/`.

| Fixture | Used by |
| --- | --- |
| `B64C122EE16C3746.asc` | `pgp_test.go` — a sample GPG public key for the PGP-key rendering path |
