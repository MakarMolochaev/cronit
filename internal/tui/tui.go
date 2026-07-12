package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/MakarMolochaev/cronit/internal/banner"
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
	modePicker
	modeBuilder
	modeRuns
	modeOutput
)

const (
	ctrlFreq = iota
	ctrlInterval
	ctrlHour
	ctrlMinute
	ctrlDom
	ctrlDows
)

var bldFreqNames = []string{"every N minutes", "every N hours", "daily", "weekly", "monthly"}

var dowLetters = [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}

const pickerMaxRows = 8

type pickItem struct {
	name    string
	sched   string
	custom  bool
	current bool
}

var builtinPresets = []pickItem{
	{name: "every minute", sched: "* * * * *"},
	{name: "every 5 minutes", sched: "*/5 * * * *"},
	{name: "every 15 minutes", sched: "*/15 * * * *"},
	{name: "every 30 minutes", sched: "*/30 * * * *"},
	{name: "hourly", sched: "0 * * * *"},
	{name: "daily at 00:00", sched: "0 0 * * *"},
	{name: "daily at 09:00", sched: "0 9 * * *"},
	{name: "weekdays at 09:00", sched: "0 9 * * 1-5"},
	{name: "weekly (Mon 09:00)", sched: "0 9 * * 1"},
	{name: "monthly (1st 00:00)", sched: "0 0 1 * *"},
}

type Model struct {
	db          *storage.Database
	version     string
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
	inName  string
	inSched string
	inCmd   string
	inFocus int
	addErr  string

	runList   []storage.Run
	runCursor int
	runOffset int
	outScroll int

	picker     []pickItem
	pickCursor int
	pickOffset int
	pickSaving bool
	pickName   string
	pickErr    string

	bldFreq      int
	bldFocus     int
	bldInterval  int
	bldHour      int
	bldMin       int
	bldDom       int
	bldDows      [7]bool
	bldDowCursor int
	bldReady     bool
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

func Run(db *storage.Database, version string) error {
	_, err := tea.NewProgram(Model{db: db, version: version}).Run()
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
			runs, _ = db.RecentRuns(id, 3)
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
			runs, _ = db.RecentRuns(id, 3)
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
	extraFooter := lipgloss.Height(m.footer()) - 1
	if m.sideBySide() {
		vis := h - 5 - extraFooter
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
		bottom = 8
	case modePicker:
		bottom = m.pickerRows() + 4
	case modeBuilder:
		bottom = 10
	case modeConfirmDelete:
		bottom = 2
	}
	vis := h - (8 + bottom) - extraFooter
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
	case modePicker:
		return m.handlePickerKey(msg)
	case modeBuilder:
		return m.handleBuilderKey(msg)
	case modeRuns:
		return m.handleRunsKey(msg)
	case modeOutput:
		return m.handleOutputKey(msg)
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
			m.editID, m.inName, m.inCmd, m.inSched, m.inFocus, m.addErr = "", manager.RandomName(), "", "", 0, ""
		case "e":
			if len(m.jobs) > 0 {
				j := m.jobs[m.cursor]
				m.mode = modeForm
				m.editID, m.inName, m.inCmd, m.inSched, m.inFocus, m.addErr = j.ID, j.Name, j.Command, j.Schedule, 0, ""
			}
		case " ", "space":
			if len(m.jobs) > 0 {
				j := m.jobs[m.cursor]
				if err := manager.SetEnabled(m.db, j.ID, !j.Enabled); err != nil {
					m.err = err
				}
				return m, m.reloadAll()
			}
		case "enter":
			if len(m.jobs) > 0 {
				return m.openRuns()
			}
		case "d":
			if len(m.jobs) > 0 {
				m.mode = modeConfirmDelete
			}
		}
		return m, nil
	}
}

