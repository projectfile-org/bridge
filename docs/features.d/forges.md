<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# Every forge mirror tells the same story

- The description, homepage, and topics on GitHub, GitLab, and Forgejo are pushed from the projectfile — edit once, and all three mirrors agree.
- Branch and tag protection follows the release branches the projectfile already declares, and the origin opens on the trunk: the stable branch takes only signed history from its owner, and no mirror lets anyone rewrite or delete a release branch.
- Repository settings live next to the code they describe, under version control, instead of in three separate web forms.
- A mirror you do not have simply receives nothing: no errors and no empty placeholders for it.
