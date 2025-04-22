package main

import (
	"fmt"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/extra"
	"github.com/vasilisp/lingograph/openai"
	"github.com/vasilisp/lingograph/store"
)

func main() {
	chat := lingograph.NewSliceChat()

	client := openai.NewClient(openai.APIKeyFromEnv())
	openAIActor := openai.NewActor(client, openai.GPT41Nano, "You are a helpful assistant.")

	// dummy; EOF will terminate
	continueChat := store.Var[bool]{}
	store.Set(chat.Store(), continueChat, true)

	pipeline := lingograph.While(
		continueChat,
		lingograph.Chain(
			extra.Stdin().Pipeline(nil, false, 0),
			openAIActor.Pipeline(func(message lingograph.Message) {
				fmt.Println(message.Content)
			}, false, 1),
		),
	)

	pipeline.Execute(chat)
}
