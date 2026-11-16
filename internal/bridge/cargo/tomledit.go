// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cargo

import (
	"fmt"
	"strings"
)

// Table headers this bridge edits.
const (
	tablePackage          = "package"
	tableWorkspace        = "workspace"
	tableWorkspacePackage = "workspace.package"
)

// headerName returns the table a "[name]" header line opens, "" for any other line.
func headerName(line string) string {
	t := strings.TrimSpace(stripComment(line))
	if !strings.HasPrefix(t, "[") || strings.HasPrefix(t, "[[") || !strings.HasSuffix(t, "]") {
		return ""
	}
	return strings.TrimSpace(t[1 : len(t)-1])
}

// isHeader reports whether line opens a table or an array-of-tables entry.
func isHeader(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "[")
}

// stripComment drops a trailing "# …" comment outside quotes from a header line.
func stripComment(line string) string {
	inBasic, inLiteral := false, false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case inBasic && c == '\\':
			i++
		case c == '"' && !inLiteral:
			inBasic = !inBasic
		case c == '\'' && !inBasic:
			inLiteral = !inLiteral
		case c == '#' && !inBasic && !inLiteral:
			return line[:i]
		}
	}
	return line
}

// valueEnd returns the index of the last line of the key/value pair starting at line i.
func valueEnd(lines []string, i int) int {
	eq := strings.Index(lines[i], "=")
	if eq < 0 {
		return i
	}
	s := scanner{}
	for j := i; j < len(lines); j++ {
		text := lines[j]
		if j == i {
			text = text[eq+1:]
		}
		s.feed(text)
		if s.closed() {
			return j
		}
	}
	return len(lines) - 1
}

// scanner tracks string and bracket nesting across the lines of one TOML value.
type scanner struct {
	depth int
	// quote is the open string delimiter: `"`, `'`, `"""` or `'''`, "" outside a string.
	quote string
}

// feed consumes one line of value text.
func (s *scanner) feed(text string) {
	for i := 0; i < len(text); i++ {
		if s.quote != "" {
			i = s.inString(text, i)
			continue
		}
		switch c := text[i]; c {
		case '#':
			return
		case '[', '{':
			s.depth++
		case ']', '}':
			s.depth--
		case '"', '\'':
			s.quote = string(c)
			if strings.HasPrefix(text[i:], strings.Repeat(string(c), 3)) {
				s.quote = strings.Repeat(string(c), 3)
				i += 2
			}
		}
	}
	if len(s.quote) == 1 {
		s.quote = ""
	}
}

// inString advances over one character inside an open string and returns the new index.
func (s *scanner) inString(text string, i int) int {
	if s.quote[0] == '"' && text[i] == '\\' {
		return i + 1
	}
	if strings.HasPrefix(text[i:], s.quote) {
		end := i + len(s.quote) - 1
		s.quote = ""
		return end
	}
	return i
}

// closed reports whether the value ended on the line just fed.
func (s *scanner) closed() bool {
	return s.depth <= 0 && s.quote == ""
}

// zone holds the line span of one table: its header line and the first line past it.
type zone struct{ header, end int }

// findZone locates the direct key/value span of table name, header -1 when absent.
func findZone(lines []string, name string) zone {
	for i := range lines {
		if headerName(lines[i]) != name {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if isHeader(lines[j]) {
				end = j
				break
			}
			if keyOf(lines[j]) != "" {
				j = valueEnd(lines, j)
			}
		}
		return zone{header: i, end: end}
	}
	return zone{header: -1, end: -1}
}

// keyOf returns the bare key a "key = value" line assigns, "" for any other line.
func keyOf(line string) string {
	t := strings.TrimSpace(line)
	if t == "" || t[0] == '#' || t[0] == '[' {
		return ""
	}
	eq := strings.Index(t, "=")
	if eq <= 0 {
		return ""
	}
	return strings.TrimSpace(t[:eq])
}

// lastContent returns the last non-blank, non-comment line of z, or its header.
func lastContent(lines []string, z zone) int {
	for i := z.end - 1; i > z.header; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" && t[0] != '#' {
			return valueEnd(lines, i)
		}
	}
	return z.header
}

// setKey writes key = value into table, removing the key when value is empty.
func setKey(lines []string, table, key, value string) []string {
	z := findZone(lines, table)
	if z.header < 0 {
		if value == "" {
			return lines
		}
		lines, z = addTable(lines, table)
	}
	assignment := key + " = " + value
	for i := z.header + 1; i < z.end; i++ {
		if keyOf(lines[i]) != key {
			continue
		}
		end := valueEnd(lines, i)
		rest := append([]string{}, lines[end+1:]...)
		lines = lines[:i]
		if value != "" {
			lines = append(lines, assignment)
		}
		return append(lines, rest...)
	}
	if value == "" {
		return lines
	}
	at := lastContent(lines, z) + 1
	return insert(lines, at, assignment)
}

// addTable creates an empty table header where Cargo users expect it and returns its zone.
func addTable(lines []string, table string) ([]string, zone) {
	at := firstData(lines)
	if table == tableWorkspacePackage {
		if ws := findZone(lines, tableWorkspace); ws.header >= 0 {
			at = lastContent(lines, ws) + 1
		}
	}
	block := []string{"[" + table + "]"}
	if at > 0 && strings.TrimSpace(lines[at-1]) != "" {
		block = append([]string{""}, block...)
	}
	if at < len(lines) && strings.TrimSpace(lines[at]) != "" {
		block = append(block, "")
	}
	lines = insert(lines, at, block...)
	header := at
	if block[0] == "" {
		header++
	}
	return lines, zone{header: header, end: header + 1}
}

// firstData returns the index of the first line past the leading comment block.
func firstData(lines []string) int {
	for i, l := range lines {
		if t := strings.TrimSpace(l); t != "" && t[0] != '#' {
			return i
		}
	}
	return len(lines)
}

// insert splices extra into lines at index at.
func insert(lines []string, at int, extra ...string) []string {
	out := make([]string, 0, len(lines)+len(extra))
	out = append(out, lines[:at]...)
	out = append(out, extra...)
	return append(out, lines[at:]...)
}

// quote renders s as a TOML basic string.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// quoteList renders items as a one-line TOML array of basic strings.
func quoteList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = quote(s)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
