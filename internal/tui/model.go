package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/velariumai/pdv/internal/config"
	"github.com/velariumai/pdv/internal/download"
)

type Model struct {
	engine *download.Engine
	cfg    *config.Config
}

func NewModel(engine *download.Engine, cfg *config.Config) Model {
	return Model{engine: engine, cfg: cfg}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		if v.String() == "q" || v.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	return "PDV TUI placeholder\nPress q to quit.\n"
}
