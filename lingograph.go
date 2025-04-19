package lingograph

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vasilisp/lingograph/internal/util"
)

type Role uint8

const (
	User Role = iota
	Assistant
)

type actorID uint32

type Message struct {
	Role    Role
	Content string
	actor   actorID
}

type Chat interface {
	History() []Message
	write(message Message)
	trim()
}

type SliceChat struct {
	history []Message
}

func (c *SliceChat) History() []Message {
	return c.history
}

func (c *SliceChat) write(message Message) {
	c.history = append(c.history, message)
}

func (c *SliceChat) trim() {
	c.history = make([]Message, 0)
}

func NewSliceChat() Chat {
	return &SliceChat{history: make([]Message, 0)}
}

const userActorID actorID = 0

var lastActorID uint32 = 0

type Pipeline interface {
	Execute(chat Chat) error
	trims() bool
}

type staticPipeline struct {
	actorID actorID
	roleID  Role
	message string
	trim    bool
}

func (a *staticPipeline) Execute(chat Chat) error {
	if a.trim {
		chat.trim()
	}

	chat.write(Message{Role: a.roleID, Content: a.message})

	return nil
}

func (a *staticPipeline) trims() bool {
	return a.trim
}

func UserPrompt(message string, trim bool) Pipeline {
	return &staticPipeline{actorID: userActorID, roleID: User, message: message, trim: trim}
}

type Actor struct {
	actorID actorID
	roleID  Role
	fn      func(history []Message) ([]Message, error)
}

func NewActor(role Role, fn func(history []Message) (string, error)) Actor {
	util.Assert(fn != nil, "NewActor nil fn")

	fnWrapped := func(history []Message) ([]Message, error) {
		content, err := fn(history)
		if err != nil {
			return nil, err
		}

		// actorID will be set by the caller
		return []Message{{Role: role, Content: content}}, nil
	}

	return Actor{
		actorID: actorID(atomic.AddUint32(&lastActorID, 1)),
		roleID:  role,
		fn:      fnWrapped,
	}
}

func NewActorUnsafe(role Role, fn func(history []Message) ([]Message, error)) Actor {
	util.Assert(fn != nil, "NewActorUnsafe nil fn")

	return Actor{
		actorID: actorID(atomic.AddUint32(&lastActorID, 1)),
		roleID:  role,
		fn:      fn,
	}
}

type ActorPipeline struct {
	Actor
	echo       func(Message)
	trim       bool
	retryLimit int
}

func (a Actor) Pipeline(echo func(Message), trim bool, retryLimit int) Pipeline {
	return &ActorPipeline{
		Actor:      a,
		echo:       echo,
		trim:       trim,
		retryLimit: retryLimit,
	}
}

func (a *ActorPipeline) Execute(chat Chat) error {
	history := chat.History()

	var err error
	var newMessages []Message = nil

	for i := range max(1, a.retryLimit) {
		// FIXME: fn can theoretically modify history
		newMessages, err = a.fn(history)
		if err == nil {
			break
		}

		backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
		util.Log.Printf("error executing pipeline: %v", err)
		time.Sleep(backoff)
	}

	if a.trim {
		chat.trim()
	}

	for _, message := range newMessages {
		if a.echo != nil {
			a.echo(message)
		}

		message.actor = a.actorID

		chat.write(message)
	}

	return nil
}

func (a *ActorPipeline) trims() bool {
	return a.trim
}

type chain struct {
	pipelines []Pipeline
}

func (c chain) Execute(chat Chat) error {
	for _, pipeline := range c.pipelines {
		err := pipeline.Execute(chat)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c chain) trims() bool {
	for _, pipeline := range c.pipelines {
		if pipeline.trims() {
			return true
		}
	}

	return false
}

func Chain(pipelines ...Pipeline) Pipeline {
	return chain{pipelines: pipelines}
}

type chatSplitter struct {
	messages             []Message
	offsetUniqueMessages int
}

func split(chat Chat, nr int) []*chatSplitter {
	splitters := make([]*chatSplitter, nr)

	for i := range splitters {
		messages := make([]Message, len(chat.History()))
		copy(messages, chat.History())
		splitters[i] = &chatSplitter{
			messages:             messages,
			offsetUniqueMessages: len(messages),
		}
	}

	return splitters
}

func (c *chatSplitter) History() []Message {
	return c.messages
}

func (c *chatSplitter) write(message Message) {
	c.messages = append(c.messages, message)
}

func (c *chatSplitter) trim() {
	c.messages = make([]Message, 0)
	c.offsetUniqueMessages = 0
}

func (c *chatSplitter) uniqueMessages() []Message {
	return c.messages[c.offsetUniqueMessages:]
}

type parallel struct {
	pipelines []Pipeline
}

func Parallel(pipelines ...Pipeline) Pipeline {
	return parallel{pipelines: pipelines}
}

func (p parallel) trims() bool {
	for _, pipeline := range p.pipelines {
		if !pipeline.trims() {
			return false
		}
	}
	return true
}

func (p parallel) Execute(chat Chat) error {
	if len(p.pipelines) == 0 {
		return nil
	}

	splitters := split(chat, len(p.pipelines))

	wg := sync.WaitGroup{}
	wg.Add(len(p.pipelines))

	var mu sync.Mutex
	var errors []error

	fn := func(i int) {
		splitter := splitters[i]
		err := p.pipelines[i].Execute(splitter)
		if err != nil {
			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
			return
		}
		wg.Done()
	}

	for i := range p.pipelines {
		go fn(i)
	}

	wg.Wait()

	if len(errors) > 0 {
		for _, err := range errors {
			util.Log.Printf("error executing pipeline: %v", err)
		}

		return errors[0]
	}

	if p.trims() {
		chat.trim()
	}

	for i := range p.pipelines {
		splitter := splitters[i]
		for _, message := range splitter.uniqueMessages() {
			chat.write(message)
		}
	}

	return nil
}

type loop struct {
	pipeline Pipeline
	limit    int
}

func (l loop) Execute(chat Chat) error {
	for i := 0; l.limit < 0 || i < l.limit; i++ {
		err := l.pipeline.Execute(chat)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l loop) trims() bool {
	return l.pipeline.trims()
}

func Loop(pipeline Pipeline, limit int) Pipeline {
	return loop{pipeline: pipeline, limit: limit}
}
