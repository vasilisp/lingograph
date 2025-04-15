package lingograph

import (
	"sync"
	"sync/atomic"
)

type Role uint8

const (
	System Role = iota
	User
	Assistant
)

type Message struct {
	Role    Role
	Content string
	Actor   Actor
}

type Chat interface {
	History() []Message
	write(message Message)
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

func NewSliceChat() Chat {
	return &SliceChat{history: make([]Message, 0)}
}

type chainLink func(history []Message) (Message, error)

type actorID uint64

const (
	systemActorID actorID = 0
	userActorID   actorID = 1
)

var nextActorID uint64 = 2

type Actor interface {
	id() actorID
	role() Role
	generate() chainLink
}

type staticActor struct {
	actorID actorID
	roleID  Role
	message string
}

func (a staticActor) id() actorID {
	return a.actorID
}

func (a staticActor) role() Role {
	return a.roleID
}

func (a staticActor) generate() chainLink {
	return func(history []Message) (Message, error) {
		return Message{Role: a.roleID, Content: a.message}, nil
	}
}

func newStaticActor(actorID actorID, role Role, message string) Actor {
	return staticActor{actorID: actorID, roleID: role, message: message}
}

func NewSystemPrompt(message string) Actor {
	return newStaticActor(systemActorID, System, message)
}

func NewUserPrompt(message string) Actor {
	return newStaticActor(userActorID, User, message)
}

type programmaticActor struct {
	actorID actorID
	roleID  Role
	fn      func(history []Message) (string, error)
}

func (a programmaticActor) id() actorID {
	return a.actorID
}

func (a programmaticActor) role() Role {
	return a.roleID
}

func (a programmaticActor) generate() chainLink {
	return func(history []Message) (Message, error) {
		content, err := a.fn(history)
		if err != nil {
			return Message{}, err
		}
		return Message{Role: a.roleID, Content: content}, nil
	}
}

func NewProgrammaticActor(role Role, fn func(history []Message) (string, error)) Actor {
	id := atomic.AddUint64(&nextActorID, 1)
	return programmaticActor{
		actorID: actorID(id),
		roleID:  role,
		fn:      fn,
	}
}

type Pipeline interface {
	Execute(chat Chat)
}

type chain struct {
	links []chainLink
}

func (c chain) Execute(chat Chat) {
	for _, link := range c.links {
		message, err := link(chat.History())
		if err != nil {
			panic(err)
		}
		chat.write(message)
	}
}

func NewChain(actor1, actor2 Actor, actors ...Actor) Pipeline {
	links := make([]chainLink, 0, len(actors)+2)
	links = append(links, actor1.generate(), actor2.generate())
	for _, actor := range actors {
		links = append(links, actor.generate())
	}
	return chain{links: links}
}

type chatSplitter struct {
	oldMessages []Message
	newMessages []Message
}

func split(chat Chat) chatSplitter {
	return chatSplitter{
		oldMessages: chat.History(),
		newMessages: make([]Message, 0),
	}
}

func (c chatSplitter) history() []Message {
	return c.oldMessages
}

func (c chatSplitter) write(message Message) {
	c.newMessages = append(c.newMessages, message)
}

type parallel struct {
	links []chainLink
}

func NewParallel(link1, link2 chainLink, links ...chainLink) Pipeline {
	return parallel{links: append([]chainLink{link1, link2}, links...)}
}

func (p parallel) Execute(chat Chat) {
	splitters := make([]chatSplitter, len(p.links))
	for i := range p.links {
		splitters[i] = split(chat)
	}

	wg := sync.WaitGroup{}
	wg.Add(len(p.links))

	fn := func(i int) {
		splitter := splitters[i]
		message, err := p.links[i](splitter.history())
		if err != nil {
			panic(err)
		}
		splitter.write(message)
		wg.Done()
	}

	for i := range p.links {
		go fn(i)
	}

	wg.Wait()

	for _, splitter := range splitters {
		for _, message := range splitter.newMessages {
			chat.write(message)
		}
	}
}
