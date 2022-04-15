package batch

import "testing"

func TestReduce(t *testing.T) {
	tc := [...]struct {
		name      string
		in        []int
		initValue int
		out       int
		reducer   Reducer[int, int]
	}{
		{
			name:      "sum numbers",
			in:        []int{1, 2, 3},
			initValue: 0,
			out:       6,
			reducer: func(accum, _, item int) int {
				return accum + item
			},
		},
		{
			name:      "reducer with nop op",
			in:        []int{1, 2, 3},
			initValue: 0,
			out:       0,
			reducer: func(accum, _, item int) int {
				return accum
			},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			got := Reduce(tt.in, tt.reducer, 0)

			if got != tt.out {
				t.Errorf("expected: %d, got: %d", tt.out, got)
			}
		})
	}
}
