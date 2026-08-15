<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# Proyección de projectfile a archivos

- Genera y mantiene ida y vuelta los archivos externos que una forja espera a partir de un único documento projectfile: manifiestos de paquetes, CITATION.cff, LICENSE más los textos por licencia SPDX, la familia gitignore, el readme y los documentos comunitarios.
- Sincroniza cada archivo externo de forma bidireccional; un solo comando actualiza todos, o uno por nombre.
- Una verificación de deriva renderiza sin escribir y reporta cada archivo desviado o ausente como diff unificado, fallando ante deriva cuando se le pide.
