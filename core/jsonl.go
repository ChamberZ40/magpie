package core

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"os"
)

// DefaultJSONLLineLimit bounds a single transcript line. Agent transcripts put a
// whole tool output on one line, and a few hundred KB is routine — 8 MB is the
// point past which the line is certainly machine noise rather than a message.
const DefaultJSONLLineLimit = 8 << 20

// jsonlReadBuffer is the working buffer. Most lines fit; the ones that do not are
// assembled across reads.
const jsonlReadBuffer = 64 * 1024

// ScanJSONL calls fn once per non-empty line of r.
//
// It exists because bufio.Scanner is the wrong tool for an agent transcript:
// Scanner treats a line longer than its buffer as the end of the stream, and the
// only sign is Err() returning ErrTooLong. A caller that does not check it — as
// every transcript reader here did not — reads the beginning of a session and
// reports success. A 400 KB tool-output line inside the first hundred lines of a
// rollout is normal, so /history and the /switch preview were showing the opening
// of the conversation and calling it the tail.
//
// A line past maxLine is skipped and the scan continues: it is tool output, not a
// message, and abandoning the file there is what caused the bug. Read failures,
// by contrast, are returned — a broken file must not pass for a short
// conversation.
//
// line is only valid for the duration of the call; copy it to keep it.
func ScanJSONL(r io.Reader, maxLine int, fn func(line []byte)) error {
	if maxLine <= 0 {
		maxLine = DefaultJSONLLineLimit
	}
	br := bufio.NewReaderSize(r, jsonlReadBuffer)

	var (
		buf     []byte
		dropped bool
		skipped int
	)
	for {
		chunk, err := br.ReadSlice('\n')
		if err == bufio.ErrBufferFull {
			if dropped || len(buf)+len(chunk) > maxLine {
				// Keep draining to the newline so the next line still starts
				// where it should, but stop accumulating.
				dropped = true
				buf = buf[:0]
			} else {
				buf = append(buf, chunk...)
			}
			continue
		}
		if err != nil && err != io.EOF {
			return err
		}

		if !dropped && len(buf)+len(chunk) <= maxLine {
			buf = append(buf, chunk...)
			if line := bytes.TrimRight(buf, "\r\n"); len(line) > 0 {
				fn(line)
			}
		} else {
			skipped++
		}

		if err == io.EOF {
			break
		}
		buf = buf[:0]
		dropped = false
	}

	if skipped > 0 {
		slog.Debug("transcript: skipped oversized lines", "count", skipped, "max_line_bytes", maxLine)
	}
	return nil
}

// ScanJSONLFile is ScanJSONL over a file, with the default line limit.
func ScanJSONLFile(path string, fn func(line []byte)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return ScanJSONL(f, DefaultJSONLLineLimit, fn)
}
