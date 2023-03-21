package actor

import (
	"aquarium/ai"
	"bytes"
	"fmt"
	"io"
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
	fmt.Printf("%s Starting actor loop\n", a.Id)

	// instantiate docker container
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.WithVersion("1.41"))
	if err != nil {
		panic(err)
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: "instance",
		Cmd:   []string{"tail", "-f", "/dev/null"},
	}, nil, nil, nil, "")
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

	fmt.Printf("%s Container started with id %s\n", a.Id, a.containerId)

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
	terminalExecConnection.Conn.Write([]byte(fmt.Sprintf("script -f /tmp/out-%s\n", a.Id))) // write all terminal output to file /tmp/out-id
	terminalExecConnection.Conn.Write([]byte("/bin/bash\n"))
	a.terminalConnection = terminalExecConnection
	a.cli = cli
	a.ctx = ctx

	fmt.Printf("%s Container terminal attached: %s\n", a.Id, a.containerId)

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
		fmt.Printf("Actor %s fatal error: %s\n", a.Id, err)
		close(a.quit)
	}

	a.IterationCount++
	fmt.Printf("%s iteration %d\n", a.Id, a.IterationCount)

	var nextCommand string
	var err error

	if a.IterationCount == 1 {
		nextCommand, err = ai.GenInitialDialogue()
	} else {
		nextCommand, err = ai.GenNextDialogue(a.commandState)
	}
	// shortcut
	//nextCommand = "mkdir test && cd test && pwd"
	if err != nil {
		handleError(err)
		return
	}

	// Execute command in container
	fmt.Printf("%s iteration %d: executing %s\n", a.Id, a.IterationCount, nextCommand)
	a.terminalConnection.Conn.Write([]byte(nextCommand + "\n"))

	// wait for command to finish
	time.Sleep(5 * time.Second) // TODO not correct

	// read output
	// create operator connection to terminal (secondary connection for administration)
	operatorExecConfig, err := a.cli.ContainerExecCreate(a.ctx, a.containerId, types.ExecConfig{
		Cmd:          []string{"cat", fmt.Sprintf("/tmp/out-%s", a.Id)},
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
	lines := strings.Split(capturedTerminalOut, "\n")[4:]
	a.commandState = strings.Join(lines, "\n")

	//fmt.Println("result:")
	//fmt.Println(a.commandState)

}
