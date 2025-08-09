package actor

import (
	"aquarium/ai"
	"aquarium/logger"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"context"
	"math/rand"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"time"
)

type Actor struct {
	cli                   *client.Client
	ctx                   context.Context
	lastCommand           string
	lastCommandOutput     string
	terminalStateString   string
	terminalStateOutcomes []ai.CommandPair // [command: outcome, command: outcome, etc]
	containerId           string
	model                 string
	url                   string
	goal                  string
	contextMode           string
	id                    string
	iterationCount        int
	iterationLimit        int
	terminalConnection    types.HijackedResponse
	quit                  chan struct{}
}

func NewActor(model string, url string, goal string, contextMode string, iterationLimit int) *Actor {
	rand.Seed(time.Now().UnixNano())
	id := fmt.Sprintf("%08x", rand.Uint32())

	return &Actor{
		model:          model,
		url:            url,
		goal:           goal,
		contextMode:    contextMode,
		iterationLimit: iterationLimit,
		id:             id,
		iterationCount: 0,
		quit:           make(chan struct{}),
	}
}

func (a *Actor) Loop() <-chan struct{} {
	done := make(chan struct{})
	logger.Logf("%s Starting actor loop.\n", a.id)
	logger.Logf("%s Prompt: %s\n", a.id, a.goal)
	logger.Logf("%s Model: %s\n", a.id, a.model)
	logger.Logf("%s Context mode: %s\n", a.id, a.contextMode)

	// instantiate docker container
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.41"))
	if err != nil {
		panic(err)
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: "aquarium",
			Cmd:   []string{"tail", "-f", "/dev/null"}, // wait indefinitely
		},
		&container.HostConfig{
			NetworkMode: "aquarium",
			SecurityOpt: []string{"apparmor:unconfined"},
		}, nil, nil, "")
	if err != nil {
		panic(err)
	}

	a.containerId = resp.ID
	if err := cli.NetworkConnect(ctx, "aquarium", resp.ID, nil); err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	logger.Logf("%s Container started with id %s\n", a.id, a.containerId)

	// create actor connection to terminal
	terminalExecConfig, err := cli.ContainerExecCreate(ctx, a.containerId, types.ExecConfig{
		Tty:          true,
		Cmd:          []string{"/bin/bash"},
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
	})
	if err != nil {
		panic(err)
	}

	terminalExecConnection, err := cli.ContainerExecAttach(ctx, terminalExecConfig.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		panic(err)
	}

	// initialize terminal
	terminalExecConnection.Conn.Write([]byte("su ubuntu\n"))
	terminalExecConnection.Conn.Write([]byte("cd\n"))
	terminalExecConnection.Conn.Write([]byte("script -f /tmp/out\n")) // write all terminal output to file /tmp/out
	terminalExecConnection.Conn.Write([]byte("/bin/bash\n"))
	a.terminalConnection = terminalExecConnection
	a.cli = cli
	a.ctx = ctx

	logger.Logf("%s Container terminal attached: %s\n", a.id, a.containerId)

	// Log all output from actor's terminal to logger.lLogTerminalf()
	go func() {
		for {
			time.Sleep(50 * time.Millisecond) // terminal logging interval

			output, err := a.ReadTerminalOut()
			if err != nil {
				if !strings.Contains(err.Error(), "is not running") {
					logger.Logf("Docker terminal logging error: %s\n", err)
				}
				return
			}

			logger.LogTerminalf("%s", output)
		}
	}()

	go func() {
		defer close(done)
		for {
			select {
			case <-a.quit:
				return
			default:
				a.iteration()
				time.Sleep(1000 * time.Millisecond) // actor loop interval. meant to keep output slow and readable. can be removed
			}
		}
	}()

	return done
}

