package main

import (
	"fmt"

	"github.com/vasilisp/lingograph"
	"github.com/vasilisp/lingograph/openai"
)

type Person struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email"`
}

type WrappedString struct {
	Value string `json:"value"`
}

const systemPrompt = `
You are the database system of a company. You receive queries in natural language.

Your job is to translate the queries into function calls.

- add a person to the database providing the name, age and email
- search for a person by name`

func main() {
	chat := lingograph.NewSliceChat()

	openAIModel := openai.NewModel(openai.GPT4oMini, openai.APIKeyFromEnv())
	openAIActor := openai.NewActor(openAIModel, systemPrompt)

	db := make(map[string]Person)

	openai.AddFunction(openAIActor, "add_person", "Add a person to the database", func(person Person) (string, error) {
		db[person.Name] = person
		fmt.Println("added", person.Name, "to the database")
		return fmt.Sprintf("added %s to the database", person.Name), nil
	})

	openai.AddFunction(openAIActor, "search_person", "Search for a person by name", func(name WrappedString) (string, error) {
		person, ok := db[name.Value]
		if ok {
			fmt.Printf("found %s in the database. They are %d years old and their email is %s\n", person.Name, person.Age, person.Email)
			return person.Name, nil
		}

		return "", fmt.Errorf("person not found")
	})

	chain := lingograph.Chain(
		lingograph.UserPrompt("Add John Doe to the database. He is 40 years old and his email is john.doe@example.com.", false),
		openAIActor.Pipeline(nil, false, 3),
		lingograph.UserPrompt("Search for John Doe in the database.", false),
		openAIActor.Pipeline(nil, false, 3),
	)

	chain.Execute(chat)
}
