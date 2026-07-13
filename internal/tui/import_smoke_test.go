package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/MakarMolochaev/cronit/internal/crontab"
)

func TestRenderImportScreenWidths(t *testing.T) {
	imports := []crontab.ForeignEntry{
		{Schedule: "@daily", Command: "/usr/local/bin/backup.sh --home --verbose --exclude-cache", Name: "backup home dir", LineNo: 3, CommentLine: 2},
		{Schedule: "*/5 * * * *", Command: "curl -fsS https://example.com/ping", Name: "sunny-quokka", LineNo: 4, CommentLine: -1},
		{Schedule: "0 9 * * 1-5", Command: `echo "hello %USER%"`, LineNo: 5, CommentLine: -1, Err: "'%' has special meaning in crontab and is not supported"},
	}
	for _, w := range []int{40, 60, 80, 100, 140} {
		for cursor := 0; cursor < len(imports); cursor++ {
			m := Model{
				mode:      modeImport,
				width:     w,
				height:    24,
				imports:   imports,
				impSel:    []bool{true, false, false},
				impCursor: cursor,
				foreignN:  3,
			}
			out := m.render()
			for _, ln := range strings.Split(out, "\n") {
				if lw := lipgloss.Width(ln); lw > max(w, 44) {
					t.Fatalf("w=%d cursor=%d: line width %d exceeds %d: %q", w, cursor, lw, w, ln)
				}
			}
		}
	}

	m := Model{mode: modeImport, width: 80, height: 24}
	if !strings.Contains(m.render(), "no unmanaged entries") {
		t.Fatalf("empty import screen missing placeholder")
	}
}
