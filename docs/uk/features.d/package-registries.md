<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

<!-- textlint-disable terminology,common-misspellings -->

# Одне місце для метаданих твого пакунка

- Твій `package.json` для JavaScript чи TypeScript слідує за projectfile, а зміни там повертаються назад — версія ніколи не новіша з одного боку, ніж з іншого.
- Твій `composer.json` для PHP тримається в тій самій синхронізації, щоб Packagist бачив те, що каже projectfile.
- Твій `pyproject.toml` для Python теж синхронізується, а розділи, які редагуєш вручну, лишаються саме такими, як ти їх залишив.
- Твій `shard.yml` для Crystal теж покрито: реліз шарда починається з projectfile, а не з другої копії тих самих даних.

<!-- textlint-enable -->
