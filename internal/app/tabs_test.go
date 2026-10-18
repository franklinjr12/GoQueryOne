package app

import (
	"fmt"
	"reflect"
	"testing"
)

func TestFilesToRestore(t *testing.T) {
	exists := func(p string) bool { return p != "gone.sql" }
	got := FilesToRestore([]string{"a.sql", " ", "gone.sql", "A.SQL", "b.sql"}, exists)
	if !reflect.DeepEqual(got, []string{"a.sql", "b.sql"}) {
		t.Fatalf("got %v", got)
	}
	var many []string
	for i := 0; i < MaxRestoredFiles+5; i++ {
		many = append(many, fmt.Sprintf("f%d.sql", i))
	}
	if n := len(FilesToRestore(many, exists)); n != MaxRestoredFiles {
		t.Fatalf("cap: %d", n)
	}
}
