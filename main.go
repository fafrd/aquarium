package main

import (
	"aquarium/actor"
	"sync"
	"time"
)

func main() {

	actors := []*actor.Actor{
		actor.NewActor(),
		//actor.NewActor(),
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(len(actors))

	for _, a := range actors {
		go func(actor *actor.Actor) {
			defer wg.Done()
			<-actor.Loop()
		}(a)
		time.Sleep(250 * time.Millisecond)
	}

	go func() {
		defer close(done)
		wg.Wait()
	}()

	<-done
}
