package slicev

// RO provides read-only access to a slice of type T
type RO[T any] interface {
	// Len returns the number of elements in the slice
	Len() int
	// At returns the element at index i
	At(i int) T
	// CopyTo copies elements to the destination slice
	CopyTo(dst []T) int
	// Iterator returns an iterator over the elements
	Iterator() Iterator[T]
	seal()
}

type ro[T any] struct {
	slice []T
}

// NewRO creates a new read-only wrapper around a slice
func NewRO[T any](slice []T) RO[T] {
	return &ro[T]{slice: slice}
}

func (r *ro[T]) Len() int {
	return len(r.slice)
}

func (r *ro[T]) At(i int) T {
	return r.slice[i]
}

func (r *ro[T]) CopyTo(dst []T) int {
	return copy(dst, r.slice)
}

func (r *ro[T]) Iterator() Iterator[T] {
	return &iterator[T]{
		slice:   r.slice,
		current: 0,
	}
}

func (r *ro[T]) seal() {}

// Iterator provides iteration over elements
type Iterator[T any] interface {
	// Next advances to the next element and returns true if there is one
	Next() bool
	// Value returns the current element
	Value() T
	seal()
}

type iterator[T any] struct {
	slice   []T
	current int
}

func (it *iterator[T]) Next() bool {
	if it.current >= len(it.slice) {
		return false
	}
	it.current++
	return true
}

func (it *iterator[T]) Value() T {
	return it.slice[it.current-1]
}

func (it *iterator[T]) seal() {}