func (m Model) openRuns() (tea.Model, tea.Cmd) {
	id := m.selectedID()
	runs, err := m.db.RecentRuns(id, 50)
	if err != nil {
		m.err = err
		return m, nil
	}
	m.runList = runs
	m.runCursor, m.runOffset, m.outScroll = 0, 0, 0
	m.mode = modeRuns
	return m, nil
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
			_, err = manager.Add(m.db, strings.TrimSpace(m.inName), strings.TrimSpace(m.inSched), m.inCmd)
		} else {
			err = manager.Update(m.db, m.editID, strings.TrimSpace(m.inName), strings.TrimSpace(m.inSched), m.inCmd)
		}
		if err != nil {
			m.addErr = err.Error()
			return m, nil
		}
		m.mode = modeNormal
		m.editID = ""
		return m, m.reloadAll()
	case "up", "shift+tab":
		m.inFocus = (m.inFocus + 2) % 3
		return m, nil
	case "down", "tab":
		m.inFocus = (m.inFocus + 1) % 3
		return m, nil
	case "ctrl+t":
		return m.openPicker(), nil
	case "ctrl+b":
		return m.openBuilder(), nil
	case "backspace":
		m.setField(trimLastRune(m.field()))
		return m, nil
	}
	if t := msg.Key().Text; t != "" {
		m.setField(m.field() + t)
	}
	return m, nil
}

func (m Model) field() string {
	switch m.inFocus {
	case 0:
		return m.inName
	case 1:
		return m.inCmd
	default:
		return m.inSched
	}
}

func (m *Model) setField(v string) {
	switch m.inFocus {
	case 0:
		m.inName = v
	case 1:
		m.inCmd = v
	default:
		m.inSched = v
	}
}

func (m Model) openPicker() Model {
	m.mode = modePicker
	m.pickCursor, m.pickOffset = 0, 0
	m.pickSaving, m.pickName, m.pickErr = false, "", ""
	return m.reloadPicker()
}

func (m Model) reloadPicker() Model {
	var items []pickItem
	if cur := strings.TrimSpace(m.inSched); cur != "" {
		items = append(items, pickItem{name: "current", sched: cur, current: true})
	}
	if tpls, err := m.db.Templates(); err == nil {
		for _, t := range tpls {
			items = append(items, pickItem{name: t.Name, sched: t.Schedule, custom: true})
		}
	}
	items = append(items, builtinPresets...)
	m.picker = items
	if m.pickCursor >= len(items) {
		m.pickCursor = len(items) - 1
	}
	if m.pickCursor < 0 {
		m.pickCursor = 0
	}
	m.ensurePickVisible()
	return m
}

func (m Model) pickerRows() int {
	n := len(m.picker)
	if n > pickerMaxRows {
		n = pickerMaxRows
	}
	if n < 1 {
		n = 1
	}
	return n
}

func (m *Model) ensurePickVisible() {
	vis := m.pickerRows()
	if m.pickCursor < m.pickOffset {
		m.pickOffset = m.pickCursor
	}
	if m.pickCursor >= m.pickOffset+vis {
		m.pickOffset = m.pickCursor - vis + 1
	}
	if m.pickOffset < 0 {
		m.pickOffset = 0
	}
}

func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pickSaving {
		switch msg.String() {
		case "esc", "escape":
			m.pickSaving, m.pickErr = false, ""
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.pickName)
			if name == "" {
				m.pickErr = "name required"
				return m, nil
			}
			sched := strings.TrimSpace(m.inSched)
			if _, err := schedule.Parse(sched); err != nil {
				m.pickErr = "invalid schedule: " + err.Error()
				return m, nil
			}
			if err := m.db.SaveTemplate(name, sched); err != nil {
				m.pickErr = err.Error()
				return m, nil
			}
			m.pickSaving, m.pickName, m.pickErr = false, "", ""
			return m.reloadPicker(), nil
		case "backspace":
			m.pickName = trimLastRune(m.pickName)
			return m, nil
		}
		if t := msg.Key().Text; t != "" {
			m.pickName += t
		}
		return m, nil
	}

	switch msg.String() {
	case "esc", "escape":
		m.mode = modeForm
	case "enter":
		if m.pickCursor >= 0 && m.pickCursor < len(m.picker) {
			m.inSched = m.picker[m.pickCursor].sched
		}
		m.mode = modeForm
	case "up", "k":
		if m.pickCursor > 0 {
			m.pickCursor--
			m.ensurePickVisible()
		}
	case "down", "j":
		if m.pickCursor < len(m.picker)-1 {
			m.pickCursor++
			m.ensurePickVisible()
		}
	case "s":
		if strings.TrimSpace(m.inSched) == "" {
			m.pickErr = "schedule field is empty — type one first"
			return m, nil
		}
		m.pickSaving, m.pickName, m.pickErr = true, "", ""
	case "d":
		if m.pickCursor >= 0 && m.pickCursor < len(m.picker) && m.picker[m.pickCursor].custom {
			if err := m.db.DeleteTemplate(m.picker[m.pickCursor].name); err != nil {
				m.pickErr = err.Error()
				return m, nil
			}
			return m.reloadPicker(), nil
		}
	}
	return m, nil
}

