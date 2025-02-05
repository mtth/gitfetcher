package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	humanize "github.com/dustin/go-humanize"
	gitfetcher "github.com/mtth/gitfetcher/internal"
)

func runStatus(syncables []gitfetcher.Syncable) error {
	var rows []table.Row
	for _, s := range syncables {
		rows = append(rows, table.Row{
			s.RootDir(),
			humanize.Time(s.LastSyncedAt()),
			s.SyncStatus().String(),
		})
	}

	if !isTTY {
		for _, r := range rows {
			fmt.Println(strings.Join(r, "\t"))
		}
		return nil
	}

	columns := []table.Column{
		{Title: "Path", Width: 40},
		{Title: "Last synced", Width: 20},
		{Title: "Status", Width: 10},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)
	m := statusTable{t}
	_, err := tea.NewProgram(m).Run()
	return err
}

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type statusTable struct {
	table table.Model
}

func (m statusTable) Init() tea.Cmd { return nil }

func (m statusTable) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m statusTable) View() string {
	return baseStyle.Render(m.table.View()) + "\n" + m.table.HelpView() + "\n"
}
