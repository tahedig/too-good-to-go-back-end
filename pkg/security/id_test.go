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
