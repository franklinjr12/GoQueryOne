package core

import (
	"reflect"
	"testing"

	"github.com/franklinjr12/GoQueryOne/internal/config"
)

func TestUnfilledPlaceholders(t *testing.T) {
	got := UnfilledPlaceholders("Driver={X};Server=<server>;Database=<database>;Host=<server>;PWD=a<b")
	if !reflect.DeepEqual(got, []string{"<server>", "<database>"}) {
		t.Fatalf("got %v", got)
	}
	if UnfilledPlaceholders("DSN=x;") != nil {
		t.Fatal("expected none")
	}
}

func TestBuildConnectionStringDriverExtra(t *testing.T) {
	p := config.ConnectionProfile{Type: "driver", Driver: "ODBC Driver 18 for SQL Server", ConnectionString: " ;Server=db1;Database=sales", Username: "sa"}
	got, err := BuildConnectionString(p, "pw")
	if err != nil {
		t.Fatal(err)
	}
	want := "Driver={ODBC Driver 18 for SQL Server};Server=db1;Database=sales;UID=sa;PWD=pw;"
	if got != want {
		t.Fatalf("got %q", got)
	}
}
