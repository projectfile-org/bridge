<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# Ensamblado de fragmentos de características y hoja de ruta

- Ensambla los fragmentos `docs/*.d/*.md` en documentos compuestos como FEATURES.md y ROADMAP.md.
- Anida el contenido heredado de repositorios padres por etiqueta de versión, con las copias vendorizadas confirmadas para reproducibilidad sin red.
- Refrescar las copias heredadas solo necesita la etiqueta más reciente del padre y una consulta de archivo: sin API de forja ni tabla de URL por forja.
