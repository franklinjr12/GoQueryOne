package app

import "testing"

func TestUIState(t *testing.T) {
	s := NewUIState(nil, "main/")
	_ = s.Put("main/client/split", "300 900")
	_ = s.PutExpiring("other/x", "1")
	if v, ok := s.Get("main/client/split"); !ok || v != "300 900" {
		t.Fatal(v, ok)
	}
	if _, ok := s.Get("other/x"); ok {
		t.Fatal("prefix filter")
	}
	if _, ok := s.Timestamp("main/client/split"); ok || s.ExpireDuration() != 0 || s.Load() != nil || s.Save() != nil {
		t.Fatal("no-ops")
	}
	s.SetExpireDuration(1)
	_ = s.Remove("main/client/split")
	if len(s.Values) != 0 {
		t.Fatal("remove")
	}
	all := NewUIState(map[string]string{"a": "1"})
	_ = all.Put("b", "2")
	if len(all.Values) != 2 {
		t.Fatal("no prefixes keeps everything")
	}
}
