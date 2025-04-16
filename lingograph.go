package lingograph

import (
	"sync"
	"sync/atomic"

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

var nextActorID uint64 = 1

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

func NewUserPrompt(message string, trim bool) Pipeline {
	return &staticPipeline{actorID: userActorID, roleID: User, message: message, trim: trim}
}

type Actor struct {
	actorID actorID
	roleID  Role
	fn      func(history []Message) (string, error)
}

func NewActor(role Role, fn func(history []Message) (string, error)) Actor {
	util.Assert(fn != nil, "NewProgrammaticActor nil fn")

	id := atomic.AddUint64(&nextActorID, 1)

	return Actor{
		actorID: actorID(id),
		roleID:  role,
		fn:      fn,
	}
}

type ActorPipeline struct {
	Actor
	echo func(Message)
	trim bool
}

func (a Actor) Pipeline(echo func(Message), trim bool) Pipeline {
	return &ActorPipeline{
		Actor: a,
		echo:  echo,
		trim:  trim,
	}
}

func (a *ActorPipeline) Execute(chat Chat) error {
	content, err := a.fn(chat.History())
	if err != nil {
		return err
	}

	if a.trim {
		chat.trim()
	}

	message := Message{Role: a.roleID, Content: content}

	if a.echo != nil {
		a.echo(message)
	}

	chat.write(message)

	return nil
}

func (a *ActorPipeline) trims() bool {
	return a.trim
}

type chain struct {
	links []Pipeline
}

func (c chain) Execute(chat Chat) error {
	for _, link := range c.links {
		err := link.Execute(chat)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c chain) trims() bool {
	for _, link := range c.links {
		if link.trims() {
			return true
		}
	}

	return false
}

func NewChain(pipeline1, pipeline2 Pipeline, pipelines ...Pipeline) Pipeline {
	links := make([]Pipeline, 0, len(pipelines)+2)

	links = append(links, pipeline1, pipeline2)
	for _, pipeline := range pipelines {
		links = append(links, pipeline)
	}

	return chain{links: links}
}

type chatSplitter struct {
	oldMessages []Message
	newMessages []Message
}

func split(chat Chat, nr int) []*chatSplitter {
	splitters := make([]*chatSplitter, nr)
	oldMessages := make([]Message, len(chat.History()))
	copy(oldMessages, chat.History())

	for i := range splitters {
		splitters[i] = &chatSplitter{
			oldMessages: oldMessages,
			newMessages: make([]Message, 0),
		}
	}

	return splitters
}

func (c *chatSplitter) History() []Message {
	return c.oldMessages
}

func (c *chatSplitter) write(message Message) {
	c.newMessages = append(c.newMessages, message)
}

func (c *chatSplitter) trim() {
	c.oldMessages = make([]Message, 0)
	c.newMessages = make([]Message, 0)
}

type parallel struct {
	links []Pipeline
}

func NewParallel(pipeline1, pipeline2 Pipeline, pipelines ...Pipeline) Pipeline {
	links := make([]Pipeline, 0, len(pipelines)+2)

	links = append(links, pipeline1, pipeline2)
	for _, pipeline := range pipelines {
		links = append(links, pipeline)
	}

	return parallel{links: links}
}

func (p parallel) trims() bool {
	for _, link := range p.links {
		if !link.trims() {
			return false
		}
	}
	return true
}

func (p parallel) Execute(chat Chat) error {
	splitters := split(chat, len(p.links))

	wg := sync.WaitGroup{}
	wg.Add(len(p.links))

	var mu sync.Mutex
	var errors []error

	fn := func(i int) {
		splitter := splitters[i]
		err := p.links[i].Execute(splitter)
		if err != nil {
			mu.Lock()
			errors = append(errors, err)
			mu.Unlock()
			return
		}
		wg.Done()
	}

	for i := range p.links {
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

	for link := range p.links {
		splitter := splitters[link]
		for _, message := range splitter.newMessages {
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

func NewLoop(pipeline Pipeline, limit int) Pipeline {
	return loop{pipeline: pipeline, limit: limit}
}
