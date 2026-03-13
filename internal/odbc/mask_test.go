package odbc

import (
	"strings"
	"testing"
)

func TestMaskSecrets(t *testing.T) {
	input := `Driver={X};Uid=user;Pwd=supersecret;Password=another;{"password":"abc"}`
	got := MaskSecrets(input)
	if strings.Contains(got, "supersecret") {
		t.Fatalf("masked output leaked secret: %s", got)
	}
	if strings.Contains(got, "another") {
		t.Fatalf("masked output leaked password: %s", got)
	}
	if strings.Contains(got, `"password":"abc"`) {
		t.Fatalf("masked output leaked json password: %s", got)
	}
}
