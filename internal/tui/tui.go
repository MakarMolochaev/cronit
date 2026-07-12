package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/MakarMolochaev/cronit/internal/crontab"
	"github.com/MakarMolochaev/cronit/internal/manager"
	"github.com/MakarMolochaev/cronit/internal/schedule"
	"github.com/MakarMolochaev/cronit/internal/storage"
)

const refreshInterval = 2 * time.Second

const (
	modeNormal = iota
	modeForm
	modeConfirmDelete
)

var (
	titleBarStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")).Padding(0, 1)
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	panelTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	selectedRowStyle = lipgloss.NewStyle().Bold(true).
				Foreground(lipgloss.Color("231")).Background(lipgloss.Color("62"))
	schedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	descStyle   = lipgloss.NewStyle().Faint(true)
	warnStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	faintStyle  = lipgloss.NewStyle().Faint(true)
	focusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("231"))
)

type Model struct {
	db          *storage.Database
	jobs        []storage.Job
	runs        []storage.Run
	cursor      int
	offset      int
	err         error
	cronRunning bool
	cronKnown   bool
	width       int
	height      int

	mode    int
	editID  string
	inSched string
	inCmd   string
	inFocus int
	addErr  string
}

type tickMsg time.Time

type loadedMsg struct {
	jobs        []storage.Job
	runs        []storage.Run
	cronRunning bool
	cronKnown   bool
}

type runsMsg []storage.Run

type errMsg struct{ err error }

func Run(db *storage.Database) error {
	_, err := tea.NewProgram(Model{db: db}).Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.reloadAll(), tick())
}

func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) selectedID() string {
	if m.cursor >= 0 && m.cursor < len(m.jobs) {
		return m.jobs[m.cursor].ID
	}
	return ""
}

func (m Model) reloadAll() tea.Cmd {
	db, id := m.db, m.selectedID()
	return func() tea.Msg {
		jobs, err := db.Jobs()
		if err != nil {
			return errMsg{err}
		}
		var runs []storage.Run
		if id != "" {
			runs, _ = db.RecentRuns(id, 5)
		}
		running, known := crontab.DaemonRunning()
		return loadedMsg{jobs: jobs, runs: runs, cronRunning: running, cronKnown: known}
	}
}

func (m Model) reloadRuns() tea.Cmd {
	db, id := m.db, m.selectedID()
	return func() tea.Msg {
		var runs []storage.Run
		if id != "" {
			runs, _ = db.RecentRuns(id, 5)
		}
		return runsMsg(runs)
	}
}

func (m *Model) clampCursor() {
	if m.cursor >= len(m.jobs) {
		m.cursor = len(m.jobs) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureVisible()
}

func (m *Model) ensureVisible() {
	vis := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
	maxOff := len(m.jobs) - vis
	if maxOff < 0 {
		maxOff = 0
	}
	if m.offset > maxOff {
		m.offset = maxOff
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Model) sideBySide() bool {
	return m.width >= 100 && m.mode == modeNormal
}

func (m Model) visibleRows() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	if m.sideBySide() {
		vis := h - 5
		if m.cronKnown && !m.cronRunning {
			vis--
		}
		if m.err != nil {
			vis--
		}
		if vis < 3 {
			vis = 3
		}
		return vis
	}
	bottom := 11
	switch m.mode {
	case modeForm:
		bottom = 5
	case modeConfirmDelete:
		bottom = 2
	}
	vis := h - (8 + bottom)
	if vis < 3 {
		vis = 3
	}
	return vis
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureVisible()
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.reloadAll(), tick())
	case loadedMsg:
		m.jobs = msg.jobs
		m.runs = msg.runs
		m.cronRunning = msg.cronRunning
		m.cronKnown = msg.cronKnown
		m.err = nil
		m.clampCursor()
		return m, nil
	case runsMsg:
		m.runs = msg
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeForm:
		return m.handleFormKey(msg)
	case modeConfirmDelete:
		switch msg.String() {
		case "y":
			if id := m.selectedID(); id != "" {
				if err := manager.Remove(m.db, id); err != nil {
					m.err = err
				}
			}
			m.mode = modeNormal
			return m, m.reloadAll()
		case "n", "esc", "escape":
			m.mode = modeNormal
		}
		return m, nil
	default:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
				return m, m.reloadRuns()
			}
		case "down", "j":
			if m.cursor < len(m.jobs)-1 {
				m.cursor++
				m.ensureVisible()
				return m, m.reloadRuns()
			}
		case "a":
			m.mode = modeForm
			m.editID, m.inSched, m.inCmd, m.inFocus, m.addErr = "", "", "", 0, ""
		case "e":
			if len(m.jobs) > 0 {
				j := m.jobs[m.cursor]
				m.mode = modeForm
				m.editID, m.inSched, m.inCmd, m.inFocus, m.addErr = j.ID, j.Schedule, j.Command, 0, ""
			}
		case "d":
			if len(m.jobs) > 0 {
				m.mode = modeConfirmDelete
			}
		}
		return m, nil
	}
}

