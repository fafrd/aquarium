package actor

import (
	"fmt"

	"aquarium/ai"
	"math/rand"

	"time"
)

type Actor struct {
	Id             string
	IterationCount int
	CommandState   string
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

	var nextCommand string
	var err error

	if a.IterationCount == 1 {
		nextCommand, err = ai.GenInitialDialogue()
	} else {
		nextCommand, err = ai.GenNextDialogue(a.CommandState)
	}

	if err != nil {
		fmt.Printf("Actor %s fatal error: %s\n", a.Id, err)
		close(a.quit)
		return
	}

	fmt.Printf("%s iteration %d: %s\n", a.Id, a.IterationCount, nextCommand)

	// TODO: Execute command
}
