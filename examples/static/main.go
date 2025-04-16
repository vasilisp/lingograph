package main

import (
	"fmt"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/openai"
)

func main() {
	chat := lingograph.NewSliceChat()

	gptModel := openai.NewModel(openai.GPT4oMini, openai.APIKeyFromEnv())

	chain := lingograph.NewChain(
		lingograph.NewSystemPrompt("You are a helpful assistant."),
		lingograph.NewUserPrompt("Remind me what date it is today."),
		openai.NewOpenAIActor(gptModel, nil, 3, true),
	)

	chain.Execute(chat)

	// history trimmed after LLM step, only one message left
	for _, message := range chat.History() {
		fmt.Println(message.Role, ":", message.Content)
	}
}
