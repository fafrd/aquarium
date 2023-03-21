package actor

import (
	//"aquarium/ai"
	"bytes"
	"fmt"

	"context"
	"math/rand"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"time"
)

type Actor struct {
	containerId    string
	ctx            context.Context
	cli            *client.Client
	Id             string
	IterationCount int
	commandState   string
	quit           chan struct{}
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
	a.ctx = ctx
	a.cli = cli
	if err := cli.NetworkConnect(ctx, "aquarium", resp.ID, nil); err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	fmt.Printf("%s Container started with id %s\n", a.Id, a.containerId)

	go func() {
		defer close(done)
		for {
			select {
			case <-a.quit:
				return
			default:
				a.iteration()
				time.Sleep(2 * time.Second)
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

	/*
		if a.IterationCount == 1 {
			nextCommand, err = ai.GenInitialDialogue()
		} else {
			nextCommand, err = ai.GenNextDialogue(a.commandState)
		}
	*/
	// shortcut
	nextCommand = "sudo nmap -sS -Pn -p- amazon.com"

	if err != nil {
		handleError(err)
		return
	}

	fmt.Printf("%s iteration %d: executing %s\n", a.Id, a.IterationCount, nextCommand)

	// Execute command in container
	if err != nil {
		handleError(err)
		return
	}

	execResp, err := a.cli.ContainerExecCreate(a.ctx, a.containerId, types.ExecConfig{
		Cmd:          []string{"/bin/bash", "-c", nextCommand},
		AttachStderr: true,
		AttachStdout: true,
	})
	if err != nil {
		handleError(err)
		return
	}

	reader, err := a.cli.ContainerExecAttach(a.ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		handleError(err)
		return
	}
	defer reader.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, reader.Reader); err != nil {
		handleError(err)
		return
	}

	execRespCode, err := a.cli.ContainerExecInspect(a.ctx, execResp.ID)

	fmt.Printf("execRespCode: %d\n", execRespCode.ExitCode)

	//a.commandState = buf.String()
	fmt.Println("result:")
	fmt.Println(stdoutBuf.String())
	fmt.Println(stderrBuf.String())
}
