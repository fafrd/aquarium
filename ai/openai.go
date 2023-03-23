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

var initialPrompt = `You now have control of an Ubuntu Linux server. %s Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.

Respond with a linux command to give to the server.`
var nextPrompt = `You now have control of an Ubuntu Linux server. %s Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.

Previous commands and output:
%s

Give the next input to the terminal.`
var tokens = 100

func GenInitialDialogue(goal string) (string, error) {
	prompt := fmt.Sprintf(initialPrompt, goal)
	return genDialogue(prompt)
}

func GenNextDialogue(goal string, previousOutput string) (string, error) {
	prompt := fmt.Sprintf(nextPrompt, goal, previousOutput)
	return genDialogue(prompt)
}

func genDialogue(aiPrompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("undefined env var OPENAI_API_KEY")
	}

	ctx := context.Background()
	client := gpt3.NewClient(apiKey)

	logger.Debugf("### Sending request to OpenAI:\n%s\n", aiPrompt)

	resp, err := client.CompletionWithEngine(ctx, "text-davinci-003", gpt3.CompletionRequest{
		Prompt:      []string{aiPrompt},
		MaxTokens:   gpt3.IntPtr(tokens),
		Temperature: gpt3.Float32Ptr(0.0),
		Echo:        false,
	})
	if err != nil {
		return "", err
	}

	trimmedResponse := strings.TrimSpace(resp.Choices[0].Text)
	logger.Debugf("### Received response from OpenAI:\n%s\n", trimmedResponse)

	sanitizedResponse := strings.Split(trimmedResponse, "\n")[0]
	return sanitizedResponse, nil
}
