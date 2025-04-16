package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/openai"
)

func stdinActor() lingograph.Actor {
	return lingograph.NewActor(lingograph.User, func(history []lingograph.Message) (string, error) {
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
	openAIActor := openai.NewModel(openai.GPT4oMini, openai.APIKeyFromEnv()).Actor(3)

	pipeline := lingograph.NewChain(
		lingograph.NewUserPrompt("You are a helpful assistant.", false),
		lingograph.NewLoop(
			lingograph.NewChain(
				stdinActor.Pipeline(nil, false),
				openAIActor.Pipeline(func(message lingograph.Message) {
					fmt.Println(message.Content)
				}, false),
			),
			10,
		),
	)

	pipeline.Execute(chat)
}
