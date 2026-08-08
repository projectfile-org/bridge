<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

# SECURITY.md render fixtures

Input `projectfile.yaml` + rendered golden output, so a template change can be
checked against a known-good result. The toolchain ignores `testdata/`.

## Layout

| Fixture | Exercises |
| --- | --- |
| `golden/full/` | Every feature at once: `supported-versions`, `report-url` off (email branch), `disclosure-window`, `gpg-key` + `gpg-fingerprint`, `links[].type=pgp-key`, `bug-bounty-url`, and three `vulnerabilities.suppress` entries (one reason-less). Renders EN + ES + UK. |
| `golden/minimal/` | The gating case: only a contact + `report-url`. No Supported Versions, no Encrypting section, no Bug Bounty, no Acknowledged table; default 30-day window. EN only. |

## Regenerate

```sh
# from bridge/, after `go build ./cmd/pf-bridge-security`
for d in testdata/golden/full testdata/golden/minimal; do
  ./pf-bridge-security to SECURITY.md "$d" --force
done
```

Diff the trees to see what a template or view change did to the output. The
`# Security Policy` heading and the SPDX header are added by the renderer, not
the template; the language bar (`[ES](…) · [UK](…)`) is injected by
`RenderLocalized` when more than one language renders.
