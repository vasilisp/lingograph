package main

import (
	"fmt"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/openai"
)

func main() {
	chat := lingograph.NewSliceChat()

	openAIActor := openai.NewModel(openai.GPT4oMini, openai.APIKeyFromEnv()).Actor(3)

	chain := lingograph.NewChain(
		lingograph.NewUserPrompt("Remind me what date it is today.", false),
		openAIActor.Pipeline(nil, true),
	)

	chain.Execute(chat)

	// history trimmed after LLM step, only one message left
	for _, message := range chat.History() {
		fmt.Println(message.Role, ":", message.Content)
	}
}
