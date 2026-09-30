<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

[Español](docs/es/FEATURES.md) · [Українська](docs/uk/FEATURES.md)

# Features

## Project Features

### Community files without the copypaste

- The contributing guide, code of conduct, security policy, and support page are generated from one declaration, so they never contradict each other.
- An AI-use policy states in one file what assistance is welcome and what is off limits, instead of scattered notes across the repository.
- A diversity and inclusion statement starts from the community boilerplate and carries your own effort notes.
- Every file above renders in the languages you choose — Spanish and Ukrainian are included, and any other language follows the same path once you supply its wording.
- Ownership and sponsorship are covered too: `CODEOWNERS` routes review requests, and the funding files point sponsors at every place you accept support.

### Feature and roadmap docs that include your parents

- You write one short file per feature, and the overview document assembles itself — including the readme section that quotes it.
- What your upstream parent projects publish appears under their own name, refreshed every time you regenerate.
- Checking for changes never needs the network: it re-reads the committed document and shows what drifted.

### Every forge mirror tells the same story

- The description, homepage, and topics on GitHub, GitLab, and Forgejo are pushed from the projectfile — edit once, and all three mirrors agree.
- Repository settings live next to the code they describe, under version control, instead of in three separate web forms.
- A mirror you do not have simply receives nothing: no errors and no empty placeholders for it.

### Licensed and citable

- `LICENSE` is written with your copyright holder and year in place, so the file you publish is the finished text, not a template.
- The full text of every licence term you declare ships beside it under `LICENSES/`, one file per term.
- `CITATION.cff` is kept in sync, so researchers cite the project correctly with no extra effort on your side.

### One place for your package metadata

- Your JavaScript or TypeScript `package.json` follows the projectfile, and edits there flow back — the version is never newer in one place than in the other.
- Your PHP `composer.json` stays in the same sync, so Packagist sees what the projectfile says.
- Your Python `pyproject.toml` stays in sync too, while the sections you hand-edit remain exactly as you left them.
- Your Crystal `shard.yml` is covered as well: a shard release starts from the projectfile, not from a second copy of the same facts.

### A readme that keeps up with the project

- Installation and usage instructions describe what the project actually ships — a container image, a package, or a binary — with one recipe per way to get it.
- Container projects get one pull line per registry they publish to, so no destination is missing and no stale one lingers.
- Badges, related projects, and links to the community files assemble themselves from the same declaration.
- The feature list appears inside the readme too, in the reader’s language whenever a translation exists.

### Start in minutes, stay in sync afterwards

- A new project starts from an interactive scaffold that detects the ecosystem and proposes forge links and registries from the repository address and the stack.
- Scanners read the working copy — the stack, the authors, the remotes — back into the projectfile, so the document starts true and stays true.
- One command updates every generated file; naming one updates just that file.
- A check run renders without writing and shows each changed or missing file as a diff, failing the build when you ask it to.

### Every tool reads the same lists

- Ignore files for Git, containers, npm, and AI assistants all come from one include and exclude list — a path ignored in one place is ignored everywhere it matters.
- Lint and editor configs such as yamllint, browserslist, and Git attributes derive from the same declaration.
- A vulnerability you suppress once stays suppressed in Trivy, Grype, OSV-Scanner, and audit-ci alike.
- Release automation keeps one shape too, so version bumps and release notes behave the same on every release.
