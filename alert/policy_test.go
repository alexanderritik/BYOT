package alert

import "testing"

func TestFailureAlertDue(t *testing.T) {
	th := 3
	for _, tc := range []struct {
		n    int
		want bool
	}{
		{1, false}, {2, false}, {3, true}, {4, false}, {5, false}, {6, true}, {9, true}, {10, false},
	} {
		if got := FailureAlertDue(tc.n, th); got != tc.want {
			t.Fatalf("n=%d want %v got %v", tc.n, tc.want, got)
		}
	}
}
