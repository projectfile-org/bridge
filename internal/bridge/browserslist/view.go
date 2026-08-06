// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package browserslist

// queries is the line-oriented view fed to the byte assembler. .browserslistrc
// is one query per line (e.g. `last 2 versions`, `> 0.5%`, `not dead`) — there
// is no nesting, so a plain string slice is the whole model and no text/template
// is warranted (unlike FUNDING.yml / SECURITY.md).
type queries = []string
