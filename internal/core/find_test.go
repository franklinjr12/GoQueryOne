package core

import "testing"

func TestFindNext(t *testing.T) {
	text := "SELECT a FROM t; select b FROM Ação"
	s, e, w, ok := FindNext(text, "select", 0, false, true)
	if !ok || s != 0 || e != 6 || w {
		t.Fatal(s, e, w, ok)
	}
	s, _, w, ok = FindNext(text, "select", 1, false, true)
	if !ok || s != 17 || w {
		t.Fatal(s, w, ok)
	}
	s, _, w, ok = FindNext(text, "select", 18, false, true)
	if !ok || s != 0 || !w {
		t.Fatal("wrap", s, w, ok)
	}
	if _, _, _, ok := FindNext(text, "select", 18, false, false); ok {
		t.Fatal("no wrap")
	}
	if _, _, _, ok := FindNext(text, "SELECT", 1, true, false); ok {
		t.Fatal("match case")
	}
	if s, _, _, ok := FindNext(text, "AÇÃO", 0, false, false); !ok || text[s:] != "Ação" {
		t.Fatal("unicode fold", s, ok)
	}
	if _, _, _, ok := FindNext(text, "", 0, false, true); ok {
		t.Fatal("empty")
	}
	if _, _, _, ok := FindNext("ab", "abc", 0, false, true); ok {
		t.Fatal("longer needle")
	}
	if s, _, _, ok := FindNext(text, "SELECT", -5, true, false); !ok || s != 0 {
		t.Fatal("bad from")
	}
}