func (m Model) openBuilder() Model {
	m.mode = modeBuilder
	m.bldFocus = 0
	m.bldDowCursor = 0
	if !m.bldReady {
		m.bldFreq = 2
		m.bldInterval = 5
		m.bldHour = 9
		m.bldMin = 0
		m.bldDom = 1
		m.bldReady = true
	}
	return m
}

func (m Model) bldControls() []int {
	switch m.bldFreq {
	case 0:
		return []int{ctrlFreq, ctrlInterval}
	case 1:
		return []int{ctrlFreq, ctrlInterval, ctrlMinute}
	case 2:
		return []int{ctrlFreq, ctrlHour, ctrlMinute}
	case 3:
		return []int{ctrlFreq, ctrlHour, ctrlMinute, ctrlDows}
	case 4:
		return []int{ctrlFreq, ctrlHour, ctrlMinute, ctrlDom}
	}
	return []int{ctrlFreq}
}

func wrap(v, lo, hi int) int {
	if v < lo {
		return hi
	}
	if v > hi {
		return lo
	}
	return v
}

func (m *Model) bldAdjust(ctrl, delta int) {
	switch ctrl {
	case ctrlFreq:
		n := len(bldFreqNames)
		m.bldFreq = (m.bldFreq + delta + n) % n
		if c := m.bldControls(); m.bldFocus >= len(c) {
			m.bldFocus = len(c) - 1
		}
	case ctrlInterval:
		hi := 59
		if m.bldFreq == 1 {
			hi = 23
		}
		m.bldInterval = wrap(m.bldInterval+delta, 1, hi)
	case ctrlHour:
		m.bldHour = wrap(m.bldHour+delta, 0, 23)
	case ctrlMinute:
		m.bldMin = wrap(m.bldMin+delta, 0, 59)
	case ctrlDom:
		m.bldDom = wrap(m.bldDom+delta, 1, 31)
	case ctrlDows:
		m.bldDowCursor = wrap(m.bldDowCursor+delta, 0, 6)
	}
}

func (m Model) bldDowField() string {
	count := 0
	for _, on := range m.bldDows {
		if on {
			count++
		}
	}
	if count == 0 || count == 7 {
		return "*"
	}
	var days []string
	for i, on := range m.bldDows {
		if on {
			days = append(days, strconv.Itoa(i))
		}
	}
	return strings.Join(days, ",")
}

func (m Model) bldExpr() string {
	switch m.bldFreq {
	case 0:
		if m.bldInterval <= 1 {
			return "* * * * *"
		}
		return fmt.Sprintf("*/%d * * * *", m.bldInterval)
	case 1:
		if m.bldInterval <= 1 {
			return fmt.Sprintf("%d * * * *", m.bldMin)
		}
		return fmt.Sprintf("%d */%d * * *", m.bldMin, m.bldInterval)
	case 2:
		return fmt.Sprintf("%d %d * * *", m.bldMin, m.bldHour)
	case 3:
		return fmt.Sprintf("%d %d * * %s", m.bldMin, m.bldHour, m.bldDowField())
	case 4:
		return fmt.Sprintf("%d %d %d * *", m.bldMin, m.bldHour, m.bldDom)
	}
	return "* * * * *"
}

