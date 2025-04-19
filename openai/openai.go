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
	modelID ChatModel
}

func APIKeyFromEnv() string {
	return os.Getenv("OPENAI_API_KEY")
}

func NewModel(modelID ChatModel, apiKey string) Model {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return Model{client: &client, modelID: modelID}
}

type Function struct {
	name string
	def  openai.FunctionDefinitionParam
	fn   func(string) ([]lingograph.Message, error)
}

func call(functions map[string]Function, name, args string) ([]lingograph.Message, error) {
	fn, ok := functions[name]
	if !ok {
		return nil, fmt.Errorf("function not found")
	}

	return fn.fn(args)
}

func (m *Model) ask(systemPrompt string, history []lingograph.Message, functions map[string]Function) ([]lingograph.Message, error) {
	length := len(history)
	if systemPrompt != "" {
		length++
	}
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, length)

	if systemPrompt != "" {
		messages = append(messages, openai.SystemMessage(systemPrompt))
	}

	for _, msg := range history {
		switch msg.Role {
		case lingograph.Assistant:
			messages = append(messages, openai.AssistantMessage(msg.Content))
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

	response, err := m.client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Model:    m.modelID.ToOpenAI(),
		Messages: messages,
		Tools:    toolParams,
	})
	if err != nil {
		return nil, err
	}

	newMessages := make([]lingograph.Message, 0, len(response.Choices))

	for _, choice := range response.Choices {
		msg := choice.Message
		if msg.ToolCalls != nil && len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				functionName := toolCall.Function.Name
				arguments := toolCall.Function.Arguments
				result, err := call(functions, functionName, arguments)
				if err != nil {
					return nil, fmt.Errorf("error calling function %s: %w", functionName, err)
				}
				// FIXME: these should be tool messages (but a bit complicated to implement)
				newMessages = append(newMessages, result...)
			}
		} else {
			newMessages = append(newMessages, lingograph.Message{Role: lingograph.Assistant, Content: msg.Content})
		}
	}

	util.Assert(len(response.Choices) > 0, "no choices")
	return newMessages, nil
}

type Actor struct {
	lingoActor lingograph.Actor
	functions  map[string]Function
}

func NewActor(model Model, systemPrompt string) Actor {
	functions := make(map[string]Function)

	actor := Actor{functions: functions}

	actor.lingoActor = lingograph.NewActorUnsafe(
		lingograph.Assistant,
		func(history []lingograph.Message) ([]lingograph.Message, error) {
			return model.ask(systemPrompt, history, actor.functions)
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

func AddFunctionUnsafe[I any](a Actor, name string, description string, fn func(I) ([]lingograph.Message, error)) {
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

	fnWrapped := func(input string) ([]lingograph.Message, error) {
		var i I
		err := json.Unmarshal([]byte(input), &i)
		if err != nil {
			return nil, err
		}

		return fn(i)
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

func AddFunction[I any, O any](a Actor, name string, description string, fn func(I) (O, error)) {
	AddFunctionUnsafe(a, name, description,
		func(i I) ([]lingograph.Message, error) {
			o, err := fn(i)
			if err != nil {
				return nil, err
			}

			json, err := json.Marshal(o)
			if err != nil {
				return nil, err
			}

			return []lingograph.Message{{Role: lingograph.Assistant, Content: string(json)}}, nil
		})
}

func (a Actor) LingographActor() lingograph.Actor {
	return a.lingoActor
}

func (a Actor) Pipeline(echo func(lingograph.Message), trim bool, retryLimit int) lingograph.Pipeline {
	return a.lingoActor.Pipeline(echo, trim, retryLimit)
}
