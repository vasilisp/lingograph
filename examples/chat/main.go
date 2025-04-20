package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/openai"
	"github.com/vasilisp/lingograph/store"
)

func stdinActor() lingograph.Actor {
	return lingograph.NewActor(lingograph.User, func(history []lingograph.Message, store store.Store) (string, error) {
		reader := bufio.NewReader(os.Stdin)

		text, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		return strings.TrimSpace(text), nil
	})
}

func main() {
	chat := lingograph.NewSliceChat()

	stdinActor := stdinActor()
	openAIModel := openai.NewModel(openai.GPT4oMini, openai.APIKeyFromEnv())
	openAIActor := openai.NewActor(openAIModel, "You are a helpful assistant.")

	pipeline := lingograph.Loop(
		lingograph.Chain(
			stdinActor.Pipeline(nil, false, 0),
			openAIActor.Pipeline(func(message lingograph.Message) {
				fmt.Println(message.Content)
			}, false, 3),
		),
		10,
	)

	pipeline.Execute(chat)
}
