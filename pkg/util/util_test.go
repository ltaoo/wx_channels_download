package util

import "testing"

func TestEncodeUint64ToBase64(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "5397107284542310304", want: "SuZgeu3RR6A"},
		{in: "14887697630331082851", want: "zpu6HctPGGM"},
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
		{in: "SuZgeu3RR6A", want: "5397107284542310304"},
		{in: "zpu6HctPGGM", want: "14887697630331082851"},
	}
	for _, tt := range tests {
		got := DecodeBase64ToUint64String(tt.in)
		if got != tt.want {
			t.Fatalf("DecodeBase64ToUint64String(%q)=%q, want %q", tt.in, got, tt.want)
		}
	}
}

