<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

<!-- textlint-disable terminology,common-misspellings -->

# Cada herramienta lee las mismas listas

- Los archivos de exclusión para Git, contenedores, npm y asistentes de IA nacen de una sola lista de inclusiones y exclusiones — una ruta ignorada en un lado se ignora donde importa.
- Las configuraciones de lint y de editor como yamllint, browserslist y los atributos de Git derivan de la misma declaración.
- Una vulnerabilidad que suprimes una vez queda suprimida por igual en Trivy, Grype, OSV-Scanner y audit-ci.
- La automatización de versiones también mantiene una sola forma, para que los aumentos de versión y las notas de cada versión se comporten igual siempre.

<!-- textlint-enable -->
