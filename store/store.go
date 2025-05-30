package store

import (
	"sync"
	"sync/atomic"
)

// global variable for generating unique IDs: vars may be used for different
// stores, so types have to be the same globally
var nextID int64

// Store is an interface that defines the operations for a heterogeneous key-value map.
type Store interface {
	// RO returns a read-only view of the Store.
	RO() StoreRO
	vars() *sync.Map
}

// store is a heterogeneous key-value map.
type store struct {
	varsMap *sync.Map
}

func (s *store) vars() *sync.Map {
	return s.varsMap
}

// NewStore creates a new Store.
func NewStore() Store {
	return &store{varsMap: &sync.Map{}}
}

// Var is a unique identifier for a variable in the Store.
type Var[T any] struct {
	id int64
}

// FreshVar creates a new Var with a unique ID.
func FreshVar[T any]() Var[T] {
	return Var[T]{id: atomic.AddInt64(&nextID, 1)}
}

// Get retrieves the value of a Var from the Store. The second return value
// indicates whether the variable was found.
func Get[T any](r Store, v Var[T]) (T, bool) {
	var valT T

	val, found := r.vars().Load(v.id)
	if !found {
		return valT, false
	}

	valT, ok := val.(T)
	if !ok {
		panic("Get: store type assertion failed")
	}

	return valT, true
}

// Set sets the value of a Var in the Store.
func Set[T any](r Store, v Var[T], val T) {
	r.vars().Store(v.id, val)
}

// StoreRO is a read-only view of a Store.
type StoreRO interface {
	store() Store
}

type storeRO struct {
	r Store
}

func (s *storeRO) store() Store {
	return s.r
}

func (r *store) RO() StoreRO {
	return &storeRO{r: r}
}

// GetRO retrieves the value of a Var from the read-only Store. The second
// return value indicates whether the variable was bound.
func GetRO[T any](r StoreRO, v Var[T]) (T, bool) {
	return Get(r.store(), v)
}
