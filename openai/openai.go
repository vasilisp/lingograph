package openai

import (
	"context"
	"math"
	"os"
	"time"

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

func (m *Model) askWithRetry(history []lingograph.Message, limit int) (string, error) {
	var err error

	for i := range limit {
		result, err := m.ask(history)

		if err == nil {
			return result, nil
		}

		if i < limit-1 {
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			util.Log.Printf("OpenAI error, will retry in %v: %v", backoff, err)
			time.Sleep(backoff)
		}
	}

	return "", err
}

func NewOpenAIActor(model Model, retryLimit int) lingograph.Pipeline {
	return lingograph.NewProgrammaticActor(
		lingograph.Assistant,
		func(history []lingograph.Message) (string, error) {
			result, err := model.askWithRetry(history, 3)
			if err != nil {
				return "", err
			}
			return result, nil
		},
	)
}
