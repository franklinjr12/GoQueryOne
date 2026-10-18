package app

import (
	"strings"
	"testing"
	"time"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

func TestNewProfileID(t *testing.T) {
	now := time.Unix(0, 42)
	if got := NewProfileID("  Prod  DB ", now); got != "prod-db-42" {
		t.Fatalf("got %q", got)
	}
	if got := NewProfileID("", now); got != "connection-42" {
		t.Fatalf("got %q", got)
	}
}

func TestUniqueProfileName(t *testing.T) {
	ex := []config.ConnectionProfile{{Name: "A"}, {Name: "a (2)"}}
	if got := UniqueProfileName("B", ex); got != "B" {
		t.Fatal(got)
	}
	if got := UniqueProfileName("a", ex); got != "a (3)" {
		t.Fatal(got)
	}
}

func TestDuplicateProfile(t *testing.T) {
	p := config.ConnectionProfile{ID: "x", Name: "Prod", Type: "dsn", DSN: "PROD", Username: "sa", SavePassword: true, CredentialRef: "dpapi:GoQueryOne:x"}
	ex := []config.ConnectionProfile{p, {Name: "Prod copy"}}
	cp := DuplicateProfile(p, ex, time.Unix(0, 7))
	if cp.Name != "Prod copy (2)" || cp.ID == p.ID || !strings.HasPrefix(cp.ID, "prod-copy-(2)-") {
		t.Fatalf("copy = %+v", cp)
	}
	if cp.CredentialRef != "" || cp.SavePassword {
		t.Fatal("credential must not be shared")
	}
	if cp.DSN != "PROD" || cp.Username != "sa" || cp.Type != "dsn" {
		t.Fatalf("fields not copied: %+v", cp)
	}
	if ProfileLabel(p) != "Prod [dsn]" {
		t.Fatal(ProfileLabel(p))
	}
}
