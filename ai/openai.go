package ai

import (
	"aquarium/logger"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
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
	tokens              = 200
	recursionDepthLimit = 3
)

type CommandPair struct {
	Command string
	Result  string
}

func (c CommandPair) String() string {
	return fmt.Sprintf("%s\n%s", c.Command, c.Result)
}

func GenInitialCommand(model string, url string, goal string) (string, error) {
	prompt := fmt.Sprintf(initialPrompt, goal)
	result, err := genDialogue(model, url, prompt, true)
	if err != nil {
		return "", err
	}

	firstLine := strings.Split(result, "\n")[0]
	return firstLine, nil
}

func GenNextCommand(model string, url string, goal string, previousCommands []CommandPair) (string, error) {
	var previousCommandsString string
	for _, pair := range previousCommands {
		previousCommandsString += fmt.Sprintf("%s\n\n", pair)
	}

	prompt := fmt.Sprintf(nextPrompt, goal, previousCommandsString)
	result, err := genDialogue(model, url, prompt, true)
	if err != nil {
		return "", err
	}

	firstLine := strings.Split(result, "\n")[0]
	return firstLine, nil
}

func GenCommandOutcomeTruncated(model string, url string, previousCommand string, previousOutput string) (string, error) {
	prompt := fmt.Sprintf(outcomeTruncated, previousOutput, previousCommand)
	return genDialogue(model, url, prompt, false)
}

