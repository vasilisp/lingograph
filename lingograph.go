package lingograph

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vasilisp/lingograph/internal/util"
	"github.com/vasilisp/lingograph/pkg/slicev"
	"github.com/vasilisp/lingograph/store"
)

const maxHistoryLength = 1000

type Role uint8

const (
	User Role = iota
	Assistant
	Function
)

func (r Role) String() string {
	switch r {
	case User:
		return "user"
	case Assistant:
		return "assistant"
	case Function:
		return "function"
	}
	return "unknown"
}

type actorID uint32

type Message struct {
	Role          Role
	Content       string
	actor         actorID
	ModelMetadata any
}

type Chat interface {
	History() slicev.RO[Message]
	write(message Message)
	trim()
	Store() store.Store
}

type chat struct {
	history      []Message
	store        store.Store
	offsetUnique int
}

func (c *chat) History() slicev.RO[Message] {
	return slicev.NewRO(c.history)
}

func (c *chat) write(message Message) {
	if len(c.history) >= maxHistoryLength {
		keep := maxHistoryLength / 2
		if keep < c.offsetUnique {
			c.offsetUnique = 0
		} else {
			c.offsetUnique -= len(c.history) - keep
		}
		c.history = c.history[len(c.history)-keep:]
	}
	c.history = append(c.history, message)
}

func (c *chat) trim() {
	c.history = make([]Message, 0)
	c.offsetUnique = 0
}

func (c *chat) Store() store.Store {
	return c.store
}

func (c *chat) uniqueMessages() []Message {
	return c.history[c.offsetUnique:]
}

func NewChat() Chat {
	return &chat{history: make([]Message, 0), store: store.NewStore(), offsetUnique: 0}
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
	fn      func(slicev.RO[Message], store.Store) ([]Message, error)
}

func NewActor(role Role, fn func(slicev.RO[Message], store.Store) (string, error)) Actor {
	util.Assert(fn != nil, "NewActor nil fn")

	fnWrapped := func(history slicev.RO[Message], r store.Store) ([]Message, error) {
		content, err := fn(history, r)
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

func NewActorUnsafe(role Role, fn func(slicev.RO[Message], store.Store) ([]Message, error)) Actor {
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

	retryLimit := max(1, a.retryLimit)

	for i := range retryLimit {
		newMessages, err = a.fn(history, chat.Store())
		if err == nil {
			break
		}

		util.Log.Printf("error executing pipeline: %v", err)

		if i < retryLimit-1 {
			backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
			time.Sleep(backoff)
		}
	}
	if err != nil {
		return err
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

func split(c Chat, nr int) []*chat {
	splitters := make([]*chat, nr)

	for i := range splitters {
		history := c.History()
		messages := make([]Message, history.Len())
		history.CopyTo(messages)

		splitters[i] = &chat{
			history:      messages,
			offsetUnique: len(messages),
			store:        c.Store(),
		}
	}

	return splitters
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

type Condition func(store.StoreRO) bool

type while struct {
	condition Condition
	pipeline  Pipeline
}

func (w while) Execute(chat Chat) error {
	for w.condition(chat.Store().RO()) {
		err := w.pipeline.Execute(chat)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w while) trims() bool {
	return w.pipeline.trims()
}

func While(condition Condition, pipeline Pipeline) Pipeline {
	return while{pipeline: pipeline, condition: condition}
}

type ifPipeline struct {
	condition Condition
	left      Pipeline
	right     Pipeline
}

func (p ifPipeline) Execute(chat Chat) error {
	if p.condition(chat.Store().RO()) {
		return p.left.Execute(chat)
	}
	return p.right.Execute(chat)
}

func (p ifPipeline) trims() bool {
	// CHECKME: grey area
	return p.left.trims() && p.right.trims()
}

func If(condition Condition, left Pipeline, right Pipeline) Pipeline {
	return ifPipeline{condition: condition, left: left, right: right}
}
