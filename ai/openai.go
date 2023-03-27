package ai

import (
	"aquarium/logger"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/PullRequestInc/go-gpt3"
)

const (
	initialPrompt = `You now have control of an Ubuntu Linux server. %s Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output. (This is a noninteractive terminal, so you cannot use nano or vi.)

Respond with a linux command to give to the server.

`
	nextPrompt = `You now have control of an Ubuntu Linux server. %s Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output. (This is a noninteractive terminal, so you cannot use nano or vi.)

Previous commands and outcomes:
%sGive the next input to the terminal.

`
	outcomeSingle = `A Linux command was run, and this was its output:

%s

The original command was '%s'. What was the outcome?

`
	outcomeTruncated = `A Linux command was run, and it had a very long output. This is the last 10 lines:

%s

The original command was '%s'. What was the outcome?

`
	fragmentSummary = `This is the partial output of a Linux command. Please summarize what happened in this Linux command.

%s

`
	totalSummary = `A Linux command was run, and it had a very long output. The following segments are the summaries of each part of the output, in order:

%sThe original command was '%s'. What was the outcome?

`
	tokens = 200
)

type CommandPair struct {
	Command string
	Result  string
}

func (c CommandPair) String() string {
	return fmt.Sprintf("%s\n%s", c.Command, c.Result)
}

func GenInitialDialogue(goal string) (string, error) {
	prompt := fmt.Sprintf(initialPrompt, goal)
	result, err := genDialogue(prompt)
	if err != nil {
		return "", err
	}

	firstLine := strings.Split(result, "\n")[0]
	return firstLine, nil
}

func GenNextDialogue(goal string, previousCommands []CommandPair) (string, error) {
	var previousCommandsString string
	for _, pair := range previousCommands {
		previousCommandsString += fmt.Sprintf("%s\n\n", pair)
	}

	prompt := fmt.Sprintf(nextPrompt, goal, previousCommandsString)
	result, err := genDialogue(prompt)
	if err != nil {
		return "", err
	}

	firstLine := strings.Split(result, "\n")[0]
	return firstLine, nil
}

func GenCommandOutcomeTruncated(previousCommand string, previousOutput string) (string, error) {
	prompt := fmt.Sprintf(outcomeTruncated, previousOutput, previousCommand)
	return genDialogue(prompt)
}

func GenCommandOutcome(previousCommand string, previousOutput string, recursionDepthLimit int) (string, error) {
	if previousOutput == "" {
		return "There was no output from this command.", nil
	}

	prompt := fmt.Sprintf(outcomeSingle, previousOutput, previousCommand)
	response, err := genDialogue(prompt)

	if err != nil {
		if strings.Contains(fmt.Sprintf("%s", err), "Please reduce your prompt") {
			logger.Logf("Last command output was too large to process in one request. Splitting output into chunks and summarizing chunks individually.\n")

			// recursively chunk up the output and get chunk summaries
			summaries, err := summarizeCommandOutputMultipart(previousOutput, 1, recursionDepthLimit)
			if err != nil {
				return "", err
			}

			// then ask for the outcome of those summaries
			response, err = determineOutcomeOfSummaryChunks(previousCommand, summaries)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	return response, nil
}

func summarizeCommandOutputMultipart(output string, recursionDepth int, recursionDepthLimit int) ([]CommandPair, error) {
	summaries := make([]CommandPair, 0)

	outputLines := strings.Split(output, "\n")
	midpoint := len(outputLines) / 2
	firstHalf := strings.Join(outputLines[:midpoint], "\n")
	secondHalf := strings.Join(outputLines[midpoint:], "\n")

	resultChan := make(chan []CommandPair)
	errChan := make(chan error)
	var wg sync.WaitGroup

	wg.Add(2)

	go summarizeCommandOutputSingle(firstHalf, recursionDepth, recursionDepthLimit, resultChan, errChan, &wg)
	go summarizeCommandOutputSingle(secondHalf, recursionDepth, recursionDepthLimit, resultChan, errChan, &wg)

	go func() {
		wg.Wait()
		close(resultChan)
		close(errChan)
	}()

	for {
		select {
		case result, ok := <-resultChan:
			if ok {
				summaries = append(summaries, result...)
			} else {
				resultChan = nil
			}
		case err, ok := <-errChan:
			if ok {
				return nil, err
			} else {
				errChan = nil
			}
		}

		if resultChan == nil && errChan == nil {
			break
		}
	}

	return summaries, nil
}

func summarizeCommandOutputSingle(half string, recursionDepth int, recursionDepthLimit int, resultChan chan<- []CommandPair, errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	logger.Logf("Summarizing chunk...\n")
	prompt := fmt.Sprintf(fragmentSummary, half)
	halfSummary, err := genDialogue(prompt)
	if err == nil {
		resultChan <- []CommandPair{{
			Command: half,
			Result:  halfSummary,
		}}
	} else {
		if strings.Contains(fmt.Sprintf("%s", err), "Please reduce your prompt") {
			logger.Logf("Last command output was STILL too large to process in one request. Splitting again...\n")
			if recursionDepth+1 > recursionDepthLimit {
				errChan <- fmt.Errorf("recursion depth limit exceeded. Output from last command was too large. (limit is %d, which implies a max of %d requests to OpenAI)", recursionDepthLimit, int(math.Pow(2, float64(recursionDepthLimit))))
				return
			}
			halfPair, err := summarizeCommandOutputMultipart(half, recursionDepth+1, recursionDepthLimit)
			if err != nil {
				errChan <- err
			} else {
				resultChan <- halfPair
			}
		} else {
			errChan <- err
		}
	}
}

func determineOutcomeOfSummaryChunks(command string, summaries []CommandPair) (string, error) {
	var previousSummariesString string
	for i, pair := range summaries {
		previousSummariesString += fmt.Sprintf("Part %d:\n%s\n\n", i+1, pair.Result)
	}

	prompt := fmt.Sprintf(totalSummary, previousSummariesString, command)
	return genDialogue(prompt)
}

func genDialogue(aiPrompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("undefined env var OPENAI_API_KEY")
	}

	ctx := context.Background()
	client := gpt3.NewClient(apiKey)

	logger.Debugf("### Sending request to OpenAI:\n%s\n\n", aiPrompt)

	resp, err := client.CompletionWithEngine(ctx, "text-davinci-003", gpt3.CompletionRequest{
		Prompt:      []string{aiPrompt},
		MaxTokens:   gpt3.IntPtr(tokens),
		Temperature: gpt3.Float32Ptr(0.0),
		Echo:        false,
	})
	if err != nil {
		logger.Debugf("### ERROR from OpenAI:\n%s\n\n", err)
		return "", err
	}

	trimmedResponse := strings.TrimSpace(resp.Choices[0].Text)
	logger.Debugf("### Received response from OpenAI:\n%s\n\n\n", trimmedResponse)
	return trimmedResponse, nil
}
