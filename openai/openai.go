package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/internal/util"
	"github.com/vasilisp/lingograph/pkg/slicev"
	"github.com/vasilisp/lingograph/store"
)

type ChatModel uint8

const (
	GPT4o ChatModel = iota
	GPT4oMini
	GPT41
	GPT41Mini
	GPT41Nano
)

func (m ChatModel) ToOpenAI() openai.ChatModel {
	switch m {
	case GPT4o:
		return openai.ChatModelGPT4o
	case GPT4oMini:
		return openai.ChatModelGPT4oMini
	case GPT41:
		return "gpt-4.1"
	case GPT41Mini:
		return "gpt-4.1-mini"
	case GPT41Nano:
		return "gpt-4.1-nano"
	default:
		util.Assert(false, "invalid chat model")
	}

	// dummy return
	return openai.ChatModelGPT4o
}

type Client struct {
	client *openai.Client
}

func APIKeyFromEnv() string {
	key, exists := os.LookupEnv("OPENAI_API_KEY")
	if !exists {
		log.Fatal("OPENAI_API_KEY environment variable is not set")
	}
	return key
}

func NewClient(apiKey string) Client {
	if apiKey == "" {
		log.Fatal("apiKey is empty")
	}
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return Client{client: &client}
}

type Function struct {
	name string
	def  openai.FunctionDefinitionParam
	fn   func(string, store.Store) ([]lingograph.Message, error)
}

func call(functions map[string]Function, toolCall openai.ChatCompletionMessageToolCall, r store.Store) ([]lingograph.Message, error) {
	fn, ok := functions[toolCall.Function.Name]
	if !ok {
		return nil, fmt.Errorf("function not found")
	}

	messages, err := fn.fn(toolCall.Function.Arguments, r)
	if err != nil {
		return nil, err
	}

	messagesWithMetadata := make([]lingograph.Message, 0, len(messages))
	for i, msg := range messages {
		if msg.Role == lingograph.Function {
			// for multiple responses per tool call: each needs a unique call ID
			msg.ModelMetadata = functionCallID{ID: fmt.Sprintf("%s_%d", toolCall.ID, i)}
		}
		messagesWithMetadata = append(messagesWithMetadata, msg)
	}

	return messagesWithMetadata, nil
}

type functionCallMetadata struct {
	param       openai.ChatCompletionMessageToolCallParam
	nrResponses int
}

type functionCallID struct {
	ID string
}

