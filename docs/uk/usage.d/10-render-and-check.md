<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

# Згенеруйте `README.md` і виявіть розбіжність

`check` порівнює кожен оголошений файл із тим, що згенерував би projectfile, і нічого не записує, тож підходить для pre-commit-хука чи перевірки в CI.

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
