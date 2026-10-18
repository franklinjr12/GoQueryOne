package core

import "unicode/utf16"

// StatementAt returns the statement whose range contains caret, plus its
// byte-offset range [start, end) in text. caret is a byte offset into text
// (UTF-8), consistent with SplitSQLScriptRanges.
//
// Win32 edit controls - including lxn/walk's TextEdit, via EM_GETSEL/
// EM_SETSEL - report caret and selection positions as UTF-16 code-unit
// offsets, not byte offsets. Callers bridging from such a control must
// convert with UTF16OffsetToByteOffset before calling StatementAt, and
// convert the returned range back with ByteOffsetToUTF16Offset before
// calling SetTextSelection.
//
// A caret inside a statement's range selects that statement. A caret in the
// gap between two statements (e.g. a blank line, or after the final ';')
// selects the nearest preceding statement, matching ODBC Query Tool. A
// caret before the first statement selects that first statement, since
// there is no preceding one.
func StatementAt(text string, caret int) (stmt string, start, end int, ok bool) {
	ranges := SplitSQLScriptRanges(text)
	if len(ranges) == 0 {
		return "", 0, 0, false
	}
	if caret < 0 {
		caret = 0
	}
	if caret > len(text) {
		caret = len(text)
	}

	for i, r := range ranges {
		if caret < r.Start {
			if i == 0 {
				return text[r.Start:r.End], r.Start, r.End, true
			}
			prev := ranges[i-1]
			return text[prev.Start:prev.End], prev.Start, prev.End, true
		}
		if caret <= r.End {
			return text[r.Start:r.End], r.Start, r.End, true
		}
	}

	last := ranges[len(ranges)-1]
	return text[last.Start:last.End], last.Start, last.End, true
}

// UTF16OffsetToByteOffset converts a UTF-16 code-unit offset (as reported by
// a Win32 edit control) into a byte offset into the UTF-8 encoded text.
func UTF16OffsetToByteOffset(text string, utf16Offset int) int {
	if utf16Offset <= 0 {
		return 0
	}
	units := 0
	for i, r := range text {
		if units >= utf16Offset {
			return i
		}
		units += utf16RuneLen(r)
	}
	return len(text)
}

// ByteOffsetToUTF16Offset converts a byte offset into UTF-8 encoded text
// into the equivalent UTF-16 code-unit offset expected by a Win32 edit
// control.
func ByteOffsetToUTF16Offset(text string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	units := 0
	for i, r := range text {
		if i >= byteOffset {
			return units
		}
		units += utf16RuneLen(r)
	}
	return units
}

func utf16RuneLen(r rune) int {
	n := utf16.RuneLen(r)
	if n < 1 {
		return 1
	}
	return n
}
