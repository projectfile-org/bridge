<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

<!-- textlint-disable terminology,common-misspellings -->

[English](../../FEATURES.md) · [Українська](../uk/FEATURES.md)

# Características

## Características del proyecto

### Documentos comunitarios sin copiar y pegar

- La guía de contribución, el código de conducta, la política de seguridad y la página de ayuda se generan desde una sola declaración, así nunca se contradicen.
- Una política de uso de IA declara en un solo archivo qué ayuda es bienvenida y qué está fuera de los límites, en vez de notas dispersas por el repositorio.
- Una declaración de diversidad e inclusión parte de la plantilla comunitaria y lleva tus propias notas de esfuerzo.
- Cada archivo anterior se genera en los idiomas que elijas — el español y el ucraniano vienen incluidos, y cualquier otro idioma sigue el mismo camino cuando aportas su redacción.
- La propiedad y el patrocinio también están cubiertos: `CODEOWNERS` dirige las revisiones, y los archivos de financiación señalan a los patrocinadores cada lugar donde aceptas ayuda.

### Documentos de características y de ruta que incluyen a tus padres

- Escribes un archivo corto por característica, y el documento general se ensambla solo — incluida la sección del readme que lo cita.
- Lo que publican tus proyectos padres aparece bajo su propio nombre, actualizado cada vez que regeneras.
- Comprobar cambios nunca necesita red: relee el documento confirmado y muestra qué se desvió.

### Cada espejo de forja cuenta la misma historia

- La descripción, la página principal y los temas en GitHub, GitLab y Forgejo se envían desde el projectfile — edita una vez y los tres espejos coinciden.
- Los ajustes del repositorio viven junto al código que describen, bajo control de versiones, en vez de en tres formularios web separados.
- Un espejo que no tienes simplemente no recibe nada: sin errores y sin marcadores vacíos.

### Licenciado y citable

- `LICENSE` se escribe con tu titular de derechos y tu año en su lugar, para que el archivo publicado sea el texto final, no una plantilla.
- El texto completo de cada término de licencia que declares se publica junto a él bajo `LICENSES/`, un archivo por término.
- `CITATION.cff` se mantiene sincronizado, para que los investigadores citen el proyecto correctamente sin esfuerzo extra de tu parte.

### Un solo lugar para los metadatos de tu paquete

- Tu `package.json` de JavaScript o TypeScript sigue al projectfile, y los cambios allí también vuelven — la versión nunca es más nueva en un lado que en el otro.
- Tu `composer.json` de PHP se mantiene en la misma sincronización, para que Packagist vea lo que dice el projectfile.
- Tu `pyproject.toml` de Python también se sincroniza, mientras las secciones que editas a mano quedan exactamente como las dejaste.
- Tu `shard.yml` de Crystal también está cubierto: una versión del shard parte del projectfile, no de una segunda copia de los mismos datos.

### Un readme que sigue el ritmo del proyecto

- Las instrucciones de instalación y uso describen lo que el proyecto realmente publica — una imagen de contenedor, un paquete o un binario — con una receta por cada forma de obtenerlo.
- Los proyectos de contenedores reciben una línea de descarga por cada registro donde publican, para que no falte ningún destino ni quede ninguno obsoleto.
- Las insignias, los proyectos relacionados y los enlaces a los documentos comunitarios se ensamblan solos desde la misma declaración.
- La lista de características aparece dentro del readme, en el idioma del lector cuando existe una traducción.

### Empieza en minutos, mantén la sincronía después

- Un proyecto nuevo parte de un asistente interactivo que detecta el ecosistema y propone enlaces de forja y registros desde la dirección del repositorio y la pila.
- Los escáneres leen la copia de trabajo — la pila, los autores, los remotos — de vuelta al projectfile, para que el documento empiece cierto y siga cierto.
- Un solo comando actualiza cada archivo generado; nombrar uno actualiza solo ese archivo.
- Una comprobación genera sin escribir y muestra cada archivo cambiado o ausente como diff, fallando la construcción cuando se lo pides.

### Cada herramienta lee las mismas listas

- Los archivos de exclusión para Git, contenedores, npm y asistentes de IA nacen de una sola lista de inclusiones y exclusiones — una ruta ignorada en un lado se ignora donde importa.
- Las configuraciones de lint y de editor como yamllint, browserslist y los atributos de Git derivan de la misma declaración.
- Una vulnerabilidad que suprimes una vez queda suprimida por igual en Trivy, Grype, OSV-Scanner y audit-ci.
- La automatización de versiones también mantiene una sola forma, para que los aumentos de versión y las notas de cada versión se comporten igual siempre.
<!-- textlint-enable -->
