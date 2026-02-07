package proc

import "testing"

func TestParseETimeToMinutes(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"00:29", 0},
		{"00:30", 1},
		{"10:29", 10},
		{"10:30", 11},
		{"01:02:29", 62},
		{"01:02:30", 63},
		{"02-01:00:00", 2940},
		{"", 0},
	}

	for _, c := range cases {
		if got := ParseETimeToMinutes(c.in); got != c.want {
			t.Fatalf("输入=%q, got=%d, want=%d", c.in, got, c.want)
		}
	}
}