func (m Model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape":
		m.mode = modeNormal
		m.editID = ""
		return m, nil
	case "enter":
		var err error
		if m.editID == "" {
			_, err = manager.Add(m.db, strings.TrimSpace(m.inSched), m.inCmd)
		} else {
			err = manager.Update(m.db, m.editID, strings.TrimSpace(m.inSched), m.inCmd)
		}
		if err != nil {
			m.addErr = err.Error()
			return m, nil
		}
		m.mode = modeNormal
		m.editID = ""
		return m, m.reloadAll()
	case "up":
		m.inFocus = 0
		return m, nil
	case "down":
		m.inFocus = 1
		return m, nil
	case "tab":
		m.inFocus = 1 - m.inFocus
		return m, nil
	case "backspace":
		if m.inFocus == 0 {
			m.inCmd = trimLastRune(m.inCmd)
		} else {
			m.inSched = trimLastRune(m.inSched)
		}
		return m, nil
	}
	if t := msg.Key().Text; t != "" {
		if m.inFocus == 0 {
			m.inCmd += t
		} else {
			m.inSched += t
		}
	}
	return m, nil
}

func trimLastRune(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(r[:len(r)-1])
}

func trunc(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 3 {
		return string(r[:max(w, 0)])
	}
	return string(r[:w-3]) + "..."
}

func (m Model) dims() (panelW, rowW int) {
	w := m.width
	if w <= 0 {
		w = 84
	}
	panelW = w - 4
	if panelW < 40 {
		panelW = 40
	}
	rowW = panelW - 2
	return
}

func labeledPanel(title, body string, w int) string {
	return panelTitleStyle.Render(title) + "\n" + panelStyle.Width(w).Render(body)
}

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.BackgroundColor = lipgloss.Color("235")
	v.ForegroundColor = lipgloss.Color("252")
	return v
}

func (m Model) splitWidths() (leftCW, rightCW int) {
	w := m.width
	leftTotal := w * 3 / 5
	rightTotal := w - leftTotal - 1
	leftCW = leftTotal - 2
	rightCW = rightTotal - 2
	if leftCW < 30 {
		leftCW = 30
	}
	if rightCW < 24 {
		rightCW = 24
	}
	return
}

func (m Model) jobsTitle(vis int) string {
	if len(m.jobs) > vis {
		return fmt.Sprintf("jobs [%d-%d/%d]", m.offset+1, min(m.offset+vis, len(m.jobs)), len(m.jobs))
	}
	return "jobs"
}

func (m Model) topSections(width int) []string {
	sections := []string{titleBarStyle.Render("cronit")}
	if m.cronKnown && !m.cronRunning {
		sections = append(sections, warnStyle.Render(trunc("⚠ cron daemon not running — jobs won't fire (sudo systemctl enable --now cronie)", width)))
	}
	if m.err != nil {
		sections = append(sections, errorStyle.Render(trunc("error: "+m.err.Error(), width)))
	}
	return sections
}

func (m Model) bottomPanel(panelW, rowW int) string {
	switch m.mode {
	case modeForm:
		title := "add job"
		if m.editID != "" {
			title = "edit job"
		}
		return labeledPanel(title, m.renderForm(rowW), panelW)
	case modeConfirmDelete:
		if len(m.jobs) > 0 {
			body := warnStyle.Render(trunc(fmt.Sprintf("delete %q ?", m.jobs[m.cursor].Command), rowW))
			return labeledPanel("confirm", body, panelW)
		}
		return ""
	default:
		return labeledPanel("details", m.renderDetails(rowW), panelW)
	}
}

