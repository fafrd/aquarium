package actor

import (
	"aquarium/ai"
	"aquarium/logger"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"context"
	"math/rand"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"time"
)

type Actor struct {
	cli                *client.Client
	ctx                context.Context
	containerId        string
	terminalConnection types.HijackedResponse
	Id                 string
	IterationCount     int
	commandState       string
	quit               chan struct{}
}

func NewActor() *Actor {
	rand.Seed(time.Now().UnixNano())
	id := fmt.Sprintf("%08x", rand.Uint32())

	return &Actor{
		Id:             id,
		IterationCount: 0,
		quit:           make(chan struct{}),
	}
}

func (a *Actor) Loop() <-chan struct{} {
	done := make(chan struct{})
	logger.Logf("%s Starting actor loop\n", a.Id)

	// instantiate docker container
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.41"))
	if err != nil {
		panic(err)
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image: "instance",
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

	logger.Logf("%s Container started with id %s\n", a.Id, a.containerId)

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

	logger.Logf("%s Container terminal attached: %s\n", a.Id, a.containerId)

	// Log all output from actor's terminal to logger.LogTerminalf()
	go func() {
		bytesLoggedSoFar := 0
		for {
			time.Sleep(50 * time.Millisecond) // terminal logging interval

			// read output
			operatorExecConfig, err := a.cli.ContainerExecCreate(a.ctx, a.containerId, types.ExecConfig{
				Cmd:          []string{"ansifilter", "/tmp/out"},
				AttachStdin:  true,
				AttachStderr: true,
				AttachStdout: true,
			})
			if err != nil {
				logger.Logf("Docker terminal logging error: %s\n", err)
				return
			}
			operatorExecConnection, err := a.cli.ContainerExecAttach(a.ctx, operatorExecConfig.ID, types.ExecStartCheck{})
			if err != nil {
				logger.Logf("Docker terminal logging error: %s\n", err)
				return
			}
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, operatorExecConnection.Reader); err != nil {
				logger.Logf("Docker terminal logging error: %s\n", err)
				return
			}

			capturedTerminalOut := buf.String()

			// now, log bytes that we have not yet logged
			if len(capturedTerminalOut) > bytesLoggedSoFar {
				raw := capturedTerminalOut[bytesLoggedSoFar:]
				logger.LogTerminalf(raw)
				bytesLoggedSoFar = len(capturedTerminalOut)
			}
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
				time.Sleep(5 * time.Second)
			}
		}
	}()

	return done
}

func (a *Actor) iteration() {
	handleError := func(err error) {
		logger.Logf("Actor %s fatal error: %s\n", a.Id, err)
		close(a.quit)
	}

	getProcs := func() (procs map[int]string, count int, err error) {
		// create operator connection
		operatorExecConfig, err := a.cli.ContainerExecCreate(a.ctx, a.containerId, types.ExecConfig{
			Cmd:          []string{"ps", "-u", "root,ubuntu", "-o", "pid=,user=,comm="},
			AttachStderr: false,
			AttachStdout: true,
		})
		if err != nil {
			return nil, 0, err
		}
		operatorExecConnection, err := a.cli.ContainerExecAttach(a.ctx, operatorExecConfig.ID, types.ExecStartCheck{})
		if err != nil {
			return nil, 0, err
		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, operatorExecConnection.Reader); err != nil {
			return nil, 0, err
		}

		kv := make(map[int]string)
		lines := strings.Split(buf.String(), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) == 3 {
				pid := 0
				fmt.Sscanf(fields[0], "%d", &pid)
				kv[pid] = fields[1]
			}
		}
		return kv, len(kv), nil
	}

	a.IterationCount++
	logger.Logf("%s iteration %d\n", a.Id, a.IterationCount)

	var nextCommand string
	var err error

	if a.IterationCount == 1 {
		nextCommand, err = ai.GenInitialDialogue()
	} else {
		nextCommand, err = ai.GenNextDialogue(a.commandState)
	}
	// shortcut
	//nextCommand = "apt-get update"
	if err != nil {
		handleError(err)
		return
	}

	// rewrite apt-get as apt-get -y
	pattern := regexp.MustCompile(`(apt(?:-get)?\s+(?:install|upgrade)\s+)(\S+)`)
	replacement := "${1}-y $2"
	nextCommand = pattern.ReplaceAllString(nextCommand, replacement)

	_, initialProcCount, err := getProcs()
	if err != nil {
		handleError(err)
		return
	}

	// Execute command in container
	logger.Logf("%s iteration %d: executing %s\n", a.Id, a.IterationCount, nextCommand)
	a.terminalConnection.Conn.Write([]byte(nextCommand + "\n"))

	// wait for command to finish- poll getProcs until it returns the initial # of processes
	// (kind of a hacky approach)
	time.Sleep(250 * time.Millisecond)
	for {
		_, procCount, err := getProcs()
		//logger.Logf("%s iteration %d: %d processes (initial proc count %d)\n", a.Id, a.IterationCount, procCount, initialProcCount)
		if err != nil {
			handleError(err)
			return
		}
		if procCount == initialProcCount {
			break
		}
		logger.Logf("%s iteration %d: waiting for command to finish...\n", a.Id, a.IterationCount)
		time.Sleep(2 * time.Second)
	}

	// read output
	operatorExecConfig, err := a.cli.ContainerExecCreate(a.ctx, a.containerId, types.ExecConfig{
		Cmd:          []string{"cat", "/tmp/out"},
		AttachStderr: true,
		AttachStdout: true,
	})
	if err != nil {
		handleError(err)
		return
	}
	operatorExecConnection, err := a.cli.ContainerExecAttach(a.ctx, operatorExecConfig.ID, types.ExecStartCheck{})
	if err != nil {
		handleError(err)
		return
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, operatorExecConnection.Reader); err != nil {
		handleError(err)
		return
	}

	capturedTerminalOut := strings.TrimSpace(buf.String())
	lines := strings.Split(capturedTerminalOut, "\n")[3:]
	// todo- only pass the last 40 or so lines?
	a.commandState = strings.Join(lines, "\n")

}