func (m Model) handleBuilderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	controls := m.bldControls()
	if m.bldFocus >= len(controls) {
		m.bldFocus = len(controls) - 1
	}
	if m.bldFocus < 0 {
		m.bldFocus = 0
	}
	cur := controls[m.bldFocus]

	switch msg.String() {
	case "esc", "escape":
		m.mode = modeForm
	case "enter":
		m.inSched = m.bldExpr()
		m.mode = modeForm
	case "tab", "down":
		m.bldFocus = (m.bldFocus + 1) % len(controls)
	case "shift+tab", "up":
		m.bldFocus = (m.bldFocus - 1 + len(controls)) % len(controls)
	case "left", "h":
		m.bldAdjust(cur, -1)
	case "right", "l":
		m.bldAdjust(cur, 1)
	case " ", "space":
		if cur == ctrlDows {
			m.bldDows[m.bldDowCursor] = !m.bldDows[m.bldDowCursor]
		}
	}
	return m, nil
}

func (m Model) runsVisible() int {
	v := m.height - 6
	if v < 3 {
		v = 3
	}
	return v
}

func (m *Model) ensureRunVisible() {
	vis := m.runsVisible()
	if m.runCursor < m.runOffset {
		m.runOffset = m.runCursor
	}
	if m.runCursor >= m.runOffset+vis {
		m.runOffset = m.runCursor - vis + 1
	}
	if m.runOffset < 0 {
		m.runOffset = 0
	}
}

func (m Model) currentRun() (storage.Run, bool) {
	if m.runCursor >= 0 && m.runCursor < len(m.runList) {
		return m.runList[m.runCursor], true
	}
	return storage.Run{}, false
}

func (m Model) maxOutScroll() int {
	mx := len(m.outputLines()) - m.runsVisible()
	if mx < 0 {
		mx = 0
	}
	return mx
}

func (m Model) handleRunsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "escape", "q":
		m.mode = modeNormal
	case "up", "k":
		if m.runCursor > 0 {
			m.runCursor--
			m.ensureRunVisible()
		}
	case "down", "j":
		if m.runCursor < len(m.runList)-1 {
			m.runCursor++
			m.ensureRunVisible()
		}
	case "enter", "o":
		if len(m.runList) > 0 {
			m.outScroll = 0
			m.mode = modeOutput
		}
	}
	return m, nil
}

