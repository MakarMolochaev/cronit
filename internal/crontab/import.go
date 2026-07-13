package crontab

import (
	"strings"

	"github.com/MakarMolochaev/cronit/internal/schedule"
)

type ForeignEntry struct {
	Schedule    string
	Command     string
	Name        string
	Line        string
	LineNo      int
	CommentLine int
	Err         string
}

func ForeignEntries() ([]ForeignEntry, error) {
	content, err := Read()
	if err != nil {
		return nil, err
	}
	return parseForeign(content), nil
}

func parseForeign(content string) []ForeignEntry {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	var out []ForeignEntry
	comment := ""
	commentLine := -1
	skipNext := false
	for i, raw := range strings.Split(content, "\n") {
		t := strings.TrimSpace(raw)
		if skipNext {
			skipNext = false
			comment, commentLine = "", -1
			continue
		}
		switch {
		case t == "":
			comment, commentLine = "", -1
		case strings.HasPrefix(t, marker):
			skipNext = true
			comment, commentLine = "", -1
		case strings.HasPrefix(t, "#"):
			comment = strings.TrimSpace(strings.TrimPrefix(t, "#"))
			commentLine = i
		case isEnvAssign(t):
			comment, commentLine = "", -1
		default:
			e := parseEntry(t)
			e.Line, e.LineNo = raw, i
			e.Name, e.CommentLine = comment, commentLine
			out = append(out, e)
			comment, commentLine = "", -1
		}
	}
	return out
}

func isEnvAssign(s string) bool {
	i := strings.IndexByte(s, '=')
	if i <= 0 {
		return false
	}
	name := strings.TrimSpace(s[:i])
	if name == "" {
		return false
	}
	for j, r := range name {
		switch {
		case r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			if j == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func parseEntry(t string) ForeignEntry {
	var e ForeignEntry
	e.CommentLine = -1
	n := 5
	if strings.HasPrefix(t, "@") {
		n = 1
	}
	e.Schedule, e.Command = cutFields(t, n)
	if e.Command == "" {
		e.Err = "missing command"
		return e
	}
	if _, err := schedule.Parse(e.Schedule); err != nil {
		e.Err = err.Error()
		return e
	}
	cmd, ok := unescapePercent(e.Command)
	if !ok {
		e.Err = "unescaped '%' has special meaning in crontab and is not supported"
		return e
	}
	e.Command = cmd
	return e
}

func unescapePercent(s string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '%' {
			b.WriteByte('%')
			i++
			continue
		}
		if s[i] == '%' {
			return "", false
		}
		b.WriteByte(s[i])
	}
	return b.String(), true
}

func cutFields(s string, n int) (head, rest string) {
	i := 0
	for f := 0; f < n; f++ {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		start := i
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		if i == start {
			return strings.Join(strings.Fields(s), " "), ""
		}
	}
	return strings.Join(strings.Fields(s[:i]), " "), strings.TrimSpace(s[i:])
}
