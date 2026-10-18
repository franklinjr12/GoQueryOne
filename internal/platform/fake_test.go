package platform

import (
	"errors"
	"testing"
)

var (
	_ Clipboard   = (*FakeClipboard)(nil)
	_ FileDialogs = (*FakeFileDialogs)(nil)
	_ ODBCAdmin   = (*FakeODBCAdmin)(nil)
)

func TestFakeClipboardRoundTrip(t *testing.T) {
	c := NewFakeClipboard()
	if err := c.SetText("hello"); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	got, err := c.Text()
	if err != nil {
		t.Fatalf("Text: %v", err)
	}
	if got != "hello" {
		t.Fatalf("Text() = %q, want %q", got, "hello")
	}
}

func TestFakeFileDialogsQueuedResponses(t *testing.T) {
	dlgs := NewFakeFileDialogs()
	dlgs.OpenResponses = []FakeFileResponse{{Path: "a.sql", OK: true}}
	dlgs.SaveResponses = []FakeFileResponse{{Err: errors.New("disk full")}}

	path, ok, err := dlgs.ShowOpen(OpenFileRequest{Title: "Open"})
	if err != nil || !ok || path != "a.sql" {
		t.Fatalf("ShowOpen() = (%q, %v, %v)", path, ok, err)
	}
	if len(dlgs.OpenRequests) != 1 || dlgs.OpenRequests[0].Title != "Open" {
		t.Fatalf("expected request to be recorded, got %#v", dlgs.OpenRequests)
	}

	// Second call has no queued response left.
	path, ok, err = dlgs.ShowOpen(OpenFileRequest{})
	if err != nil || ok || path != "" {
		t.Fatalf("expected cancel on exhausted queue, got (%q, %v, %v)", path, ok, err)
	}

	_, _, err = dlgs.ShowSave(SaveFileRequest{Title: "Save"})
	if err == nil || err.Error() != "disk full" {
		t.Fatalf("expected queued error, got %v", err)
	}
	if len(dlgs.SaveRequests) != 1 {
		t.Fatalf("expected save request to be recorded, got %#v", dlgs.SaveRequests)
	}
}

func TestFakeODBCAdminReturnsConfiguredPath(t *testing.T) {
	admin := NewFakeODBCAdmin()
	admin.Path = `C:\Windows\System32\odbcad32.exe`

	path, err := admin.Open("x64")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if path != admin.Path {
		t.Fatalf("Open() = %q, want %q", path, admin.Path)
	}
	if len(admin.OpenedArches) != 1 || admin.OpenedArches[0] != "x64" {
		t.Fatalf("expected arch to be recorded, got %#v", admin.OpenedArches)
	}
}

func TestFakeODBCAdminDefaultsToError(t *testing.T) {
	admin := NewFakeODBCAdmin()
	if _, err := admin.Open("x86"); err == nil {
		t.Fatal("expected error when no path is configured")
	}
}

func TestFakeODBCAdminPropagatesConfiguredError(t *testing.T) {
	admin := NewFakeODBCAdmin()
	admin.Err = errors.New("boom")
	if _, err := admin.Open("x86"); err == nil || err.Error() != "boom" {
		t.Fatalf("expected configured error, got %v", err)
	}
}
