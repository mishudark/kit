package batch

// Reducer takes a an accumulator value as well as the index and the actual item from the iterator,
// then applies the reducer to each item
type Reducer[In, Out any] func(accum Out, index int, item In) Out

func Reduce[In, Out any](items []In, reducer Reducer[In, Out], initialValue Out) Out {
	for i, item := range items {
		initialValue = reducer(initialValue, i, item)
	}

	return initialValue
}
