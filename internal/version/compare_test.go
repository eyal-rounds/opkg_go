package version

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0", "1.0", 0},
		{"1.0", "1.0.1", -1},
		{"1.0.1", "1.0", 1},
		{"1.0~beta", "1.0", -1},
		{"1.0", "1.0~beta", 1},
		{"1.0a", "1.0b", -1},
		{"2:1.0", "1:5.0", 1},
		{"1.0-2", "1.0-10", -1},
		{"001", "1", 0},
		{"1.0+git", "1.0", 1},
	}
	for _, tc := range cases {
		if got := Compare(tc.a, tc.b); got != tc.want {
			t.Fatalf("Compare(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCompareDigitsVsEmpty(t *testing.T) {
	if got := Compare("1", "1~"); got <= 0 {
		t.Fatalf("expected 1 > 1~, got %d", got)
	}
}

func TestCompareOp(t *testing.T) {
	truthy := []struct {
		a, op, b string
	}{
		{"1.0", "=", "1.0"},
		{"1.0", "<=", "1.0"},
		{"1.0", ">", "0.9"},
		{"1.0", ">=", "1.0"},
		{"1.0", "<<", "2.0"},
		{"2.0", ">>", "1.0"},
	}
	for _, tc := range truthy {
		ok, err := CompareOp(tc.a, tc.op, tc.b)
		if err != nil {
			t.Fatalf("CompareOp(%q,%q,%q) unexpected error: %v", tc.a, tc.op, tc.b, err)
		}
		if !ok {
			t.Fatalf("CompareOp(%q,%q,%q) = false, want true", tc.a, tc.op, tc.b)
		}
	}
	falsy := []struct {
		a, op, b string
	}{
		{"1.0", ">", "1.0"},
		{"1.0", "<", "0.9"},
	}
	for _, tc := range falsy {
		ok, err := CompareOp(tc.a, tc.op, tc.b)
		if err != nil {
			t.Fatalf("CompareOp(%q,%q,%q) unexpected error: %v", tc.a, tc.op, tc.b, err)
		}
		if ok {
			t.Fatalf("CompareOp(%q,%q,%q) = true, want false", tc.a, tc.op, tc.b)
		}
	}
	if _, err := CompareOp("1", "!=", "1"); err == nil {
		t.Fatalf("expected error for unsupported operator")
	}
}
