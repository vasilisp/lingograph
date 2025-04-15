package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/openai"
)

func StdinActor() lingograph.Pipeline {
	return lingograph.NewProgrammaticActor(lingograph.User, func(history []lingograph.Message) (string, error) {
		reader := bufio.NewReader(os.Stdin)
		text, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(text), nil
	}, nil)
}

func main() {
	chat := lingograph.NewSliceChat()

	gptModel := openai.NewModel(openai.GPT4oMini, openai.APIKeyFromEnv())
	echo := func(message lingograph.Message) {
		fmt.Println(message.Content)
	}
	llm := openai.NewOpenAIActor(gptModel, echo, 3)

	pipeline := lingograph.NewChain(
		lingograph.NewSystemPrompt("You are a helpful assistant."),
		lingograph.NewLoop(
			lingograph.NewChain(
				StdinActor(),
				llm,
			),
			10,
		),
	)

	pipeline.Execute(chat)
}