func (m Model) handleOutputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if mx := m.maxOutScroll(); m.outScroll > mx {
		m.outScroll = mx
	}
	switch msg.String() {
	case "esc", "escape", "q":
		m.mode = modeRuns
	case "up", "k":
		if m.outScroll > 0 {
			m.outScroll--
		}
	case "down", "j":
		if m.outScroll < m.maxOutScroll() {
			m.outScroll++
		}
	case "pgup":
		m.outScroll -= 10
		if m.outScroll < 0 {
			m.outScroll = 0
		}
	case "pgdown", "pgdn":
		m.outScroll = min(m.outScroll+10, m.maxOutScroll())
	case "home":
		m.outScroll = 0
	case "end":
		m.outScroll = m.maxOutScroll()
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

func labeledPanel(title, body string, w int, focused bool) string {
	ts, ps := panelTitleStyle, panelStyle
	if focused {
		ts, ps = panelTitleFocusStyle, panelFocusStyle
	}
	return ts.Render(title) + "\n" + ps.Width(w).Render(body)
}

func selRow(inner string, rowW int) string {
	if pad := rowW - 2 - lipgloss.Width(inner); pad > 0 {
		inner += strings.Repeat(" ", pad)
	}
	return barStyle.Render("▎ ") + selectedRowStyle.Render(inner)
}

func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.BackgroundColor = appBg
	v.ForegroundColor = appFg
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

func (m Model) header(width int) string {
	hb := lipgloss.NewStyle().Background(headerBg)
	left := hb.Foreground(accent2).Bold(true).Render(" cronit")
	right := m.headerMeta(hb)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + hb.Render(strings.Repeat(" ", gap)) + right
}

func (m Model) headerMeta(hb lipgloss.Style) string {
	sep := hb.Foreground(fgMuted).Render(" · ")
	parts := []string{hb.Foreground(fgMuted).Render(fmt.Sprintf("%d jobs", len(m.jobs)))}
	if m.cronKnown {
		if m.cronRunning {
			parts = append(parts, hb.Foreground(success).Render("● cron"))
		} else {
			parts = append(parts, hb.Foreground(warnClr).Render("● cron off"))
		}
	}
	if m.version != "" {
		parts = append(parts, hb.Foreground(fgMuted).Render(m.version))
	}
	return strings.Join(parts, sep) + hb.Render(" ")
}

func (m Model) topSections(width int) []string {
	sections := []string{m.header(width)}
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
		return labeledPanel(title, m.renderForm(rowW), panelW, true)
	case modePicker:
		return labeledPanel("pick schedule", m.renderPicker(rowW), panelW, true)
	case modeBuilder:
		return labeledPanel("build schedule", m.renderBuilder(rowW), panelW, true)
	case modeConfirmDelete:
		if len(m.jobs) > 0 {
			body := warnStyle.Render(trunc(fmt.Sprintf("delete %q ?", m.jobs[m.cursor].Name), rowW))
			return labeledPanel("confirm", body, panelW, true)
		}
		return ""
	default:
		return labeledPanel("details", m.renderDetails(rowW), panelW, false)
	}
}

func (m Model) render() string {
	switch m.mode {
	case modeRuns:
		return m.renderRunsScreen()
	case modeOutput:
		return m.renderOutputScreen()
	}

	vis := m.visibleRows()

	if m.sideBySide() {
		leftCW, rightCW := m.splitWidths()
		left := labeledPanel(m.jobsTitle(vis), m.renderJobList(leftCW-2, vis), leftCW, true)
		right := labeledPanel("details", m.renderDetails(rightCW-2), rightCW, false)
		body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
		out := append(m.topSections(m.width), body, m.footer())
		return lipgloss.JoinVertical(lipgloss.Left, out...)
	}

	panelW, rowW := m.dims()
	sections := m.topSections(panelW + 2)
	sections = append(sections, labeledPanel(m.jobsTitle(vis), m.renderJobList(rowW, vis), panelW, m.mode == modeNormal))
	if bp := m.bottomPanel(panelW, rowW); bp != "" {
		sections = append(sections, bp)
	}
	sections = append(sections, m.footer())
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) runsScreenTitle() string {
	name := ""
	if m.cursor >= 0 && m.cursor < len(m.jobs) {
		name = m.jobs[m.cursor].Name
	}
	base := "runs"
	if name != "" {
		base = "runs · " + name
	}
	vis := m.runsVisible()
	if len(m.runList) > vis {
		return fmt.Sprintf("%s [%d-%d/%d]", base, m.runOffset+1, min(m.runOffset+vis, len(m.runList)), len(m.runList))
	}
	return base
}