func (m Model) render() string {
	vis := m.visibleRows()

	if m.sideBySide() {
		leftCW, rightCW := m.splitWidths()
		left := labeledPanel(m.jobsTitle(vis), m.renderJobList(leftCW-2, vis), leftCW)
		right := labeledPanel("details", m.renderDetails(rightCW-2), rightCW)
		body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
		out := append(m.topSections(m.width), body, m.footer())
		return lipgloss.JoinVertical(lipgloss.Left, out...)
	}

	panelW, rowW := m.dims()
	sections := m.topSections(panelW + 2)
	sections = append(sections, labeledPanel(m.jobsTitle(vis), m.renderJobList(rowW, vis), panelW))
	if bp := m.bottomPanel(panelW, rowW); bp != "" {
		sections = append(sections, bp)
	}
	sections = append(sections, m.footer())
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderJobList(rowW, vis int) string {
	if len(m.jobs) == 0 {
		return faintStyle.Render("no jobs yet — press 'a' to add one")
	}
	end := min(m.offset+vis, len(m.jobs))
	var lines []string
	for i := m.offset; i < end; i++ {
		job := m.jobs[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "▸ "
		}
		schedCol := fmt.Sprintf("%-14s", trunc(job.Schedule, 14))
		avail := rowW - 2 - 14 - 4
		if avail < 12 {
			avail = 12
		}
		descW := avail / 2
		descCol := trunc(describe(job.Schedule), descW)
		cmdCol := trunc(job.Command, avail-descW)

		if i == m.cursor {
			plain := prefix + schedCol + "  " + descCol + "  " + cmdCol
			if pad := rowW - lipgloss.Width(plain); pad > 0 {
				plain += strings.Repeat(" ", pad)
			}
			lines = append(lines, selectedRowStyle.Render(plain))
		} else {
			line := prefix + schedStyle.Render(schedCol) + "  " + descStyle.Render(descCol) + "  " + cmdCol
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderForm(rowW int) string {
	var b strings.Builder
	cmdVal, schVal := m.inCmd, m.inSched
	cmdLabel, schLabel := "  command:  ", "  schedule: "
	if m.inFocus == 0 {
		cmdLabel = focusStyle.Render("▸ command:  ")
		cmdVal += cursorStyle.Render(" ")
	} else {
		schLabel = focusStyle.Render("▸ schedule: ")
		schVal += cursorStyle.Render(" ")
	}
	b.WriteString(cmdLabel)
	b.WriteString(cmdVal)
	b.WriteString("\n")
	b.WriteString(schLabel)
	b.WriteString(schVal)
	if strings.TrimSpace(m.inSched) != "" {
		b.WriteString("\n")
		b.WriteString(descStyle.Render(trunc("  → "+describe(m.inSched), rowW)))
	}
	if m.addErr != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(trunc("error: "+m.addErr, rowW)))
	}
	return b.String()
}

func (m Model) renderDetails(rowW int) string {
	if len(m.jobs) == 0 {
		return faintStyle.Render("—")
	}
	job := m.jobs[m.cursor]
	var b strings.Builder
	b.WriteString(schedStyle.Render(job.Schedule))
	b.WriteString("  ")
	b.WriteString(descStyle.Render(describe(job.Schedule)))
	b.WriteString("\n\n")

	b.WriteString(headerStyle.Render("next runs"))
	b.WriteString("\n")
	if s, err := schedule.Parse(job.Schedule); err != nil {
		b.WriteString(errorStyle.Render("  (?) " + err.Error()))
		b.WriteString("\n")
	} else if s.Reboot {
		b.WriteString("  at system startup\n")
	} else {
		t := time.Now()
		for i := 0; i < 3; i++ {
			t = s.Next(t)
			if t.IsZero() {
				b.WriteString("  no runs within 5 years\n")
				break
			}
			b.WriteString("  ")
			b.WriteString(t.Format("Mon 2006-01-02 15:04"))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(headerStyle.Render("recent runs"))
	b.WriteString("\n")
	if len(m.runs) == 0 {
		b.WriteString(faintStyle.Render("  (never run)"))
		b.WriteString("\n")
	}
	for _, r := range m.runs {
		status := okStyle.Render("✓")
		if r.ExitCode != 0 {
			status = failStyle.Render("✗")
		}
		fmt.Fprintf(&b, "  %s %s  exit=%d  %dms\n",
			status, r.StartedAt.Format("01-02 15:04"), r.ExitCode, r.DurationMs)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) footer() string {
	full, compact := m.footerHints()
	w := m.width
	if w <= 0 {
		w = 80
	}
	hint := full
	if lipgloss.Width(full) > w {
		hint = compact
	}
	return faintStyle.Render(trunc(hint, w))
}

func (m Model) footerHints() (full, compact string) {
	switch m.mode {
	case modeForm:
		return "↑/↓/tab: switch field · enter: save · esc: cancel", "tab: field · enter: save · esc: cancel"
	case modeConfirmDelete:
		return "y: delete · n: cancel", "y: delete · n: cancel"
	default:
		return "↑/↓: select · a: add · e: edit · d: delete · q: quit", "↑/↓ · a add · e edit · d del · q quit"
	}
}

func describe(expr string) string {
	s, err := schedule.Parse(expr)
	if err != nil {
		return "(?) " + err.Error()
	}
	return s.Describe()
}
