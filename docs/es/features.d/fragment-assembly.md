<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

<!-- textlint-disable terminology,common-misspellings -->

# Ensamblado de fragmentos de características y hoja de ruta

- Ensambla los fragmentos `docs/*.d/*.md` en documentos compuestos como FEATURES.md y ROADMAP.md.
- Anida lo que publican los repositorios padres, obtenido en vivo a su etiqueta de versión más reciente — el documento confirmado es el único registro, así que regenerar es como llegan los cambios del padre.
- La obtención solo necesita la etiqueta más reciente del padre y una consulta de archivo — sin API de forja ni tabla de URL por forja — y la verificación permanece sin red releyendo las secciones confirmadas.

<!-- textlint-enable -->
