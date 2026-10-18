package core

import (
	"strings"
	"testing"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

func TestBuildConnectionStringDSN(t *testing.T) {
	profile := config.ConnectionProfile{Type: "dsn", DSN: "MyDSN", Username: "bob"}
	got, err := BuildConnectionString(profile, "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "DSN=MyDSN;UID=bob;PWD=secret;"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildConnectionStringDSNMissing(t *testing.T) {
	_, err := BuildConnectionString(config.ConnectionProfile{Type: "dsn"}, "")
	if err == nil {
		t.Fatal("expected error for missing DSN")
	}
}

func TestBuildConnectionStringFileDSNMissing(t *testing.T) {
	_, err := BuildConnectionString(config.ConnectionProfile{Type: "file_dsn"}, "")
	if err == nil {
		t.Fatal("expected error for missing file dsn path")
	}
}

func TestBuildConnectionStringDriverMissing(t *testing.T) {
	_, err := BuildConnectionString(config.ConnectionProfile{Type: "driver"}, "")
	if err == nil {
		t.Fatal("expected error for missing driver")
	}
}

func TestBuildConnectionStringConnectionStringMissing(t *testing.T) {
	_, err := BuildConnectionString(config.ConnectionProfile{Type: "connection_string"}, "")
	if err == nil {
		t.Fatal("expected error for missing connection string")
	}
}

func TestBuildConnectionStringUnsupportedType(t *testing.T) {
	_, err := BuildConnectionString(config.ConnectionProfile{Type: "bogus"}, "")
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestBuildConnectionStringDriverAlwaysBraced(t *testing.T) {
	profile := config.ConnectionProfile{Type: "driver", Driver: "SQLite3 ODBC Driver"}
	got, err := BuildConnectionString(profile, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Driver={SQLite3 ODBC Driver};"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildConnectionStringEscapesSpecialChars(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{"semicolon", "has;semicolon", "{has;semicolon}"},
		{"open brace", "has{brace", "{has{brace}"},
		{"close brace doubled", "has}brace", "{has}}brace}"},
		{"equals", "has=equals", "{has=equals}"},
		{"leading space", " leading", "{ leading}"},
		{"trailing space", "trailing ", "{trailing }"},
		{"plain", "plain", "plain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := config.ConnectionProfile{Type: "dsn", DSN: tc.value}
			got, err := BuildConnectionString(profile, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := "DSN=" + tc.want + ";"
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestBuildConnectionStringFileDSNAndUIDPWDEscaping(t *testing.T) {
	profile := config.ConnectionProfile{Type: "file_dsn", FilePath: "my;file.dsn", Username: "u=1"}
	got, err := BuildConnectionString(profile, "p;w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "FILEDSN={my;file.dsn};") {
		t.Fatalf("missing escaped FILEDSN in %q", got)
	}
	if !strings.Contains(got, "UID={u=1};") {
		t.Fatalf("missing escaped UID in %q", got)
	}
	if !strings.Contains(got, "PWD={p;w};") {
		t.Fatalf("missing escaped PWD in %q", got)
	}
}

func TestBuildConnectionStringConnectionStringPassthrough(t *testing.T) {
	profile := config.ConnectionProfile{Type: "connection_string", ConnectionString: "Driver={Foo};Server=bar;"}
	got, err := BuildConnectionString(profile, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Driver={Foo};Server=bar;" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildConnectionStringDoesNotDuplicateUIDPWD(t *testing.T) {
	profile := config.ConnectionProfile{
		Type:             "connection_string",
		ConnectionString: "Driver={Foo};UID=already;PWD=already;",
		Username:         "bob",
	}
	got, err := BuildConnectionString(profile, "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(got, "UID=") != 1 || strings.Count(got, "PWD=") != 1 {
		t.Fatalf("expected UID/PWD not to be duplicated, got %q", got)
	}
}
