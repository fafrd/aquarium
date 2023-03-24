package ai

import (
	"aquarium/logger"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/PullRequestInc/go-gpt3"
)

const initialPrompt = `You now have control of an Ubuntu Linux server. %s Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.

Respond with a linux command to give to the server.\n\n`

const nextPrompt = `You now have control of an Ubuntu Linux server. %s Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.

Previous commands and outcomes:
%sGive the next input to the terminal.\n\n`

const outcomeSingle = `A Linux command was run, and this was its output:

%s

The original command was '%s'. What was the outcome?\n\n`

const fragmentSummary = `This is the partial output of a Linux command. Please summarize what happened in this Linux command.

%s\n\n`

const totalSummary = `A Linux command was run, and it had a very long output. The following segments are the summaries of each part of the output, in order:

%sThe original command was '%s'. What was the outcome?\n\n`

const tokens = 200

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

func GenCommandOutcome(previousCommand string, previousOutput string) (string, error) {
	if previousOutput == "" {
		return "There was no output from this command.", nil
	}

	prompt := fmt.Sprintf(outcomeSingle, previousOutput, previousCommand)
	response, err := genDialogue(prompt)

	if err != nil {
		if strings.Contains(fmt.Sprintf("%s", err), "Please reduce your prompt") {
			logger.Logf("Last command output was too large to process in one request. Splitting output into chunks and summarizing chunks individually...\n")
			// recursively chunk up the output and get chunk summaries
			summaries, err := summarizeCommandOutputMultipart(previousOutput)
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

func summarizeCommandOutputMultipart(output string) ([]CommandPair, error) {
	// by entering this function, we assume that the last portion was too large to process in one request
	summaries := make([]CommandPair, 0)

	// split output into two parts
	outputLines := strings.Split(output, "\n")
	midpoint := len(outputLines) / 2
	firstHalf := strings.Join(outputLines[:midpoint], "\n")
	secondHalf := strings.Join(outputLines[midpoint:], "\n")

	// TODO we could run these in parallel
	for _, half := range []string{firstHalf, secondHalf} {
		halfSummary, err := summarizeCommandOutputSingle(half)
		if err == nil {
			summaries = append(summaries, CommandPair{
				Command: half,
				Result:  halfSummary,
			})
		} else {
			if strings.Contains(fmt.Sprintf("%s", err), "Please reduce your prompt") {
				logger.Logf("Last command output was STILL too large to process in one request. Splitting again...\n")
				halfPair, err := summarizeCommandOutputMultipart(half)
				if err != nil {
					return nil, err
				}
				summaries = append(summaries, halfPair...)
			} else {
				return nil, err
			}
		}
	}

	return summaries, nil
}

func summarizeCommandOutputSingle(output string) (string, error) {
	logger.Logf("Summarizing output chunk...\n")
	prompt := fmt.Sprintf(fragmentSummary, output)
	return genDialogue(prompt)
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