func GenCommandOutcome(model string, url string, previousCommand string, previousOutput string) (string, error) {
	if previousOutput == "" {
		return "There was no output from this command.", nil
	}

	prompt := fmt.Sprintf(outcomeSingle, previousOutput, previousCommand)
	response, err := genDialogue(model, url, prompt, false)

	if err != nil {
		if strings.Contains(fmt.Sprintf("%s", err), "Please reduce the length of the messages") {
			logger.Logf("Last command output was too large to process in one request. Splitting output into chunks and summarizing chunks individually.\n")

			// recursively chunk up the output and get chunk summaries
			summaries, err := summarizeCommandOutputMultipart(model, url, previousOutput, 1)
			if err != nil {
				return "", err
			}

			// then ask for the outcome of those summaries
			response, err = determineOutcomeOfSummaryChunks(model, url, previousCommand, summaries)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	return response, nil
}

func summarizeCommandOutputMultipart(model string, url string, output string, recursionDepth int) ([]CommandPair, error) {
	summaries := make([]CommandPair, 0)

	outputLines := strings.Split(output, "\n")
	midpoint := len(outputLines) / 2
	firstHalf := strings.Join(outputLines[:midpoint], "\n")
	secondHalf := strings.Join(outputLines[midpoint:], "\n")

	resultChan := make(chan []CommandPair)
	errChan := make(chan error)
	var wg sync.WaitGroup

	wg.Add(2)

	go summarizeCommandOutputSingle(model, url, firstHalf, recursionDepth, resultChan, errChan, &wg)
	go summarizeCommandOutputSingle(model, url, secondHalf, recursionDepth, resultChan, errChan, &wg)

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

func summarizeCommandOutputSingle(model string, url string, half string, recursionDepth int, resultChan chan<- []CommandPair, errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	logger.Logf("Summarizing chunk...\n")
	prompt := fmt.Sprintf(fragmentSummary, half)
	halfSummary, err := genDialogue(model, url, prompt, false)
	if err == nil {
		resultChan <- []CommandPair{{
			Command: half,
			Result:  halfSummary,
		}}
	} else {
		if strings.Contains(fmt.Sprintf("%s", err), "Please reduce the length of the messages") {
			logger.Logf("Last command output was STILL too large to process in one request. Splitting again...\n")
			if recursionDepth+1 > recursionDepthLimit {
				errChan <- fmt.Errorf("recursion depth limit exceeded. Output from last command was too large. (limit is %d, which implies a max of %d requests to OpenAI)", recursionDepthLimit, int(math.Pow(2, float64(recursionDepthLimit))))
				return
			}
			halfPair, err := summarizeCommandOutputMultipart(model, url, half, recursionDepth+1)
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

func determineOutcomeOfSummaryChunks(model string, url string, command string, summaries []CommandPair) (string, error) {
	var previousSummariesString string
	for i, pair := range summaries {
		previousSummariesString += fmt.Sprintf("Part %d:\n%s\n\n", i+1, pair.Result)
	}

	prompt := fmt.Sprintf(totalSummary, previousSummariesString, command)
	return genDialogue(model, url, prompt, false)
}

func genDialogue(model string, url string, aiPrompt string, expectsCommand bool) (string, error) {
	if model != "local" {
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return "", errors.New("undefined env var OPENAI_API_KEY")
		}

		ctx := context.Background()
		client := gpt3.NewClient(apiKey)

		logger.Debugf("### Sending request to OpenAI:\n%s\n\n", aiPrompt)

		messages := []gpt3.ChatCompletionRequestMessage{
			{
				Role:    "user",
				Content: aiPrompt,
			},
		}
		request := gpt3.ChatCompletionRequest{
			Model:       model,
			Messages:    messages,
			MaxTokens:   tokens,
			Temperature: gpt3.Float32Ptr(0.0),
		}
		resp, err := client.ChatCompletion(ctx, request)

		if err != nil {
			logger.Debugf("### ERROR from OpenAI:\n%s\n\n", err)
			return "", err
		}

		trimmedResponse := strings.TrimSpace(resp.Choices[0].Message.Content)
		logger.Debugf("### Received response from OpenAI:\n%s\n\n\n", trimmedResponse)
		return trimmedResponse, nil
	} else {
		var aiPromptInstruction string
		if expectsCommand {
			aiPromptInstruction = fmt.Sprintf("\n\n### Instructions:\n%s\n### Response:\n$", aiPrompt)
		} else {
			aiPromptInstruction = fmt.Sprintf("\n\n### Instructions:\n%s\n### Response:\n", aiPrompt)
		}

		data := struct {
			Prompt string   `json:"prompt"`
			Stop   []string `json:"stop"`
		}{
			Prompt: aiPromptInstruction,
			Stop:   []string{"\n", "###"},
		}
		payloadBytes, err := json.Marshal(data)
		if err != nil {
			logger.Debugf("### ERROR marshalling json:\n%s\n\n", err)
			return "", err
		}
		body := bytes.NewReader(payloadBytes)

		req, err := http.NewRequest("POST", "http://localhost:8000/v1/completions", body)
		if err != nil {
			logger.Debugf("### ERROR constructing http request:\n%s\n\n", err)
			return "", err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

		logger.Debugf("### Sending request to local model:\n%s\n\n", aiPromptInstruction)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			logger.Debugf("### ERROR from local model:\n%s\n\n", err)
			return "", err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Debugf("### ERROR from local model:\n%s\n\n", err)
			return "", err
		}

		logger.Debugf("### Received raw response from local model:\n%s\n\n\n", respBody)

		// llama-cpp-python responds in the form of:
		// {"id":"cmpl-edfe21b1-01f4-4fb0-aef3-60b2e141404d","object":"text_completion","created":1681768157,"model":"../../13B/ggml-model-q4_0.bin","choices":[{"text":" sudo -i","index":0,"logprobs":null,"finish_reason":"stop"}],"usage":{"prompt_tokens":65,"completion_tokens":4,"total_tokens":69}

		var respBodyMap map[string]interface{}
		err = json.Unmarshal(respBody, &respBodyMap)
		if err != nil {
			logger.Debugf("### ERROR from local model:\n%s\n\n", err)
			return "", err
		}

		// type assertion magic
		choices := respBodyMap["choices"].([]interface{})
		choice := choices[0].(map[string]interface{})
		text := choice["text"].(string)
		trimmedResponse := strings.TrimSpace(text)
		if trimmedResponse == "" {
			return "", errors.New("empty response from local model")
		}

		logger.Debugf("### Received response from local model:\n%s\n\n\n", trimmedResponse)
		return trimmedResponse, nil
	}
}
