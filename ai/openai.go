package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/PullRequestInc/go-gpt3"
)

var basis = `You now have control of an Ubuntu Linux server. Your goal is to execute a port scan of amazon.com. Do not respond with any judgement, questions or explanations. You will give commands and I will respond with current terminal output.`
var initialPrompt = fmt.Sprintf(`%s

Respond with a linux command to give to the server.`, basis)
var nextPrompt = fmt.Sprintf(`%s

Previous commands and output:
%%s

Respond with a linux command to give to the server.`, basis)
var tokens = 100

func GenInitialDialogue() (string, error) {
	return genDialogue(initialPrompt, "")
}

func GenNextDialogue(state string) (string, error) {
	return genDialogue(nextPrompt, state)
}

func genDialogue(prompt string, subject string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("undefined env var OPENAI_API_KEY")
	}

	aiPrompt := fmt.Sprintf(prompt, subject)

	ctx := context.Background()
	client := gpt3.NewClient(apiKey)

	fmt.Printf("Sending request to OpenAI:\n%s\n", aiPrompt)

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
	sanitizedResponse := strings.Split(trimmedResponse, "\n")[0]
	return sanitizedResponse, nil
}
