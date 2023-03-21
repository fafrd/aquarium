package main

import (
	"aquarium/actor"
	"sync"
	"time"
	/*
		"bufio"
		"os/exec"
	*/)

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

	/*
		cmd := exec.Command("ls", "-l") // Replace with your command
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Println(err)
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			fmt.Println(err)
			return
		}
		if err := cmd.Start(); err != nil {
			fmt.Println(err)
			return
		}

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				fmt.Println(scanner.Text())
			}
		}()

		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				fmt.Println(scanner.Text())
			}
		}()

		if err := cmd.Wait(); err != nil {
			fmt.Println(err)
			return
		}
	*/

}