func (m Model) renderRunsList(rowW int) string {
	if len(m.runList) == 0 {
		return faintStyle.Render("no runs recorded yet")
	}
	vis := m.runsVisible()
	end := min(m.runOffset+vis, len(m.runList))
	var lines []string
	for i := m.runOffset; i < end; i++ {
		r := m.runList[i]
		ts := r.StartedAt.Format("2006-01-02 15:04:05")
		meta := fmt.Sprintf("exit=%d  %dms", r.ExitCode, r.DurationMs)
		glyph := "✓"
		if r.ExitCode != 0 {
			glyph = "✗"
		}
		if i == m.runCursor {
			lines = append(lines, selRow(fmt.Sprintf("%s  %s  %s", glyph, ts, meta), rowW))
		} else {
			cg := okStyle.Render(glyph)
			if r.ExitCode != 0 {
				cg = failStyle.Render(glyph)
			}
			lines = append(lines, "  "+cg+"  "+ts+"  "+descStyle.Render(meta))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderRunsScreen() string {
	panelW, _ := m.dims()
	sections := []string{
		m.header(panelW + 2),
		labeledPanel(m.runsScreenTitle(), m.renderRunsList(panelW-2), panelW, true),
		m.footer(),
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		out = append(out, "  "+ln)
	}
	return out
}

func (m Model) outputLines() []string {
	r, ok := m.currentRun()
	if !ok {
		return []string{faintStyle.Render("no run selected")}
	}
	head := fmt.Sprintf("%s   exit=%d   %dms",
		r.StartedAt.Format("2006-01-02 15:04:05"), r.ExitCode, r.DurationMs)
	lines := []string{headerStyle.Render(head), "", headerStyle.Render("stdout")}
	if strings.TrimSpace(r.Stdout) == "" {
		lines = append(lines, faintStyle.Render("  (empty)"))
	} else {
		lines = append(lines, splitLines(r.Stdout)...)
	}
	lines = append(lines, "", headerStyle.Render("stderr"))
	if strings.TrimSpace(r.Stderr) == "" {
		lines = append(lines, faintStyle.Render("  (empty)"))
	} else {
		lines = append(lines, splitLines(r.Stderr)...)
	}
	return lines
}

func (m Model) renderOutputScreen() string {
	panelW, rowW := m.dims()
	all := m.outputLines()
	vis := m.runsVisible()

	scroll := m.outScroll
	if mx := m.maxOutScroll(); scroll > mx {
		scroll = mx
	}
	if scroll < 0 {
		scroll = 0
	}
	end := min(scroll+vis, len(all))

	var body []string
	for _, ln := range all[scroll:end] {
		body = append(body, trunc(ln, rowW))
	}
	title := "output"
	if len(all) > vis {
		title = fmt.Sprintf("output [%d-%d/%d]", scroll+1, end, len(all))
	}
	sections := []string{
		m.header(panelW + 2),
		labeledPanel(title, strings.Join(body, "\n"), panelW, true),
		m.footer(),
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) emptyState(rowW, vis int) string {
	hint := faintStyle.Render(trunc("press 'a' to add your first job", rowW))
	if rowW < lipgloss.Width(banner.Art) {
		return lipgloss.Place(rowW, vis, lipgloss.Center, lipgloss.Center, hint)
	}
	content := lipgloss.JoinVertical(lipgloss.Center, banner.Render(), "", hint)
	return lipgloss.Place(rowW, vis, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) renderJobList(rowW, vis int) string {
	if len(m.jobs) == 0 {
		return m.emptyState(rowW, vis)
	}
	end := min(m.offset+vis, len(m.jobs))
	nameW, schedW := 16, 14
	var lines []string
	for i := m.offset; i < end; i++ {
		job := m.jobs[i]
		glyph := "●"
		if !job.Enabled {
			glyph = "○"
		}
		name := fmt.Sprintf("%-*s", nameW, trunc(job.Name, nameW))
		sched := fmt.Sprintf("%-*s", schedW, trunc(job.Schedule, schedW))
		avail := rowW - 7 - nameW - schedW
		if avail < 6 {
			avail = 6
		}
		desc := trunc(describe(job.Schedule), avail)

		if i == m.cursor {
			lines = append(lines, selRow(fmt.Sprintf("%s %s %s  %s", glyph, name, sched, desc), rowW))
		} else {
			g, nm, sc := okStyle.Render(glyph), name, schedStyle.Render(sched)
			if !job.Enabled {
				g, nm, sc = faintStyle.Render(glyph), faintStyle.Render(name), faintStyle.Render(sched)
			}
			lines = append(lines, "  "+g+" "+nm+" "+sc+"  "+descStyle.Render(desc))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderForm(rowW int) string {
	var b strings.Builder
	fields := []struct {
		label string
		val   string
		idx   int
	}{
		{"name:     ", m.inName, 0},
		{"command:  ", m.inCmd, 1},
		{"schedule: ", m.inSched, 2},
	}
	for _, f := range fields {
		if m.inFocus == f.idx {
			b.WriteString(focusStyle.Render("▸ " + f.label))
			b.WriteString(trunc(f.val, rowW-14))
			b.WriteString(cursorStyle.Render(" "))
		} else {
			b.WriteString("  ")
			b.WriteString(f.label)
			b.WriteString(trunc(f.val, rowW-14))
		}
		b.WriteString("\n")
	}
	if strings.TrimSpace(m.inSched) != "" {
		b.WriteString(descStyle.Render(trunc("  → "+describe(m.inSched), rowW)))
		b.WriteString("\n")
	}
	b.WriteString(faintStyle.Render("  ctrl+t: pick / save · ctrl+b: build a schedule"))
	if m.addErr != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(trunc("error: "+m.addErr, rowW)))
	}
	return b.String()
}

func (m Model) renderPicker(rowW int) string {
	var lines []string
	if m.pickSaving {
		lines = append(lines, focusStyle.Render("save as: ")+m.pickName+cursorStyle.Render(" "))
		lines = append(lines, descStyle.Render(trunc("  "+strings.TrimSpace(m.inSched)+"  →  "+describe(m.inSched), rowW)))
		if m.pickErr != "" {
			lines = append(lines, errorStyle.Render(trunc("error: "+m.pickErr, rowW)))
		}
		return strings.Join(lines, "\n")
	}
	if len(m.picker) == 0 {
		return faintStyle.Render("no schedules")
	}
	vis := m.pickerRows()
	end := min(m.pickOffset+vis, len(m.picker))
	nameW := 22
	schedW := rowW - 2 - nameW - 2
	if schedW < 10 {
		schedW = 10
	}
	for i := m.pickOffset; i < end; i++ {
		it := m.picker[i]
		name := fmt.Sprintf("%-*s", nameW, trunc(it.name, nameW))
		sched := trunc(it.sched, schedW)
		switch {
		case i == m.pickCursor:
			inner := name + "  " + sched
			if it.current {
				inner += "  (s: save)"
			}
			lines = append(lines, selRow(inner, rowW))
		case it.current:
			lines = append(lines, "  "+warnStyle.Render(name)+"  "+schedStyle.Render(sched)+faintStyle.Render("  (s: save)"))
		case it.custom:
			lines = append(lines, "  "+schedStyle.Render(name)+"  "+descStyle.Render(sched))
		default:
			lines = append(lines, "  "+name+"  "+descStyle.Render(sched))
		}
	}
	if m.pickErr != "" {
		lines = append(lines, errorStyle.Render(trunc("error: "+m.pickErr, rowW)))
	}
	return strings.Join(lines, "\n")
}

func bldArrows(focused bool, s string) string {
	if focused {
		return focusStyle.Render("‹ ") + s + focusStyle.Render(" ›")
	}
	return "  " + s
}

func (m Model) bldDowsView(focused bool) string {
	var parts []string
	for i := 0; i < 7; i++ {
		s := dowLetters[i]
		if m.bldDows[i] {
			s = okStyle.Render(s)
		} else {
			s = faintStyle.Render(s)
		}
		if focused && i == m.bldDowCursor {
			s = "[" + s + "]"
		} else {
			s = " " + s + " "
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "")
}

func (m Model) bldControlView(c int, focused bool) (string, string) {
	switch c {
	case ctrlFreq:
		return "frequency:", bldArrows(focused, bldFreqNames[m.bldFreq])
	case ctrlInterval:
		unit := "min"
		if m.bldFreq == 1 {
			unit = "hours"
		}
		return "every:", bldArrows(focused, fmt.Sprintf("%d %s", m.bldInterval, unit))
	case ctrlHour:
		return "hour:", bldArrows(focused, fmt.Sprintf("%02d", m.bldHour))
	case ctrlMinute:
		return "minute:", bldArrows(focused, fmt.Sprintf("%02d", m.bldMin))
	case ctrlDom:
		return "day:", bldArrows(focused, strconv.Itoa(m.bldDom))
	case ctrlDows:
		return "weekday(s):", m.bldDowsView(focused)
	}
	return "", ""
}

func (m Model) renderBuilder(rowW int) string {
	var b strings.Builder
	controls := m.bldControls()
	for i, c := range controls {
		focused := i == m.bldFocus
		label, val := m.bldControlView(c, focused)
		marker := "  "
		lbl := fmt.Sprintf("%-12s", label)
		if focused {
			marker = focusStyle.Render("▸ ")
			lbl = focusStyle.Render(lbl)
		}
		b.WriteString(marker)
		b.WriteString(lbl)
		b.WriteString(" ")
		b.WriteString(val)
		b.WriteString("\n")
	}
	expr := m.bldExpr()
	b.WriteString("\n")
	b.WriteString(schedStyle.Render(trunc("  → "+expr, rowW)))
	b.WriteString("\n")
	b.WriteString(descStyle.Render(trunc("  → "+describe(expr), rowW)))
	return b.String()
}

func (m Model) renderDetails(rowW int) string {
	if len(m.jobs) == 0 {
		return faintStyle.Render("—")
	}
	job := m.jobs[m.cursor]
	var b strings.Builder
	status := okStyle.Render("● enabled")
	if !job.Enabled {
		status = faintStyle.Render("○ paused")
	}
	b.WriteString(headerStyle.Render(trunc(job.Name, rowW-12)))
	b.WriteString("  ")
	b.WriteString(status)
	b.WriteString("\n")
	b.WriteString(descStyle.Render(trunc("$ "+job.Command, rowW)))
	b.WriteString("\n")
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

type keyHint struct{ key, label string }

func (m Model) footerKeys() []keyHint {
	switch m.mode {
	case modeForm:
		return []keyHint{{"tab", "field"}, {"^t", "schedules"}, {"^b", "build"}, {"enter", "save"}, {"esc", "cancel"}}
	case modeBuilder:
		return []keyHint{{"tab", "field"}, {"←/→", "change"}, {"spc", "toggle"}, {"enter", "use"}, {"esc", "back"}}
	case modePicker:
		if m.pickSaving {
			return []keyHint{{"enter", "save"}, {"esc", "cancel"}}
		}
		return []keyHint{{"↑/↓", "move"}, {"enter", "use"}, {"s", "save"}, {"d", "delete"}, {"esc", "back"}}
	case modeRuns:
		return []keyHint{{"↑/↓", "select"}, {"enter", "output"}, {"esc", "back"}}
	case modeOutput:
		return []keyHint{{"↑/↓", "scroll"}, {"pgup/dn", "page"}, {"esc", "back"}}
	case modeConfirmDelete:
		return []keyHint{{"y", "delete"}, {"n", "cancel"}}
	default:
		return []keyHint{{"↑/↓", "select"}, {"a", "add"}, {"e", "edit"}, {"spc", "pause"}, {"enter", "logs"}, {"d", "delete"}, {"q", "quit"}}
	}
}

func renderChip(h keyHint) string {
	return chipStyle.Render(" "+h.key+" ") + " " + chipLabelStyle.Render(h.label)
}

func (m Model) footer() string {
	keys := m.footerKeys()
	w := m.width
	if w <= 0 {
		w = 80
	}

	chips := make([]string, len(keys))
	plain := make([]string, len(keys))
	for i, h := range keys {
		chips[i] = renderChip(h)
		plain[i] = h.key + " " + h.label
	}

	if line := strings.Join(chips, "  "); lipgloss.Width(line) <= w {
		return line
	}
	if line := strings.Join(plain, " · "); lipgloss.Width(line) <= w {
		return chipLabelStyle.Render(line)
	}
	for i, p := range plain {
		plain[i] = chipLabelStyle.Render(trunc(p, w))
	}
	return strings.Join(plain, "\n")
}

func describe(expr string) string {
	s, err := schedule.Parse(expr)
	if err != nil {
		return "(?) " + err.Error()
	}
	return s.Describe()
}
