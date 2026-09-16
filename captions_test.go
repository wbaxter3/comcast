package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseCaptions(t *testing.T) {
	tests := []struct {
		name, format, input string
		count               int
	}{
		{name: "srt", format: ".srt", input: "1\n00:00:01,000 --> 00:00:02,500\nHello\nworld", count: 1},
		{name: "bom crlf", format: ".vtt", input: "\ufeffWEBVTT\r\n\r\n00:01.000 --> 00:02.000\r\nHi", count: 1},
		{name: "vtt blocks", format: ".vtt", input: "WEBVTT title\nKind: captions\n\nNOTE comment\nignore\n\nSTYLE\n::cue { color: red }\n\nREGION\nid:one\n\ncue-id\n00:00:01.000 --> 00:00:02.000 align:start\nHello\nworld", count: 1},
		{name: "missing header", format: ".vtt", input: "00:01.000 --> 00:02.000\nHi", count: 0},
		{name: "missing header separator", format: ".vtt", input: "WEBVTT\n00:01.000 --> 00:02.000\nHi", count: 0},
		{name: "bad identifier", format: ".srt", input: "abc\n00:00:01,000 --> 00:00:02,000\nHi", count: 0},
		{name: "bad timestamp", format: ".srt", input: "1\n00:60:00,000 --> 00:61:00,000\nHi", count: 0},
		{name: "reverse", format: ".srt", input: "1\n00:00:02,000 --> 00:00:01,000\nHi", count: 0},
		{name: "missing time", format: ".srt", input: "1", count: 0},
		{name: "empty", format: ".srt", input: "", count: 0},
		{name: "no text", format: ".vtt", input: "WEBVTT\n\n00:01.000 --> 00:02.000\n ", count: 0},
		{name: "binary", format: ".srt", input: "\x00", count: 0},
		{name: "invalid utf8", format: ".srt", input: "\xff", count: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cues, err := parseCaptions(tt.input, tt.format)
			if tt.count == 0 {
				if err == nil {
					t.Fatal("expected error")
				}

				return
			}

			if err != nil || len(cues) != tt.count {
				t.Fatalf("cues=%v err=%v", cues, err)
			}

			if cues[0].Start != time.Second {
				t.Fatalf("start=%v", cues[0].Start)
			}
		})
	}
}

func TestTimestamp(t *testing.T) {
	for _, s := range []string{"00:60.000", "00:01.00", "-1:00.000", "999999999999999999:00:00.000", "2562048:00:00.000", "00:00:60.000", "0:01.000", "00:01,000"} {
		if _, err := parseTimestamp(s, true); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}

func TestCaptionFixtures(t *testing.T) {
	for _, path := range []string{"testdata/sample.srt", "testdata/sample.vtt"} {
		cues, err := readCaptions(path)
		if err != nil {
			t.Fatal(err)
		}

		if coverage(cues, 0, 10*time.Second) != 7*time.Second {
			t.Fatal("expected 7 seconds")
		}

		if captionText(cues) != "Hello there.\nWelcome back." {
			t.Fatal("unexpected text")
		}
	}

	if _, err := readCaptions("file.mp4"); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatal(err)
	}
}
