package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Cue is the shared representation for both formats. Times are offsets from the
// beginning of the media; Text preserves multiline payloads and caption markup.
type Cue struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

// readCaptions selects a parser by extension, then validates the file contents.
func readCaptions(path string) ([]Cue, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".srt" && ext != ".vtt" {
		return nil, fmt.Errorf("unsupported caption extension %q; use .srt or .vtt", ext)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read captions: %w", err)
	}

	return parseCaptions(string(data), ext)
}

// parseCaptions accepts .srt or .vtt as selected by readCaptions. It returns cues
// in file order and stops on malformed input instead of silently losing coverage.
func parseCaptions(data, format string) ([]Cue, error) {
	if !utf8.ValidString(data) || strings.ContainsRune(data, 0) {
		return nil, fmt.Errorf("captions must be UTF-8 text without NUL bytes")
	}

	// Normalize encoding markers and line endings before finding block boundaries.
	data = strings.TrimPrefix(data, "\ufeff")
	data = strings.ReplaceAll(strings.ReplaceAll(data, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(data, "\n")
	vtt := format == ".vtt"
	i := 0

	if vtt {
		header := lines[0]
		if header != "WEBVTT" && !strings.HasPrefix(header, "WEBVTT ") && !strings.HasPrefix(header, "WEBVTT\t") {
			return nil, fmt.Errorf("line 1: missing WEBVTT header")
		}

		// WebVTT header metadata ends at the first blank line, before any cues.
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			if strings.Contains(lines[i], "-->") {
				return nil, fmt.Errorf("line %d: missing blank line after WEBVTT header", i+1)
			}

			i++
		}
	}

	var cues []Cue

	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}

		// A blank line separates blocks. Retain the source offset for error messages.
		first := i
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			i++
		}

		block := lines[first:i]

		// Comments and presentation rules contain no spoken caption text.
		head := strings.TrimSpace(block[0])
		if vtt && (head == "NOTE" || strings.HasPrefix(head, "NOTE ") || strings.HasPrefix(head, "NOTE\t") || head == "STYLE" || head == "REGION") {
			continue
		}

		// SRT requires an index line; WebVTT may start directly with timestamps.
		timing := 0

		if !vtt {
			if !digits(head) {
				return nil, fmt.Errorf("line %d: expected numeric SRT cue identifier", first+1)
			}

			timing = 1
		} else if !strings.Contains(head, "-->") {
			timing = 1
		}

		if timing >= len(block) {
			return nil, fmt.Errorf("line %d: missing cue timestamps", first+1)
		}

		// WebVTT may append positioning settings; only its first three fields matter.
		fields := strings.Fields(block[timing])
		if len(fields) < 3 || fields[1] != "-->" || (!vtt && len(fields) != 3) {
			return nil, fmt.Errorf("line %d: expected start --> end timestamps", first+timing+1)
		}

		a, err := parseTimestamp(fields[0], vtt)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", first+timing+1, err)
		}

		b, err := parseTimestamp(fields[2], vtt)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", first+timing+1, err)
		}

		if b <= a {
			return nil, fmt.Errorf("line %d: cue end must be after start", first+timing+1)
		}

		// An empty payload cannot contribute coverage or language evidence.
		text := strings.TrimSpace(strings.Join(block[timing+1:], "\n"))
		if text != "" {
			cues = append(cues, Cue{Start: a, End: b, Text: text})
		}
	}

	if len(cues) == 0 {
		return nil, fmt.Errorf("caption file has no cues with text")
	}

	return cues, nil
}

// digits restricts numeric fields to ASCII digits, excluding signs and decimals.
func digits(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// parseTimestamp handles SRT's comma and WebVTT's dot before milliseconds.
// Only WebVTT allows the hours component to be omitted.
func parseTimestamp(s string, vtt bool) (time.Duration, error) {
	invalid := func() (time.Duration, error) { return 0, fmt.Errorf("invalid timestamp %q", s) }

	separator := ","
	if vtt {
		separator = "."
	}

	fraction := strings.Split(s, separator)
	if len(fraction) != 2 || len(fraction[1]) != 3 || !digits(fraction[1]) {
		return invalid()
	}

	parts := strings.Split(fraction[0], ":")
	if len(parts) != 3 && !(vtt && len(parts) == 2) {
		return invalid()
	}

	for j, part := range parts {
		if !digits(part) || (j == 0 && len(parts) == 3 && len(part) < 2) || ((j > 0 || len(parts) == 2) && len(part) != 2) {
			return invalid()
		}
	}

	var hours int64

	if len(parts) == 3 {
		var err error

		hours, err = strconv.ParseInt(parts[0], 10, 64)
		// Bound before multiplication so hostile timestamps cannot overflow.
		if err != nil || hours > 2562047 {
			return invalid()
		}

		parts = parts[1:]
	}

	// These conversions cannot fail: the fields contain only two or three digits.
	minutes, _ := strconv.ParseInt(parts[0], 10, 64)
	seconds, _ := strconv.ParseInt(parts[1], 10, 64)
	millis, _ := strconv.ParseInt(fraction[1], 10, 64)

	if minutes > 59 || seconds > 59 {
		return invalid()
	}

	// Check the full timestamp too: valid hours alone do not guarantee that adding
	// minutes and seconds stays within time.Duration's signed nanosecond range.
	ms := ((hours*60+minutes)*60+seconds)*1000 + millis
	if ms > int64((1<<63-1)/time.Millisecond) {
		return invalid()
	}

	return time.Duration(ms) * time.Millisecond, nil
}
