<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

<!-- textlint-disable terminology,common-misspellings -->

# Cada espejo de forja cuenta la misma historia

- La descripción, la página principal y los temas en GitHub, GitLab y Forgejo se envían desde el projectfile — edita una vez y los tres espejos coinciden.
- La protección de ramas y etiquetas sigue las ramas de publicación que el projectfile ya declara, y el origen se abre en el tronco: la rama estable solo acepta historia firmada de su propietario, y ningún espejo deja reescribir ni borrar una rama de publicación.
- Los ajustes del repositorio viven junto al código que describen, bajo control de versiones, en vez de en tres formularios web separados.
- Un espejo que no tienes simplemente no recibe nada: sin errores y sin marcadores vacíos.

<!-- textlint-enable -->
