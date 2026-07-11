package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/MakarMolochaev/cronit/internal/storage"
)

type Model struct {
	jobs   []storage.Job
	cursor int
}

func Run(db *storage.Database) error {
	jobs, err := db.Jobs()
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(Model{jobs: jobs}).Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.jobs)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	var b strings.Builder
	b.WriteString("cronit\n\n")
	for i, job := range m.jobs {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		b.WriteString(cursor + job.Schedule + "  " + job.Command + "\n")
	}
	b.WriteString("\n↑/↓: выбор · q: выход\n")
	return tea.NewView(b.String())
}
