package store

import (
	"sync"
	"sync/atomic"
)

// global variable for generating unique IDs: vars may be used for different
// stores, so types have to be the same globally
var nextID int64

type Store struct {
	vars *sync.Map
}

func NewStore() Store {
	return Store{vars: &sync.Map{}}
}

type Var[T any] struct {
	id int64
}

func FreshVar[T any]() Var[T] {
	return Var[T]{id: atomic.AddInt64(&nextID, 1)}
}

func Get[T any](r Store, v Var[T]) (T, bool) {
	var valT T

	val, found := r.vars.Load(v.id)
	if !found {
		return valT, false
	}

	valT, ok := val.(T)
	if !ok {
		panic("Get: store type assertion failed")
	}

	return valT, true
}

func Set[T any](r Store, v Var[T], val T) {
	r.vars.Store(v.id, val)
}
