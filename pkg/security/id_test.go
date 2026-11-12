package security

import (
	"regexp"
	"testing"
)

func TestNewID(t *testing.T) {
	uuidV4 := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	a, err := NewID()
	if err != nil || !uuidV4.MatchString(a) {
		t.Fatalf("NewID() = %q, %v; want a UUID v4", a, err)
	}
	if b, _ := NewID(); a == b {
		t.Fatal("ids must be unique")
	}
}

func TestIsID(t *testing.T) {
	id, _ := NewID()
	for s, want := range map[string]bool{
		id:                                     true,
		"7C9E6679-7425-40DE-944B-E07FC1F90AE7": true,
		"":                                     false,
		"user-1":                               false,
		"7c9e6679-7425-40de-944b-e07fc1f90ae":  false,
		"7c9e6679x7425-40de-944b-e07fc1f90ae7": false,
		"7c9e6679-7425-40de-944b-e07fc1f90aeg": false,
	} {
		if got := IsID(s); got != want {
			t.Errorf("IsID(%q) = %v, want %v", s, got, want)
		}
	}
}
