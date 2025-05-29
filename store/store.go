package store

import (
	"sync"
	"sync/atomic"
)

// global variable for generating unique IDs: vars may be used for different
// stores, so types have to be the same globally
var nextID int64

// Store is a heterogeneous key-value map.
type Store struct {
	vars *sync.Map
}

// NewStore creates a new Store.
func NewStore() Store {
	return Store{vars: &sync.Map{}}
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

// Set sets the value of a Var in the Store.
func Set[T any](r Store, v Var[T], val T) {
	r.vars.Store(v.id, val)
}

// StoreRO is a read-only view of a Store.
type StoreRO struct {
	store Store
}

// RO returns a read-only view of the Store.
func (r Store) RO() StoreRO {
	return StoreRO{store: r}
}

// GetRO retrieves the value of a Var from the read-only Store. The second
// return value indicates whether the variable was bound.
func GetRO[T any](r StoreRO, v Var[T]) (T, bool) {
	return Get(r.store, v)
}