func (client Client) ask(modelID ChatModel, systemPrompt string, history slicev.RO[lingograph.Message], functions map[string]Function, r store.Store, temperature *float64) ([]lingograph.Message, error) {
	length := history.Len()
	if systemPrompt != "" {
		length++
	}
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, length)

	if systemPrompt != "" {
		messages = append(messages, openai.SystemMessage(systemPrompt))
	}

	// FIXME: verify that the function IDs in 'assistant' messages match the
	// ones in 'function' messages. If the history has been trimmed this may not
	// be the case. Strip off function info and fall back to user messages if
	// necessary.

	it := history.Iterator()
	for it.Next() {
		msg := it.Value()
		switch msg.Role {
		case lingograph.Assistant:
			toolCalls, ok := msg.ModelMetadata.([]functionCallMetadata)
			if !ok {
				messages = append(messages, openai.AssistantMessage(msg.Content))
			} else {
				toolCallsExpanded := make([]openai.ChatCompletionMessageToolCallParam, 0, len(toolCalls))

				for _, toolCall := range toolCalls {
					for i := range toolCall.nrResponses {
						param := toolCall.param
						// has to match the expansion in call()
						param.ID = fmt.Sprintf("%s_%d", toolCall.param.ID, i)
						toolCallsExpanded = append(toolCallsExpanded, param)
					}
				}

				message := openai.ChatCompletionAssistantMessageParam{
					Content: openai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: param.NewOpt(msg.Content),
					},
				}

				if len(toolCallsExpanded) > 0 {
					message.ToolCalls = toolCallsExpanded
				}

				messages = append(messages, openai.ChatCompletionMessageParamUnion{
					OfAssistant: &message,
				})
			}
		case lingograph.Function:
			util.Assert(msg.ModelMetadata != nil, "ask nil ModelMetadata")
			toolCallID := msg.ModelMetadata.(functionCallID)
			messages = append(messages, openai.ToolMessage(msg.Content, toolCallID.ID))
		default:
			messages = append(messages, openai.UserMessage(msg.Content))
		}
	}

	toolParams := make([]openai.ChatCompletionToolParam, 0)

	for _, fn := range functions {
		toolParams = append(toolParams, openai.ChatCompletionToolParam{
			Type:     "function",
			Function: fn.def,
		})
	}

	var temperatureOpt param.Opt[float64]
	if temperature != nil {
		temperatureOpt = param.NewOpt(*temperature)
	} else {
		temperatureOpt = param.Null[float64]()
	}

	response, err := client.client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model:       modelID.ToOpenAI(),
		Messages:    messages,
		Tools:       toolParams,
		Temperature: temperatureOpt,
	})
	if err != nil {
		return nil, err
	}

	functionCalls := make([]functionCallMetadata, 0)
	responseMessages := make([]lingograph.Message, 0, len(response.Choices))

	for _, choice := range response.Choices {
		choiceMessages := make([]lingograph.Message, 0)

		for _, toolCall := range choice.Message.ToolCalls {
			result, err := call(functions, toolCall, r)
			if err != nil {
				return nil, fmt.Errorf("error calling function %s: %w", toolCall.Function.Name, err)
			}

			functionCalls = append(functionCalls, functionCallMetadata{
				param: openai.ChatCompletionMessageToolCallParam{
					ID:   toolCall.ID,
					Type: toolCall.Type,
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Name:      toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					},
				},
				nrResponses: len(result),
			})

			choiceMessages = append(choiceMessages, result...)
		}

		responseMessages = append(responseMessages, lingograph.Message{Role: lingograph.Assistant, Content: choice.Message.Content, ModelMetadata: functionCalls})
		responseMessages = append(responseMessages, choiceMessages...)
	}

	util.Assert(len(response.Choices) > 0, "no choices")
	return responseMessages, nil
}

type Actor struct {
	lingoActor lingograph.Actor
	functions  map[string]Function
}

func NewActor(client Client, chatModel ChatModel, systemPrompt string, temperature *float64) Actor {
	functions := make(map[string]Function)

	actor := Actor{functions: functions}

	actor.lingoActor = lingograph.NewActorUnsafe(
		lingograph.Assistant,
		func(history slicev.RO[lingograph.Message], r store.Store) ([]lingograph.Message, error) {
			return client.ask(chatModel, systemPrompt, history, actor.functions, r, temperature)
		},
	)

	return actor
}

func (a Actor) addFunction(fn Function) {
	a.functions[fn.name] = fn
}

func InlineRefs(s *jsonschema.Schema) (*jsonschema.Schema, error) {
	if s.Ref != "" {
		if s.Definitions == nil {
			return nil, fmt.Errorf("schema has $ref but no definitions")
		}

		refKey, err := extractDefKey(s.Ref)
		if err != nil {
			return nil, err
		}

		def, ok := s.Definitions[refKey]
		if !ok {
			return nil, fmt.Errorf("ref %q not found in definitions", refKey)
		}

		return inlineSchema(def, s.Definitions)
	}

	return inlineSchema(s, s.Definitions)
}

func inlineSchema(s *jsonschema.Schema, defs map[string]*jsonschema.Schema) (*jsonschema.Schema, error) {
	if s == nil {
		return nil, nil
	}

	// Deep copy first
	copy := *s

	copy.Definitions = nil // Remove defs to match OpenAI expectations

	// Inline all properties
	if copy.Properties != nil {
		copy.Properties = orderedmap.New[string, *jsonschema.Schema]()
		for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
			inlinedProp, err := inlineSchema(resolveRef(pair.Value, defs), defs)
			if err != nil {
				return nil, err
			}
			copy.Properties.Set(pair.Key, inlinedProp)
		}
	}

	// Inline items (for arrays)
	if s.Items != nil {
		inlinedItem, err := inlineSchema(resolveRef(s.Items, defs), defs)
		if err != nil {
			return nil, err
		}
		copy.Items = inlinedItem
	}

	return &copy, nil
}

