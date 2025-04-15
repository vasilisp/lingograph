package main

import (
	"fmt"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/openai"
)

func main() {
	chat := lingograph.NewSliceChat()

	system := lingograph.NewSystemPrompt("You are a helpful assistant.")

	user := lingograph.NewUserPrompt("Remind me what date it is today.")

	gptModel := openai.NewModel(openai.GPT4oMini, openai.APIKeyFromEnv())
	llm := openai.NewOpenAIActor(gptModel, nil, 3)

	chain := lingograph.NewChain(system, user, llm)

	chain.Execute(chat)

	if len(chat.History()) > 0 {
		fmt.Println(chat.History()[len(chat.History())-1].Content)
	} else {
		fmt.Println("no response")
	}
}
