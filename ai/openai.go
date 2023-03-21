package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/PullRequestInc/go-gpt3"
)

var prompt = `Solve the following math problem: %s`
var subject = `What is 2 + 2?`
var tokens = 100

func GenDialogue() (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("Missing env var OPENAI_API_KEY")
	}

	aiPrompt := fmt.Sprintf(prompt, subject)

	ctx := context.Background()
	client := gpt3.NewClient(apiKey)

	resp, err := client.CompletionWithEngine(ctx, "text-davinci-001", gpt3.CompletionRequest{
		Prompt:    []string{aiPrompt},
		MaxTokens: gpt3.IntPtr(tokens),
		Echo:      false,
	})
	if err != nil {
		return "", err
	}

	sanitizedresponse := strings.Replace(resp.Choices[0].Text, "\n\n", "\n", -1)
	return sanitizedresponse, nil
}
