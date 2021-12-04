package batch

import (
	"testing"
)

func TestChunk(t *testing.T) {
	t.Parallel()

	tc := [...]struct {
		name        string
		size        int
		itemsSent   int
		calledTimes int
		exec        Exec[int]
	}{
		{
			name:        "reach the chunk limit once",
			size:        2,
			itemsSent:   2,
			calledTimes: 1,
			exec: func(items []int) error {
				return nil
			},
		},
		{
			name:        "reach the chunk limit once + 1 item",
			size:        2,
			itemsSent:   3,
			calledTimes: 2,
			exec: func(items []int) error {
				return nil
			},
		},
		{
			name:        "just 1 item",
			size:        2,
			itemsSent:   1,
			calledTimes: 1,
			exec: func(items []int) error {
				if len(items) != 1 {
					t.Errorf("expected chunk items: 1, got: %d", len(items))
				}

				return nil
			},
		},
		{
			name:        "zero items sent",
			size:        2,
			itemsSent:   0,
			calledTimes: 0,
			exec: func(items []int) error {
				return nil
			},
		},
	}

	for _, tt := range tc {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			items := make(chan int)
			go func() {
				for i := 0; i < tt.itemsSent; i++ {
					items <- 1
				}
				close(items)
			}()

			var counter int
			for range Chunk(tt.size, items, tt.exec) {
				counter++
			}

			if counter != tt.calledTimes {
				t.Errorf("[%s]: expected: %d, called: %d times", tt.name, tt.calledTimes, counter)
			}
		})
	}
}
