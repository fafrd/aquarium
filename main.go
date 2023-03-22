package main

import (
	"fmt"
	"os"
	"strings"

	"aquarium/actor"
	"aquarium/logger"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/reflow/wrap"
)

type AppendContentMsg struct {
	logContent      string
	terminalContent string
}

const useHighPerformanceRenderer = false
const gap = 8

type model struct {
	logContent      string
	terminalContent string
	ready           bool
	viewportLeft    viewport.Model
	viewportRight   viewport.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "esc" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())

		width := msg.Width/2 - gap
		if !m.ready {
			m.viewportLeft = viewport.New(width, msg.Height-headerHeight)
			m.viewportLeft.YPosition = headerHeight
			m.viewportLeft.HighPerformanceRendering = useHighPerformanceRenderer

			m.viewportRight = viewport.New(width, msg.Height-headerHeight)
			m.viewportRight.YPosition = headerHeight
			m.viewportRight.HighPerformanceRendering = useHighPerformanceRenderer
			m.viewportRight.SetContent(m.terminalContent)
			m.ready = true

			m.viewportLeft.YPosition = headerHeight + 1
			m.viewportRight.YPosition = headerHeight + 1
		} else {
			m.viewportLeft.Width = width
			m.viewportLeft.Height = msg.Height - headerHeight
			m.viewportRight.Width = width
			m.viewportRight.Height = msg.Height - headerHeight
		}
		wrappedContentLeft := wordwrap.String(m.logContent, width)
		m.viewportLeft.SetContent(wrappedContentLeft)
		wrappedContentRight := wrap.String(m.terminalContent, width)
		m.viewportRight.SetContent(wrappedContentRight)

		if useHighPerformanceRenderer {
			cmds = append(cmds, viewport.Sync(m.viewportLeft), viewport.Sync(m.viewportRight))
		}
	case AppendContentMsg:
		if msg.logContent != "" {
			m.logContent += "\n" + msg.logContent
			wrappedContentLeft := wordwrap.String(m.logContent, m.viewportLeft.Width)
			m.viewportLeft.SetContent(wrappedContentLeft)
		}

		if msg.terminalContent != "" {
			m.terminalContent += msg.terminalContent
			wrappedContentRight := wrap.String(m.terminalContent, m.viewportRight.Width)
			m.viewportRight.SetContent(wrappedContentRight)
		}
	}

	// ALWAYS scroll to the bottom
	m.viewportLeft.SetYOffset(99999999999)

	// Handle keyboard and mouse events in the viewport
	m.viewportLeft, cmd = m.viewportLeft.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	vpleft := fmt.Sprintf("%s\n%s", m.headerView(), m.viewportLeft.View())
	vpright := fmt.Sprintf("%s\n%s", m.headerView(), m.viewportRight.View())
	return lipgloss.JoinHorizontal(lipgloss.Center, vpleft, strings.Repeat(" ", gap), vpright)
}

func (m model) headerView() string {
	line := strings.Repeat("─", max(0, m.viewportLeft.Width))
	return line
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	logch := make(chan string, 100)  // general log messages; each one is prepended with newline
	termch := make(chan string, 100) // terminal log messages; each one is appended directly to end
	logger.Init(logch, termch)

	//lorem := "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Suspendisse condimentum neque a purus condimentum tincidunt. Aliquam ut nunc velit. Sed commodo purus non vestibulum placerat. Vivamus tortor dolor, vestibulum non volutpat ut, convallis eu dui. Nam ullamcorper mattis molestie. Maecenas vestibulum sapien nisl, vel iaculis nisi convallis ac. Aenean rhoncus rutrum dui. Suspendisse bibendum purus a mauris ornare ultricies. Maecenas sit amet nunc pellentesque, ullamcorper elit non, egestas tortor."

	p := tea.NewProgram(
		model{logContent: "", terminalContent: string("Container not started.")},
		tea.WithAltScreen(),
	)

	go func() {
		for {
			logMessage := <-logch
			p.Send(AppendContentMsg{logContent: string(logMessage)})
		}
	}()
	go func() {
		for {
			termMessage := <-termch
			p.Send(AppendContentMsg{terminalContent: string(termMessage)})
		}
	}()

	/*
		go func() {
			time.Sleep(1 * time.Second)
			logger.Log("Log message 1 %s, %d", "test", 1)
			logger.LogTerminal("term Log message 1 %s, %d", "testr2", 4)
		}()
		go func() {
			time.Sleep(1 * time.Second)
			logger.Log("Log message 2")
			logger.LogTerminal("term Log message 2")
		}()
	*/

	go func() {
		actor := actor.NewActor()
		<-actor.Loop()
	}()

	if _, err := p.Run(); err != nil {
		fmt.Println("could not run program:", err)
		os.Exit(1)
	}

}
