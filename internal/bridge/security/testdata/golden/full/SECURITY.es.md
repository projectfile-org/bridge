<!--
SPDX-FileCopyrightText: 2026 this project
SPDX-License-Identifier: MIT
-->

<!-- pf-cli-managed: yes -->

[English](SECURITY.md) · [Українська](SECURITY.uk.md)

# Política de seguridad

## Versiones con soporte

Las siguientes versiones de this project reciben actualizaciones de seguridad:

- \>= 2.0 (current)
- 1.x (security only)

## Cómo informar de una vulnerabilidad

**No informes de vulnerabilidades de seguridad a través de incidencias, debates o solicitudes de cambio públicos.**

Hazlo escribiendo a **<security@example.org>**.

Incluye toda la información que puedas de la siguiente lista; nos ayuda a clasificar y resolver el informe más rápido:

- El tipo de problema (p. ej. desbordamiento de búfer, inyección SQL, cross-site scripting)
- La versión o versiones afectadas
- El impacto del problema, incluido cómo podría explotarlo un atacante
- Instrucciones paso a paso para reproducir el problema
- La ubicación del código fuente afectado (etiqueta, rama, commit o URL directa)
- Las rutas completas de los archivos fuente relacionados con el problema
- Cualquier configuración necesaria para reproducir el problema
- Archivos de registro relevantes, si es posible
- Código de prueba de concepto o de explotación, si es posible

Procuramos acusar recibo de los informes en un plazo de 14 days y coordinar
la divulgación en cuanto exista una corrección.

## Cifrar un informe

Si quieres enviarnos un informe cifrado, sigue estos pasos.

Importa nuestra clave pública:

```sh
gpg --keyserver keys.openpgp.org --recv-keys B64C122EE16C3746
```

O descárgala directamente e impórtala:

```sh
gpg --import pubkey.asc   # descargada de https://example.org/pubkey.asc
```

Verifica que la huella coincide antes de confiar en ella:

```sh
gpg --fingerprint B64C122EE16C3746
```

La salida debe mostrar:

```text
155E 3428 F7AC 5533 6D9A  1E8C B64C 122E E16C 3746
```

Cifra tu mensaje para nosotros:

```sh
gpg --encrypt --armor --recipient B64C122EE16C3746 message.txt
```

## Programa de recompensas

this project participa en un programa de recompensas por errores — consulta
https://example.org/.well-known/security.txt para conocer el alcance y las recompensas.

## Vulnerabilidades reconocidas

Los siguientes hallazgos fueron revisados y se suprimen de forma intencionada
(la corrección depende de una versión posterior del proyecto base o el aviso no
aplica a este proyecto):

| ID | Motivo |
| --- | --- |
| CVE-2024-12345 | Vulnerable code path is unreachable in this project |
| GHSA-abcd-1234-efgh | Fixed in pinned upstream v1.2.3 |
| GO-2024-0001 | — |
