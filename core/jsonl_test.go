package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// The bug this exists to prevent: bufio.Scanner treats one over-long line as the
// end of the stream. Agent transcripts put a whole tool output on a single line,
// so the first big command in a session used to cut off every message after it.
func TestScanJSONL_KeepsReadingPastAnOverlongLine(t *testing.T) {
	huge := strings.Repeat("x", 4096)
	input := "first\n" + huge + "\nsecond\nthird\n"

	var got []string
	if err := ScanJSONL(strings.NewReader(input), 1024, func(line []byte) {
		got = append(got, string(line))
	}); err != nil {
		t.Fatalf("ScanJSONL: %v", err)
	}

	want := []string{"first", "second", "third"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("lines = %v, want %v — an oversized line must be skipped, not end the file", got, want)
	}
}

// Several oversized lines in a row, and one at the very end, must not confuse the
// skip state into dropping the good lines between them.
func TestScanJSONL_HandlesRepeatedOverlongLines(t *testing.T) {
	huge := strings.Repeat("y", 5000)
	input := "a\n" + huge + "\n" + huge + "\nb\n" + huge + "\nc\n" + huge + "\n"

	var got []string
	if err := ScanJSONL(strings.NewReader(input), 512, func(line []byte) {
		got = append(got, string(line))
	}); err != nil {
		t.Fatalf("ScanJSONL: %v", err)
	}
	if fmt.Sprint(got) != fmt.Sprint([]string{"a", "b", "c"}) {
		t.Errorf("lines = %v, want [a b c]", got)
	}
}

func TestScanJSONL_ReadsLinesLongerThanTheReadBuffer(t *testing.T) {
	long := strings.Repeat("z", 200*1024)
	var got []string
	if err := ScanJSONL(strings.NewReader(long+"\ntail\n"), 1<<20, func(line []byte) {
		got = append(got, string(line))
	}); err != nil {
		t.Fatalf("ScanJSONL: %v", err)
	}
	if len(got) != 2 || len(got[0]) != len(long) || got[1] != "tail" {
		t.Errorf("a %d-byte line under the limit must be delivered whole; got %d lines, first %d bytes",
			len(long), len(got), len(got[0]))
	}
}

func TestScanJSONL_LastLineWithoutNewlineAndBlanksSkipped(t *testing.T) {
	var got []string
	if err := ScanJSONL(strings.NewReader("a\n\n\nb"), 1024, func(line []byte) {
		got = append(got, string(line))
	}); err != nil {
		t.Fatalf("ScanJSONL: %v", err)
	}
	if fmt.Sprint(got) != fmt.Sprint([]string{"a", "b"}) {
		t.Errorf("lines = %v, want [a b]", got)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

// A real read failure is not a truncated transcript — it must reach the caller
// rather than pass for a short conversation.
func TestScanJSONL_SurfacesReadErrors(t *testing.T) {
	want := errors.New("disk went away")
	err := ScanJSONL(failingReader{err: want}, 1024, func([]byte) {})
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}
