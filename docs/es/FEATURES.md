<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

<!-- textlint-disable terminology,common-misspellings -->

[English](../../FEATURES.md) · [Українська](../uk/FEATURES.md)

# Características

## Características del proyecto

### Identidad de forja, escaneo y scaffolding

- Envía la identidad del proyecto a GitHub, GitLab y Forgejo.
- Ejecuta escáneres de sistema de archivos y de Git sobre la copia de trabajo.
- Genera de forma interactiva un projectfile nuevo, detectando el ecosistema e infiriendo enlaces de forja y registros a partir de la URL del repositorio y del stack.
- Genera las variantes traducidas de los documentos comunitarios desde una única declaración de idioma, con español y ucraniano incluidos de serie.

### Ensamblado de fragmentos de características y hoja de ruta

- Ensambla los fragmentos `docs/*.d/*.md` en documentos compuestos como FEATURES.md y ROADMAP.md.
- Anida el contenido heredado de repositorios padres por etiqueta de versión, con las copias vendorizadas confirmadas para reproducibilidad sin red.
- Refrescar las copias heredadas solo necesita la etiqueta más reciente del padre y una consulta de archivo: sin API de forja ni tabla de URL por forja.

### Proyección de projectfile a archivos

- Genera y mantiene ida y vuelta los archivos externos que una forja espera a partir de un único documento projectfile: manifiestos de paquetes, CITATION.cff, LICENSE más los textos por licencia SPDX, la familia gitignore, el readme y los documentos comunitarios.
- Sincroniza cada archivo externo de forma bidireccional; un solo comando actualiza todos, o uno por nombre.
- Una verificación de deriva renderiza sin escribir y reporta cada archivo desviado o ausente como diff unificado, fallando ante deriva cuando se le pide.
<!-- textlint-enable -->
