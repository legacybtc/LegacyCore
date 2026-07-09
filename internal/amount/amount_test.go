package amount

import "testing"

func TestParseLBTC(t *testing.T) {
	tests := map[string]int64{
		"1":          100000000,
		"1.0":        100000000,
		"0.5":        50000000,
		"0.00000546": 546,
		"21":         2100000000,
		".25":        25000000,
	}
	for in, want := range tests {
		got, err := ParseLBTC(in)
		if err != nil {
			t.Fatalf("ParseLBTC(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseLBTC(%q)=%d want %d", in, got, want)
		}
	}
}

func TestParseLBTCRejectsAmbiguous(t *testing.T) {
	for _, in := range []string{"", "0", "-1", "1.000000001", "1e8", "1,000", "abc"} {
		if _, err := ParseLBTC(in); err == nil {
			t.Fatalf("ParseLBTC(%q) succeeded, want error", in)
		}
	}
}

func TestFormatLBTC(t *testing.T) {
	if got := FormatLBTC(100000000); got != "1.00000000" {
		t.Fatalf("got %s", got)
	}
	if got := FormatLBTC(546); got != "0.00000546" {
		t.Fatalf("got %s", got)
	}
}
