package main

import (
	"os"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/extra"
	"github.com/vasilisp/lingograph/openai"
	"github.com/vasilisp/lingograph/store"
)

func main() {
	chat := lingograph.NewSliceChat()

	client := openai.NewClient(openai.APIKeyFromEnv())
	openAIActor := openai.NewActor(client, openai.GPT41Nano, "You are a helpful assistant.")

	pipeline := lingograph.While(
		// dummy; EOF will terminate
		func(store.StoreRO) bool {
			return true
		},
		lingograph.Chain(
			extra.Stdin().Pipeline(nil, false, 0),
			openAIActor.Pipeline(extra.Echoln(os.Stdout, "assistant: "), false, 1),
		),
	)

	pipeline.Execute(chat)
}
