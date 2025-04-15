package openai

import (
	"context"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/internal/util"
)

type ChatModel uint8

const (
	GPT4o ChatModel = iota
	GPT4oMini
)

func (m ChatModel) ToOpenAI() openai.ChatModel {
	switch m {
	case GPT4o:
		return openai.ChatModelGPT4o
	case GPT4oMini:
		return openai.ChatModelGPT4oMini
	default:
		util.Assert(false, "invalid chat model")
	}

	// dummy return
	return openai.ChatModelGPT4o
}

type Model struct {
	client  *openai.Client
	modelID openai.ChatModel
}

func APIKeyFromEnv() string {
	return os.Getenv("OPENAI_API_KEY")
}

func NewModel(modelID ChatModel, apiKey string) Model {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return Model{client: &client, modelID: modelID.ToOpenAI()}
}

func (m *Model) ask(history []lingograph.Message) (string, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, len(history))
	for i, msg := range history {
		switch msg.Role {
		case lingograph.System:
			messages[i] = openai.SystemMessage(msg.Content)
		case lingograph.Assistant:
			messages[i] = openai.AssistantMessage(msg.Content)
		default:
			messages[i] = openai.UserMessage(msg.Content)
		}
	}

	response, err := m.client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model:    m.modelID,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}

	util.Assert(len(response.Choices) > 0, "no choices")
	return response.Choices[0].Message.Content, nil
}

func NewOpenAIActor(model Model) lingograph.Actor {
	return lingograph.NewProgrammaticActor(
		lingograph.Assistant,
		func(history []lingograph.Message) (string, error) {
			result, err := model.ask(history)
			if err != nil {
				return "", err
			}
			return result, nil
		},
	)
}
