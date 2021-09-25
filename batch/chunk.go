package batch

// Exec function will be executed after the desired chunk size is reached
type Exec func(items []interface{}) error

// Chunk execs a desired function when the length of the queue is equal to the provided size param, if
// after to receive the last item there are remaining items in the queue, Exec function will be
// called
func Chunk(size int, items <-chan interface{}, exec Exec) <-chan error {
	errs := make(chan error)
	go func() {

		var counter int
		bucket := make([]interface{}, 0, size)

		for item := range items {
			bucket = append(bucket, item)
			counter++

			if counter == size {
				errs <- exec(bucket)
				bucket = make([]interface{}, 0, size)
				counter = 0
			}
		}

		if len(bucket) != 0 {
			errs <- exec(bucket)
		}

		close(errs)
	}()

	return errs
}
