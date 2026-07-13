package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/MakarMolochaev/cronit/internal/crontab"
	"github.com/MakarMolochaev/cronit/internal/manager"
)

func (m Model) openImport() (tea.Model, tea.Cmd) {
	entries, err := crontab.ForeignEntries()
	if err != nil {
		m.err = err
		return m, nil
	}
	sel := make([]bool, len(entries))
	for i := range entries {
		if entries[i].Err == "" {
			sel[i] = true
			if entries[i].Name == "" {
				entries[i].Name = manager.RandomName()
			}
		}
	}
	m.imports = entries
	m.impSel = sel
	m.impCursor, m.impOffset, m.impErr = 0, 0, ""
	m.mode = modeImport
	return m, nil
}

func (m Model) impVisible() int {
	v := m.runsVisible() - 2
	if v < 3 {
		v = 3
	}
	return v
}

func (m *Model) ensureImpVisible() {
	vis := m.impVisible()
	if m.impCursor < m.impOffset {
		m.impOffset = m.impCursor
	}
	if m.impCursor >= m.impOffset+vis {
		m.impOffset = m.impCursor - vis + 1
	}
	if m.impOffset < 0 {
		m.impOffset = 0
	}
}

func (m Model) handleImportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape", "q":
		m.mode = modeNormal
	case "up", "k":
		if m.impCursor > 0 {
			m.impCursor--
			m.ensureImpVisible()
		}
	case "down", "j":
		if m.impCursor < len(m.imports)-1 {
			m.impCursor++
			m.ensureImpVisible()
		}
	case " ", "space":
		if m.impCursor >= 0 && m.impCursor < len(m.imports) && m.imports[m.impCursor].Err == "" {
			m.impSel[m.impCursor] = !m.impSel[m.impCursor]
		}
	case "a":
		all := true
		for i, e := range m.imports {
			if e.Err == "" && !m.impSel[i] {
				all = false
				break
			}
		}
		for i, e := range m.imports {
			if e.Err == "" {
				m.impSel[i] = !all
			}
		}
	case "enter":
		var sel []crontab.ForeignEntry
		for i, e := range m.imports {
			if m.impSel[i] {
				sel = append(sel, e)
			}
		}
		if len(sel) == 0 {
			m.impErr = "nothing selected"
			return m, nil
		}
		res, err := manager.Import(m.db, sel)
		if err != nil {
			m.impErr = err.Error()
			return m, nil
		}
		m.mode = modeNormal
		m.notice = fmt.Sprintf("imported %d job(s) · crontab backup: %s", res.Imported, res.BackupPath)
		return m, m.reloadAll()
	}
	return m, nil
}

func (m Model) importTitle() string {
	sel := 0
	for _, s := range m.impSel {
		if s {
			sel++
		}
	}
	base := fmt.Sprintf("import from crontab · %d selected", sel)
	vis := m.impVisible()
	if len(m.imports) > vis {
		return fmt.Sprintf("%s [%d-%d/%d]", base, m.impOffset+1, min(m.impOffset+vis, len(m.imports)), len(m.imports))
	}
	return base
}

func (m Model) renderImportList(rowW int) string {
	if len(m.imports) == 0 {
		return faintStyle.Render("no unmanaged entries in your crontab")
	}
	lines := []string{
		descStyle.Render(trunc("selected entries become cronit-managed; a crontab backup is saved first", rowW)),
		"",
	}
	vis := m.impVisible()
	end := min(m.impOffset+vis, len(m.imports))
	schedW, nameW := 14, 18
	cmdW := rowW - schedW - nameW - 27
	if cmdW < 8 {
		cmdW = 8
	}
	for i := m.impOffset; i < end; i++ {
		e := m.imports[i]
		box := "[ ]"
		if e.Err != "" {
			box = "[!]"
		} else if m.impSel[i] {
			box = "[x]"
		}
		sched := fmt.Sprintf("%-*s", schedW, trunc(e.Schedule, schedW))
		cmd := fmt.Sprintf("%-*s", cmdW, trunc(e.Command, cmdW))
		tail := "→ " + trunc(e.Name, nameW+2)
		if e.Err != "" {
			tail = "✗ " + trunc(e.Err, nameW+2)
		}
		switch {
		case i == m.impCursor:
			lines = append(lines, selRow(fmt.Sprintf("%s %s %s  %s", box, sched, cmd, tail), rowW))
		case e.Err != "":
			lines = append(lines, "  "+faintStyle.Render(box+" "+sched+" "+cmd+"  ")+errorStyle.Render(tail))
		case m.impSel[i]:
			lines = append(lines, "  "+okStyle.Render(box)+" "+schedStyle.Render(sched)+" "+cmd+"  "+descStyle.Render(tail))
		default:
			lines = append(lines, "  "+faintStyle.Render(box)+" "+schedStyle.Render(sched)+" "+cmd+"  "+descStyle.Render(tail))
		}
	}
	if m.impErr != "" {
		lines = append(lines, errorStyle.Render(trunc("error: "+m.impErr, rowW)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderImportScreen() string {
	panelW, rowW := m.dims()
	sections := []string{
		m.header(panelW + 2),
		labeledPanel(m.importTitle(), m.renderImportList(rowW), panelW, true),
		m.footer(),
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
