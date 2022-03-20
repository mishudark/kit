package batch

import (
	"sync"
)

// ForEach should iterate over the provided items to perform any operation
// example:
// func(item int) error {
//    log.Println(item)
//    return nil
// }
type ForEach[T any] func(item T) error

// Run an operation concurrently with the given number of provided workers
// example:
// items := make(chan Foo)
// for err := range Run(10, items, forEachFn) {
//   log.Println(err)
// }
func Run[T any](workers int, items <-chan T, forEach ForEach[T]) chan error {
	var wg sync.WaitGroup
	wg.Add(workers)

	err := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			for item := range items {
				err <- forEach(item)
			}
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(err)
	}()

	return err
}
