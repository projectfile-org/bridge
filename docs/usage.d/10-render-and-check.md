<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

# Render a `README.md` and catch drift

`check` compares every declared file with what the projectfile would render and writes nothing, so it fits a pre-commit hook or a CI gate.

```console
$ pf-bridge readme README.md --create-all --quiet
✓ bridge: README.md (created)
$ pf-cli set identity.summary 'A demo project'
✓ set identity.summary = A demo project
$ pf-bridge check readme --fail-on-drift --quiet 2>&1 | head -n 11
WARN  drift: file no longer matches the projectfile file=README.md
  --- README.md (on disk)
  +++ README.md (from the projectfile)
  @@ -6,7 +6,7 @@

   # org.example/demo

  -Demo project
  +A demo project

   ## License
$ pf-bridge readme README.md --quiet
✓ bridge: README.md (updated)
$ pf-bridge check readme --fail-on-drift --quiet
✓ bridge: README.md (in sync)
```
