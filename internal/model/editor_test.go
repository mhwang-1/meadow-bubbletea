package model

import "testing"

func TestInsertRunesPasteWithLF(t *testing.T) {
	e := NewEditorModel("", 80, 24)
	e.insertRunes([]rune("first\nsecond\nthird"), true)

	if got, want := len(e.lines), 3; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if got, want := e.lines[0], "first"; got != want {
		t.Fatalf("line 1 = %q, want %q", got, want)
	}
	if got, want := e.lines[1], "second"; got != want {
		t.Fatalf("line 2 = %q, want %q", got, want)
	}
	if got, want := e.lines[2], "third"; got != want {
		t.Fatalf("line 3 = %q, want %q", got, want)
	}
}

func TestInsertRunesPasteWithCRLF(t *testing.T) {
	e := NewEditorModel("", 80, 24)
	e.insertRunes([]rune("first\r\nsecond\r\nthird"), true)

	if got, want := len(e.lines), 3; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if got, want := e.lines[0], "first"; got != want {
		t.Fatalf("line 1 = %q, want %q", got, want)
	}
	if got, want := e.lines[1], "second"; got != want {
		t.Fatalf("line 2 = %q, want %q", got, want)
	}
	if got, want := e.lines[2], "third"; got != want {
		t.Fatalf("line 3 = %q, want %q", got, want)
	}
}

func TestInsertRunesPasteWithCR(t *testing.T) {
	e := NewEditorModel("", 80, 24)
	e.insertRunes([]rune("first\rsecond\rthird"), true)

	if got, want := len(e.lines), 3; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if got, want := e.lines[0], "first"; got != want {
		t.Fatalf("line 1 = %q, want %q", got, want)
	}
	if got, want := e.lines[1], "second"; got != want {
		t.Fatalf("line 2 = %q, want %q", got, want)
	}
	if got, want := e.lines[2], "third"; got != want {
		t.Fatalf("line 3 = %q, want %q", got, want)
	}
}
