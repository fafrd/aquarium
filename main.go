package main

import (
	"aquarium/ai"
	"fmt"
	/*
		"bufio"
		"os/exec"
	*/)

func main() {
	resp, err := ai.GenDialogue()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(resp)

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
