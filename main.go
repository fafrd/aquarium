package main

import (
	"flag"
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
			m.viewportRight = viewport.New(width, msg.Height-headerHeight)
			m.viewportRight.YPosition = headerHeight
			m.viewportLeft.YPosition = headerHeight + 1
			m.viewportRight.YPosition = headerHeight + 1
			m.ready = true
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
	case AppendContentMsg:
		//logger.Debugf("Recieved msg. msg.logContent: %s; msg.terminalContent: %s\n\n", msg.logContent, msg.terminalContent)

		if msg.logContent != "" {
			m.logContent += msg.logContent
			wrappedContentLeft := wordwrap.String(m.logContent, m.viewportLeft.Width)
			m.viewportLeft.SetContent(wrappedContentLeft)
		}

		if msg.terminalContent != "" {
			m.terminalContent = msg.terminalContent
			wrappedContentRight := wrap.String(m.terminalContent, m.viewportRight.Width)
			m.viewportRight.SetContent(wrappedContentRight)
		}
	}

	// ALWAYS scroll to the bottom
	m.viewportLeft.SetYOffset(99999999999)
	m.viewportRight.SetYOffset(99999999999)

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
	goal := flag.String("goal", "Your goal is to run a Minecraft server.",
		`Goal to give the AI. This will be injected within the following statement:

> You now have control of an Ubuntu Linux server.
> [YOUR GOAL WILL BE INSERTED HERE]
> Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.
>
> Respond with a linux command to give to the server.
`)
	debug := flag.Bool("debug", false, "Enable logging of AI prompts to debug.log")
	preserveContainer := flag.Bool("preserve-container", false, "Persist docker container after program completes.")
	iterationLimit := flag.Int("limit", 30, "Maximum number of commands the AI should run.")
	contextMode := flag.String("context-mode", "partial",
		`How much context from the previous command do we give the AI? This is used by the AI to determine what to run next.
- partial: We send the last 100 lines of the terminal output to the AI. (cheap, accurate)
- full: We send the entire terminal output to the AI. (expensive, very accurate)
`)
	aiModel := flag.String("model", "gpt-5-nano", "OpenAI model to use. Ignored if --url is provided. See https://platform.openai.com/docs/models")
	url := flag.String("url", "", "URL to locally hosted endpoint. If provided, this supersedes the --model flag.")

	flag.Parse()

	if *contextMode != "partial" && *contextMode != "full" {
		fmt.Println("Invalid context-mode. Must be 'partial' or 'full'.")
	}

	logch := make(chan string, 10000)  // general log messages; each one is appended (with newline)
	termch := make(chan string, 10000) // terminal log messages; each one completely replaces the previous
	logger.Init(logch, termch, *debug)

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

	go func() {
		if *url != "" {
			*aiModel = "local"
		}

		actor := actor.NewActor(*aiModel, *url, *goal, *contextMode, *iterationLimit)
		<-actor.Loop()
		if !*preserveContainer {
			err := actor.CleanupContainer()
			if err != nil {
				logger.Logf("Error cleaning up container: %s", err)
			}
		}
		logger.Logf("Done.\n")
	}()

	if _, err := p.Run(); err != nil {
		fmt.Println("could not run program:", err)
		os.Exit(1)
	}

}
