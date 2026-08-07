package util

import "testing"

func TestEncodeUint64ToBase64(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// existing test cases
		{in: "5397107284542310304", want: "SuZgeu3RR6A"},
		{in: "14887697630331082851", want: "zpu6HctPGGM"},
		// new test cases
		{in: "0", want: "AAAAAAAAAAA"},
		{in: "1", want: "AAAAAAAAAAE"},
		{in: "256", want: "AAAAAAAAAQA"},
		{in: "18446744073709551615", want: "__________8"},
		{in: "14954560208914487678", want: "z4lFRwCKGX4"},
		// invalid inputs
		{in: "-1", want: ""},
		{in: "18446744073709551616", want: ""}, // 2^64, exceeds range
		{in: "abc", want: ""},                    // not a number
	}
	for _, tt := range tests {
		got := EncodeUint64ToBase64(tt.in)
		if got != tt.want {
			t.Fatalf("EncodeUint64ToBase64(%q)=%q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDecodeBase64ToUint64String(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		// existing test cases
		{in: "SuZgeu3RR6A", want: "5397107284542310304"},
		{in: "zpu6HctPGGM", want: "14887697630331082851"},
		// new test cases
		{in: "AAAAAAAAAAA", want: "0"},
		{in: "AAAAAAAAAAE", want: "1"},
		{in: "AAAAAAAAAQA", want: "256"},
		{in: "__________8", want: "18446744073709551615"},
		{in: "z4lFRwCKGX4", want: "14954560208914487678"},
		// invalid inputs
		{in: "", want: ""},
	}
	for _, tt := range tests {
		got := DecodeBase64ToUint64String(tt.in)
		if got != tt.want {
			t.Fatalf("DecodeBase64ToUint64String(%q)=%q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRoundtrip(t *testing.T) {
	// encode → decode should return the same value
	for _, in := range []string{
		"0",
		"1",
		"256",
		"5397107284542310304",
		"14887697630331082851",
		"14954560208914487678",
		"18446744073709551615",
	} {
		enc := EncodeUint64ToBase64(in)
		if enc == "" {
			t.Fatalf("EncodeUint64ToBase64(%q) returned empty", in)
		}
		got := DecodeBase64ToUint64String(enc)
		if got != in {
			t.Fatalf("roundtrip: %q → %q → %q, want %q", in, enc, got, in)
		}
	}
}
