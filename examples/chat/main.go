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
	openAIActor := openai.NewModel(openai.GPT4oMini, openai.APIKeyFromEnv()).Actor("You are a helpful assistant.")

	pipeline := lingograph.NewLoop(
		lingograph.NewChain(
			stdinActor.Pipeline(nil, false, 0),
			openAIActor.Pipeline(func(message lingograph.Message) {
				fmt.Println(message.Content)
			}, false, 3),
		),
		10,
	)

	pipeline.Execute(chat)
}
