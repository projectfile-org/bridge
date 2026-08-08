<!--
SPDX-FileCopyrightText: 2026 this project
SPDX-License-Identifier: MIT
-->

<!-- pf-cli-managed: yes -->

[Español](SECURITY.es.md) · [Українська](SECURITY.uk.md)

# Security Policy

## Supported Versions

The following this project versions currently receive security updates:

- \>= 2.0 (current)
- 1.x (security only)

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public issues, discussions, or change requests.**

Report them by emailing **<security@example.org>**.

Please include as much of the following as you can — it helps us triage and resolve the report faster:

- The type of issue (e.g. buffer overflow, SQL injection, cross-site scripting)
- Affected version(s)
- The impact of the issue, including how an attacker might exploit it
- Step-by-step instructions to reproduce the issue
- The location of the affected source code (tag, branch, commit, or direct URL)
- Full paths of the source file(s) related to the issue
- Any configuration required to reproduce the issue
- Relevant log files, if possible
- Proof-of-concept or exploit code, if possible

We aim to acknowledge reports within 14 days and to coordinate
disclosure once a fix is available.

## Encrypting a Report

If you would like to send us an encrypted report, follow these steps.

Import our public key:

```sh
gpg --keyserver keys.openpgp.org --recv-keys B64C122EE16C3746
```

Or download it directly and import it:

```sh
gpg --import pubkey.asc   # downloaded from https://example.org/pubkey.asc
```

Verify the fingerprint matches before you trust it:

```sh
gpg --fingerprint B64C122EE16C3746
```

The output must show:

```text
155E 3428 F7AC 5533 6D9A  1E8C B64C 122E E16C 3746
```

Encrypt your message to us:

```sh
gpg --encrypt --armor --recipient B64C122EE16C3746 message.txt
```

## Bug Bounty

this project participates in a bug bounty programme — see https://example.org/.well-known/security.txt
for scope and reward details.

## Acknowledged Vulnerabilities

The following findings were reviewed and are intentionally suppressed (a fix
depends on an upstream release, or the advisory does not apply to this project):

| ID | Reason |
| --- | --- |
| CVE-2024-12345 | Vulnerable code path is unreachable in this project |
| GHSA-abcd-1234-efgh | Fixed in pinned upstream v1.2.3 |
| GO-2024-0001 | — |
