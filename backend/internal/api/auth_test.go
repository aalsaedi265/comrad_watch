package api

import "testing"

func TestIsValidEmail(t *testing.T) {
	valid := []string{
		"user@example.com",
		"a.b+tag@sub.example.co",
		"x@y.io",
	}
	for _, e := range valid {
		if !isValidEmail(e) {
			t.Errorf("isValidEmail(%q) = false, want true", e)
		}
	}

	invalid := []string{
		"",
		"no-at-sign",
		"@example.com", // empty local part
		"user@",        // empty domain
		"user@nodot",   // domain has no dot
		"user@a",       // domain too short
	}
	for _, e := range invalid {
		if isValidEmail(e) {
			t.Errorf("isValidEmail(%q) = true, want false", e)
		}
	}
}
