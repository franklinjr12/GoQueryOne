package app

import (
	"fmt"
	"reflect"
	"strings"
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

func TestAddRecentFile(t *testing.T) {
	var list []string
	for i := 0; i < 12; i++ {
		list = AddRecentFile(list, fmt.Sprintf(`C:\sql\q%d.sql`, i))
	}
	if len(list) != MaxRecentFiles || list[0] != `C:\sql\q11.sql` || list[9] != `C:\sql\q2.sql` {
		t.Fatalf("list = %v", list)
	}
	in := append([]string(nil), list...)
	got := AddRecentFile(in, `c:\SQL\q5.sql`)
	if got[0] != `c:\SQL\q5.sql` || len(got) != MaxRecentFiles {
		t.Fatalf("move to front: %v", got)
	}
	for _, p := range got[1:] {
		if strings.EqualFold(p, `C:\sql\q5.sql`) {
			t.Fatalf("duplicate kept: %v", got)
		}
	}
	if !reflect.DeepEqual(in, list) {
		t.Fatal("input modified")
	}
	if got := AddRecentFile([]string{"a"}, "  "); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("blank: %v", got)
	}
}

func TestRemoveRecentFileAndLabel(t *testing.T) {
	got := RemoveRecentFile([]string{`C:\a.sql`, "", `C:\B.sql`}, `c:\b.sql`)
	if !reflect.DeepEqual(got, []string{`C:\a.sql`}) {
		t.Fatalf("got %v", got)
	}
	if l := RecentFileLabel(0, `C:\R&D\x.sql`); l != `&1 C:\R&&D\x.sql` {
		t.Fatalf("label %q", l)
	}
	if l := RecentFileLabel(9, "y"); l != "&0 y" {
		t.Fatalf("label %q", l)
	}
}
