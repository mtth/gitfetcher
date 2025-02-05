package main

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gitfetcher "github.com/mtth/gitfetcher/internal"
)

func runSync(ctx context.Context, syncables []gitfetcher.Syncable) error {
	if !isTTY {
		for _, syncable := range syncables {
			if err := syncable.Sync(ctx); err != nil {
				return err
			}
			fmt.Printf("Synced %s.\n", syncable.RootDir())
		}
		return nil
	}

	m := newSyncTracker(ctx, syncables)
	_, err := tea.NewProgram(m).Run()
	return err
}

type syncTracker struct {
	ctx       context.Context
	syncables []gitfetcher.Syncable
	index     int
	spinner   spinner.Model
	progress  progress.Model
	done      bool
}

var (
	currentPkgNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle           = lipgloss.NewStyle().Margin(1, 2)
	checkMark           = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	crossMark           = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).SetString("✗")
)

func newSyncTracker(ctx context.Context, syncables []gitfetcher.Syncable) syncTracker {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	return syncTracker{
		ctx:       ctx,
		syncables: syncables,
		spinner:   s,
		progress:  p,
	}
}

func (m syncTracker) Init() tea.Cmd {
	return tea.Batch(trackedSync(m.ctx, m.syncables[m.index]), m.spinner.Tick)
}

func (m syncTracker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		}
	case syncCompleted:
		if m.index >= len(m.syncables)-1 {
			// Everything's been installed. We're done!
			m.done = true
			return m, tea.Sequence(
				printSyncStatus(msg), // print the last success message
				tea.Quit,             // exit the program
			)
		}

		// Update progress bar
		m.index++
		progressCmd := m.progress.SetPercent(float64(m.index) / float64(len(m.syncables)))

		return m, tea.Batch(
			progressCmd,
			printSyncStatus(msg),                     // print success message above our program
			trackedSync(m.ctx, m.syncables[m.index]), // download the next package
		)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		if newModel, ok := newModel.(progress.Model); ok {
			m.progress = newModel
		}
		return m, cmd
	}
	return m, nil
}

func printSyncStatus(msg syncCompleted) tea.Cmd {
	if msg.err == nil {
		return tea.Printf("%s %s", checkMark, msg.path)
	}
	return tea.Printf("%s %s (%v)", crossMark, msg.path, msg.err)
}

func (m syncTracker) View() string {
	n := len(m.syncables)
	w := lipgloss.Width(fmt.Sprintf("%d", n))

	if m.done {
		return doneStyle.Render(fmt.Sprintf("Done! Synced %d repositories.\n", n))
	}

	pkgCount := fmt.Sprintf(" %*d/%*d", w, m.index, w, n)
	spin := m.spinner.View() + " "
	prog := m.progress.View()

	root := currentPkgNameStyle.Render(m.syncables[m.index].RootDir())
	info := lipgloss.NewStyle().Render("Syncing " + root)

	return spin + info + "\n\n" + prog + pkgCount
}

type syncCompleted struct {
	path string
	err  error
}

func trackedSync(ctx context.Context, s gitfetcher.Syncable) tea.Cmd {
	return func() tea.Msg {
		err := s.Sync(ctx)
		return syncCompleted{path: s.RootDir(), err: err}
	}
}
