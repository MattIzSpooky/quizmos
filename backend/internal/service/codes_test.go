package service

import "testing"

func TestRandomCode(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code, err := randomCode()
		if err != nil {
			t.Fatalf("randomCode: %v", err)
		}
		if len(code) != codeLength {
			t.Fatalf("expected length %d, got %d (%q)", codeLength, len(code), code)
		}
		for _, r := range code {
			if !contains(codeAlphabet, r) {
				t.Fatalf("code %q contains character %q outside the alphabet", code, r)
			}
		}
		seen[code] = true
	}
	// Not a strict guarantee, but with a 32^6 keyspace, 1000 draws
	// colliding down to fewer than ~990 unique values would indicate a
	// broken RNG rather than bad luck.
	if len(seen) < 990 {
		t.Fatalf("expected mostly-unique codes, got only %d unique out of 1000", len(seen))
	}
}

func contains(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
