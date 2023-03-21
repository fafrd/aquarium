package actor

import (
	"fmt"

	"context"
	"math/rand"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"time"
)

type Actor struct {
	containerId    string
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
	a.IterationCount++
	fmt.Printf("%s iteration %d\n", a.Id, a.IterationCount)

	/*
		var nextCommand string
		var err error

		if a.IterationCount == 1 {
			nextCommand, err = ai.GenInitialDialogue()
		} else {
			nextCommand, err = ai.GenNextDialogue(a.commandState)
		}

		if err != nil {
			fmt.Printf("Actor %s fatal error: %s\n", a.Id, err)
			close(a.quit)
			return
		}

		fmt.Printf("%s iteration %d: %s\n", a.Id, a.IterationCount, nextCommand)
	*/

	// TODO: Execute command
	// store output into a.commandState
	/*
	   ctx := context.Background()
	   cli, err := client.NewClientWithOpts(client.FromEnv)
	   if err != nil {
	       return "", err
	   }

	   execResp, err := cli.ContainerExecCreate(ctx, a.containerID, types.ExecConfig{
	       Cmd: []string{"/bin/bash", "-c", command},
	   })
	   if err != nil {
	       return "", err
	   }

	   var buf bytes.Buffer
	   execOpts := types.ExecStartCheck{}
	   if err := cli.ContainerExecStart(ctx, execResp.ID, execOpts); err != nil {
	       return "", err
	   }

	   reader, err := cli.ContainerExecAttach(ctx, execResp.ID, types.ExecConfig{})
	   if err != nil {
	       return "", err
	   }
	   defer reader.Close()

	   if _, err := io.Copy(&buf, reader.Reader); err != nil {
	       return "", err
	   }

	   return buf.String(), nil
	*/
}
