package core

import "testing"

func TestStatementAt(t *testing.T) {
	text := "SELECT 1;\n\nSELECT 2;\nSELECT 3;\n"
	// byte offsets:
	// "SELECT 1"  -> [0,8)   then ';' at 8, '\n' at 9, '\n' at 10
	// "SELECT 2"  -> [11,19) then ';' at 19, '\n' at 20
	// "SELECT 3"  -> [21,29) then ';' at 29, '\n' at 30

	cases := []struct {
		name      string
		caret     int
		wantStmt  string
		wantStart int
		wantEnd   int
	}{
		{"inside first", 3, "SELECT 1", 0, 8},
		{"at first start", 0, "SELECT 1", 0, 8},
		{"at first end (before ;)", 8, "SELECT 1", 0, 8},
		{"blank line between stmts picks preceding", 9, "SELECT 1", 0, 8},
		{"blank line between stmts picks preceding (2)", 10, "SELECT 1", 0, 8},
		{"inside second", 15, "SELECT 2", 11, 19},
		{"after final ; and trailing newline picks last stmt", 30, "SELECT 3", 21, 29},
		{"caret beyond text clamps to end", 1000, "SELECT 3", 21, 29},
		{"negative caret clamps to zero", -5, "SELECT 1", 0, 8},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmt, start, end, ok := StatementAt(text, tc.caret)
			if !ok {
				t.Fatalf("StatementAt(%d) ok = false, want true", tc.caret)
			}
			if stmt != tc.wantStmt || start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("StatementAt(%d) = (%q, %d, %d), want (%q, %d, %d)",
					tc.caret, stmt, start, end, tc.wantStmt, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

func TestStatementAtCaretBeforeFirstStatement(t *testing.T) {
	text := "\n\n  SELECT 1;\nSELECT 2;"
	stmt, start, end, ok := StatementAt(text, 0)
	if !ok {
		t.Fatal("expected ok")
	}
	if stmt != "SELECT 1" {
		t.Fatalf("expected first statement to be selected when caret precedes it, got %q (%d,%d)", stmt, start, end)
	}
}

func TestStatementAtEmptyText(t *testing.T) {
	if _, _, _, ok := StatementAt("", 0); ok {
		t.Fatal("expected ok=false for empty text")
	}
	if _, _, _, ok := StatementAt("   \n\t  ;  ", 0); ok {
		t.Fatal("expected ok=false for text with only whitespace/delimiters")
	}
}

func TestStatementAtSingleStatementNoSemicolon(t *testing.T) {
	text := "SELECT 1"
	stmt, start, end, ok := StatementAt(text, 4)
	if !ok || stmt != "SELECT 1" || start != 0 || end != 8 {
		t.Fatalf("got (%q,%d,%d,%v)", stmt, start, end, ok)
	}
}

func TestUTF16ByteOffsetRoundTrip(t *testing.T) {
	// "São Paulo, ação 😀" mixes ASCII, Latin-1 Supplement (2-byte UTF-8,
	// 1 UTF-16 unit) and an emoji outside the BMP (4-byte UTF-8, 2 UTF-16
	// units / a surrogate pair).
	text := "São Paulo, ação 😀 fim"

	cases := []struct {
		name        string
		utf16Offset int
		wantByte    int
	}{
		{"start", 0, 0},
		// "S"(1 unit, byte 0) "ã"(1 unit, 2 bytes at byte 1) -> unit 2 starts at the 'o' (byte 3)
		{"at the o in São", 2, 3},
		{"after São", 4, 5},
		// emoji is a surrogate pair (2 utf16 units) and 4 bytes in utf-8; it starts at byte 19
		{"at emoji start", 16, 19},
		{"after emoji", 18, 23},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotByte := UTF16OffsetToByteOffset(text, tc.utf16Offset)
			if gotByte != tc.wantByte {
				t.Fatalf("UTF16OffsetToByteOffset(%q, %d) = %d, want %d", text, tc.utf16Offset, gotByte, tc.wantByte)
			}
			gotUTF16 := ByteOffsetToUTF16Offset(text, gotByte)
			if gotUTF16 != tc.utf16Offset {
				t.Fatalf("ByteOffsetToUTF16Offset(%q, %d) = %d, want %d (round trip of case %q)", text, gotByte, gotUTF16, tc.utf16Offset, tc.name)
			}
		})
	}
}

func TestUTF16OffsetToByteOffsetClamping(t *testing.T) {
	text := "abc"
	if got := UTF16OffsetToByteOffset(text, -1); got != 0 {
		t.Fatalf("expected 0 for negative offset, got %d", got)
	}
	if got := UTF16OffsetToByteOffset(text, 1000); got != len(text) {
		t.Fatalf("expected clamp to len(text)=%d, got %d", len(text), got)
	}
}

func TestByteOffsetToUTF16OffsetClamping(t *testing.T) {
	text := "abc"
	if got := ByteOffsetToUTF16Offset(text, -1); got != 0 {
		t.Fatalf("expected 0 for negative offset, got %d", got)
	}
	if got := ByteOffsetToUTF16Offset(text, 1000); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}

func TestStatementAtWithNonASCIICaret(t *testing.T) {
	// Exercise the documented bridging pattern: a UTF-16 caret offset from a
	// Win32 edit control must be converted to a byte offset before calling
	// StatementAt.
	text := "SELECT 'ação' AS v;\nSELECT '😀' AS e;"
	utf16Caret := 25 // inside the second statement, after the emoji literal starts
	byteCaret := UTF16OffsetToByteOffset(text, utf16Caret)
	stmt, _, _, ok := StatementAt(text, byteCaret)
	if !ok {
		t.Fatal("expected ok")
	}
	if stmt != "SELECT '😀' AS e" {
		t.Fatalf("got %q", stmt)
	}
}