func (a *Actor) iteration() {
	a.iterationCount++
	if (a.iterationLimit > 0) && (a.iterationCount > a.iterationLimit) {
		logger.Logf("Actor %s iteration limit reached. Quitting.\n", a.id)
		close(a.quit)
		return
	}

	handleError := func(err error) {
		logger.Logf("Actor %s fatal error: %s\n", a.id, err)
		close(a.quit)
	}

	getLastProcessPid := func() (int, error) {
		execId, err := a.cli.ContainerExecCreate(a.ctx, a.containerId, types.ExecConfig{
			Cmd:          []string{"cat", "/tmp/last.pid"},
			AttachStderr: false,
			AttachStdout: true,
		})
		if err != nil {
			return 0, err
		}
		attachment, err := a.cli.ContainerExecAttach(a.ctx, execId.ID, types.ExecStartCheck{})
		if err != nil {
			return 0, err
		}
		defer attachment.Close()

		var stdoutBuf bytes.Buffer
		_, err = stdcopy.StdCopy(&stdoutBuf, io.Discard, attachment.Reader)
		if err != nil {
			return 0, err
		}

		pid := strings.TrimSpace(stdoutBuf.String())
		if pid == "" {
			// No PID file exists yet (first run)
			return 0, nil
		}
		pidInt, err := strconv.Atoi(pid)
		if err != nil {
			return 0, err
		}

		return pidInt, nil
	}

	isLastProcessRunning := func() (bool, error) {
		lastProcessPid, err := getLastProcessPid()
		if err != nil {
			return false, err
		}
		if lastProcessPid == 0 {
			// No previous process to check (first run)
			return false, nil
		}
		execId, err := a.cli.ContainerExecCreate(a.ctx, a.containerId, types.ExecConfig{
			Cmd:          []string{"ls", "/proc/" + strconv.Itoa(lastProcessPid)},
			AttachStderr: true,
			AttachStdout: true,
		})
		if err != nil {
			return false, err
		}
		attachment, err := a.cli.ContainerExecAttach(a.ctx, execId.ID, types.ExecStartCheck{})
		if err != nil {
			return false, err
		}
		defer attachment.Close()

		var stderrBuf bytes.Buffer
		_, err = stdcopy.StdCopy(io.Discard, &stderrBuf, attachment.Reader)
		if err != nil {
			return false, err
		}

		output := stderrBuf.String()
		if strings.Contains(output, "cannot access") {
			return false, nil
		}

		return true, nil
	}

	var nextCommand string
	var err error

	if a.iterationCount == 1 {
		logger.Logf("%s iteration %d: asking AI for next command...\n", a.id, a.iterationCount)
		nextCommand, err = ai.GenInitialCommand(a.model, a.url, a.goal)
		if err != nil {
			handleError(err)
			return
		}
	} else {
		logger.Logf("%s iteration %d: asking AI to summarize output of previous command... \n", a.id, a.iterationCount)

		var prevCommandOutcome string
		if a.contextMode == "full" {
			prevCommandOutcome, err = ai.GenCommandOutcome(a.model, a.url, a.lastCommand, a.lastCommandOutput)
		} else {
			lines := strings.Split(a.lastCommandOutput, "\n")
			if len(lines) <= 10 {
				// short output, so use the normal approach
				prevCommandOutcome, err = ai.GenCommandOutcome(a.model, a.url, a.lastCommand, a.lastCommandOutput)
			} else {
				// long output, so summarize last X lines only
				const CONTEXT_LINES = 10
				lastCommandOutputTruncated := strings.Join(lines[len(lines)-CONTEXT_LINES:], "\n")
				prevCommandOutcome, err = ai.GenCommandOutcomeTruncated(a.model, a.url, a.lastCommand, lastCommandOutputTruncated)
			}
		}
		if err != nil {
			handleError(err)
			return
		}

		// append to a.terminalStateOutcomes
		a.terminalStateOutcomes = append(a.terminalStateOutcomes, ai.CommandPair{
			Command: a.lastCommand,
			Result:  prevCommandOutcome,
		})

		logger.Logf("%s iteration %d: asking AI for next command...\n", a.id, a.iterationCount)
		nextCommand, err = ai.GenNextCommand(a.model, a.url, a.goal, a.terminalStateOutcomes)
		if err != nil {
			handleError(err)
			return
		}
	}

	// rewrite apt-get as apt-get -qq
	if !strings.Contains(nextCommand, "-q") {
		pattern := regexp.MustCompile(`(apt(?:-get)?\s+(?:install|upgrade)\s+)(\S+)`)
		replacement := "${1}-qq $2"
		nextCommand = pattern.ReplaceAllString(nextCommand, replacement)
	}
	// add -y to apt, apt-get and add-apt-repository
	if !strings.Contains(nextCommand, "-y") {
		pattern := regexp.MustCompile(`(apt(?:-get)?\s+(?:install|upgrade)\s+|add-apt-repository\s+)(\S+)`)
		replacement := "${1}-y $2"
		nextCommand = pattern.ReplaceAllString(nextCommand, replacement)
	}
	// rewrite wget as wget -nv
	if !strings.Contains(nextCommand, "-nv") && !strings.Contains(nextCommand, "apt") {
		pattern := regexp.MustCompile(`(wget\s+)(\S+)`)
		replacement := "${1}-nv $2"
		nextCommand = pattern.ReplaceAllString(nextCommand, replacement)
	}
	// rewrite tar -v as tar
	pattern := regexp.MustCompile(`(tar\s+)([^\s]*v[^\s]*\s+)(.+)`)
	if pattern.MatchString(nextCommand) {
		noVFlag := strings.ReplaceAll(pattern.FindStringSubmatch(nextCommand)[2], "v", "")
		replacement := fmt.Sprintf("${1}%s $3", noVFlag)
		nextCommand = pattern.ReplaceAllString(nextCommand, replacement)
	}

	if err != nil {
		handleError(err)
		return
	}

	// Check if command starts with a shell builtin that can't be exec'd
	shellBuiltins := []string{"cd", "export", "source", ".", "alias", "unalias", "set", "unset", "eval", "exec", "exit", "return", "break", "continue", "declare", "typeset", "local", "readonly", "shift"}
	commandWords := strings.Fields(nextCommand)
	isBuiltin := false
	if len(commandWords) > 0 {
		firstCommand := commandWords[0]
		for _, builtin := range shellBuiltins {
			if firstCommand == builtin {
				isBuiltin = true
				break
			}
		}
	}

	var realCommand string
	if isBuiltin {
		// For shell builtins, don't use exec since they can't be exec'd
		realCommand = "/bin/bash -c \"echo \\$\\$>/tmp/last.pid; " + strings.ReplaceAll(nextCommand, "\"", "\"'\"'\"") + "\"\n"
	} else {
		realCommand = "/bin/bash -c \"echo \\$\\$>/tmp/last.pid && exec " + strings.ReplaceAll(nextCommand, "\"", "\"'\"'\"") + "\"\n"
	}
	// Execute command in container
	logger.Logf("%s iteration %d: executing %s\n", a.id, a.iterationCount, nextCommand)
	a.terminalConnection.Conn.Write([]byte(realCommand))

	// wait for command to finish- poll isLastProcessRunning() until it returns false
	waitMessageSent := false
	for {
		isRunning, err := isLastProcessRunning()
		time.Sleep(250 * time.Millisecond)
		if err != nil {
			handleError(err)
			return
		}
		if !isRunning {
			break
		}
		if !waitMessageSent {
			logger.Logf("%s iteration %d: waiting for command to finish...\n", a.id, a.iterationCount)
			waitMessageSent = true
		}
		time.Sleep(1 * time.Second)
	}

	// read output
	newTerminalState, err := a.ReadTerminalOut()
	if err != nil {
		handleError(err)
		return
	}

	// calculate new additions between newTerminalState and old terminal state
	oldTerminalStateLineCount := len(strings.Split(a.terminalStateString, "\n"))
	if oldTerminalStateLineCount <= 1 {
		oldTerminalStateLineCount = 5
	}
	newTerminalStateLines := strings.Split(newTerminalState, "\n")
	newTerminalStateLines = newTerminalStateLines[oldTerminalStateLineCount-1 : len(newTerminalStateLines)-2] // take difference

	// update state
	a.lastCommandOutput = strings.Join(newTerminalStateLines, "\n")
	a.lastCommand = nextCommand
	a.terminalStateString = newTerminalState
}

