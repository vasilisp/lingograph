package main

import (
	"fmt"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/openai"
)

func main() {
	chat := lingograph.NewSliceChat()

	openAIActor := openai.NewModel(openai.GPT4oMini, openai.APIKeyFromEnv()).Actor("You are a helpful assistant.")

	chain := lingograph.Chain(
		lingograph.UserPrompt("Remind me what date it is today.", false),
		openAIActor.Pipeline(nil, true, 3),
	)

	chain.Execute(chat)

	// history trimmed after LLM step, only one message left
	for _, message := range chat.History() {
		fmt.Println(message.Role, ":", message.Content)
	}
}
