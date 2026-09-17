<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

<!-- textlint-disable terminology,common-misspellings -->

# Un solo lugar para los metadatos de tu paquete

- Tu `package.json` de JavaScript o TypeScript sigue al projectfile, y los cambios allí también vuelven — la versión nunca es más nueva en un lado que en el otro.
- Tu `composer.json` de PHP se mantiene en la misma sincronización, para que Packagist vea lo que dice el projectfile.
- Tu `pyproject.toml` de Python también se sincroniza, mientras las secciones que editas a mano quedan exactamente como las dejaste.
- Tu `shard.yml` de Crystal también está cubierto: una versión del shard parte del projectfile, no de una segunda copia de los mismos datos.

<!-- textlint-enable -->
