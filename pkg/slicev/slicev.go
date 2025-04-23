package slicev

// RO provides read-only access to a slice of type T
type RO[T any] struct {
	slice []T
}

// NewRO creates a new read-only wrapper around a slice
func NewRO[T any](slice []T) RO[T] {
	return RO[T]{slice: slice}
}

// Len returns the number of elements
func (r RO[T]) Len() int {
	return len(r.slice)
}

// At returns the element at index i
func (r RO[T]) At(i int) T {
	return r.slice[i]
}

func (r RO[T]) CopyTo(dst []T) int {
	return copy(dst, r.slice)
}

// Iterator returns an iterator over the elements
func (r RO[T]) Iterator() *Iterator[T] {
	return &Iterator[T]{
		slice:   r.slice,
		current: 0,
	}
}

// Iterator provides iteration over elements
type Iterator[T any] struct {
	slice   []T
	current int
}

// Next advances to the next element and returns true if there is one
func (it *Iterator[T]) Next() bool {
	if it.current >= len(it.slice) {
		return false
	}
	it.current++
	return true
}

// Value returns the current element
func (it *Iterator[T]) Value() T {
	return it.slice[it.current-1]
}