func (a *Actor) ReadTerminalOut() (string, error) {
	// read output
	operatorExecConfig, err := a.cli.ContainerExecCreate(a.ctx, a.containerId, types.ExecConfig{
		Cmd:          []string{"/bin/bash", "-c", "/tmp/logterm"},
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
	})
	if err != nil {
		return "", err
	}
	operatorExecConnection, err := a.cli.ContainerExecAttach(a.ctx, operatorExecConfig.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
	}
	defer operatorExecConnection.Close()

	var stdoutBuf bytes.Buffer
	// we discard stderr here. we are using script(1) which records everything to stdout
	_, err = stdcopy.StdCopy(&stdoutBuf, io.Discard, operatorExecConnection.Reader)
	if err != nil {
		return "", err
	}

	raw := stdoutBuf.String()
	var sanitized string
	raw = strings.ReplaceAll(raw, "\r", "\n")

	// remove any remaining lines that are pure whitespace
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) != "" {
			sanitized += line + "\n"
		}
	}

	// deduplicate subsequent lines w the same content
	lines := strings.Split(sanitized, "\n")
	var result []string

	previousLine := ""
	for _, line := range lines {
		if line != previousLine {
			result = append(result, line)
			previousLine = line
		}
	}

	deduplicated := strings.Join(result, "\n")
	return deduplicated, nil
}

func (a *Actor) CleanupContainer() error {
	logger.Logf("%s: cleaning up container %s\n", a.id, a.containerId)
	time.Sleep(250 * time.Millisecond)
	err := a.cli.ContainerRemove(a.ctx, a.containerId, types.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		return err
	}
	return nil
}