func resolveRef(s *jsonschema.Schema, defs map[string]*jsonschema.Schema) *jsonschema.Schema {
	if s == nil || s.Ref == "" {
		return s
	}

	refKey, err := extractDefKey(s.Ref)
	if err != nil {
		return s
	}

	if def, ok := defs[refKey]; ok {
		return def
	}

	return s
}

func extractDefKey(ref string) (string, error) {
	const prefix = "#/$defs/"
	if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
		return "", fmt.Errorf("unsupported ref format: %s", ref)
	}
	return ref[len(prefix):], nil
}

func ToOpenAISchema(s *jsonschema.Schema) (map[string]any, error) {
	if s == nil {
		return nil, errors.New("schema is nil")
	}

	out := map[string]any{}
	if s.Type != "" {
		out["type"] = s.Type
	}

	if s.Description != "" {
		out["description"] = s.Description
	}

	if len(s.Required) > 0 {
		out["required"] = s.Required
	}

	if s.AdditionalProperties == jsonschema.FalseSchema {
		out["additionalProperties"] = false
	}

	// Handle properties
	if s.Properties != nil && s.Properties.Len() > 0 {
		props := map[string]any{}
		for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
			name := pair.Key
			prop := pair.Value
			inlined, err := ToOpenAISchema(prop)
			if err != nil {
				return nil, err
			}
			props[name] = inlined
		}
		out["properties"] = props
	}

	// Handle array items
	if s.Type == "array" && s.Items != nil {
		items, err := ToOpenAISchema(s.Items)
		if err != nil {
			return nil, err
		}
		out["items"] = items
	}

	// Optionally handle enums, formats, etc.
	if len(s.Enum) > 0 {
		out["enum"] = s.Enum
	}
	if s.Format != "" {
		out["format"] = s.Format
	}

	return out, nil
}

func AddFunctionUnsafe[I any](a Actor, name string, description string, fn func(I, store.Store) ([]string, error)) {
	var zero I
	reflector := &jsonschema.Reflector{}
	schema := reflector.Reflect(&zero)

	inlinedSchema, err := InlineRefs(schema)
	if err != nil {
		log.Fatalf("cannot inline schema: %s", err)
	}

	openAISchema, err := ToOpenAISchema(inlinedSchema)
	if err != nil {
		log.Fatalf("cannot convert schema to OpenAI schema: %s", err)
	}

	fnWrapped := func(input string, r store.Store) ([]lingograph.Message, error) {
		var i I
		err := json.Unmarshal([]byte(input), &i)
		if err != nil {
			return nil, err
		}

		results, err := fn(i, r)
		if err != nil {
			return nil, err
		}

		messages := make([]lingograph.Message, 0, len(results))
		for _, result := range results {
			messages = append(messages, lingograph.Message{Role: lingograph.Function, Content: result})
		}

		return messages, nil
	}

	a.addFunction(Function{
		name: name,
		def: openai.FunctionDefinitionParam{
			Name:        name,
			Description: param.NewOpt(description),
			Parameters:  openAISchema,
		},
		fn: fnWrapped,
	})
}

func AddFunction[I any, O any](a Actor, name string, description string, fn func(I, store.Store) (O, error)) {
	AddFunctionUnsafe(a, name, description,
		func(i I, r store.Store) ([]string, error) {
			o, err := fn(i, r)
			if err != nil {
				return nil, err
			}

			json, err := json.Marshal(o)
			if err != nil {
				return nil, err
			}

			return []string{string(json)}, nil
		})
}

func (a Actor) LingographActor() lingograph.Actor {
	return a.lingoActor
}

func (a Actor) Pipeline(echo func(lingograph.Message), trim bool, retryLimit int) lingograph.Pipeline {
	return a.lingoActor.Pipeline(echo, trim, retryLimit)
}
