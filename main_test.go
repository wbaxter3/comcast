package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name, percent, response string
		types                   []string
		code                    int
	}{
		{"passes threshold", "70", `{"lang":"en-US"}`, nil, 0},
		{"zero required", "0", `{"lang":"en-US"}`, nil, 0},
		{"coverage fails", "80", `{"lang":"en-US"}`, []string{"caption_coverage"}, 0},
		{"full required", "100", `{"lang":"en-US"}`, []string{"caption_coverage"}, 0},
		{"language fails", "70", `{"lang":"es-ES"}`, []string{"incorrect_language"}, 0},
		{"both fail", "80", `{"lang":"en-GB"}`, []string{"caption_coverage", "incorrect_language"}, 0},
		{"service fails no partial results", "80", `bad`, nil, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, tt.response) }))
			defer server.Close()

			var out, errOut bytes.Buffer

			code := run([]string{"-start", "0", "-end", "10", "-coverage", tt.percent, "-endpoint", server.URL, "testdata/sample.srt"}, &out, &errOut)
			if code != tt.code || (errOut.Len() > 0) != (tt.code == 1) {
				t.Fatalf("code=%d stderr=%s", code, &errOut)
			}

			decoder := json.NewDecoder(&out)

			for _, want := range tt.types {
				var got map[string]any
				if err := decoder.Decode(&got); err != nil {
					t.Fatal(err)
				}

				if got["type"] != want {
					t.Fatalf("got %v", got)
				}

				if want == "caption_coverage" && got["actual_percent"] != float64(70) {
					t.Fatal(got)
				}
			}

			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				t.Fatalf("unexpected output %v err=%v", extra, err)
			}
		})
	}
}

func TestBadArguments(t *testing.T) {
	base := []string{"-start", "0", "-end", "10", "-coverage", "80", "-endpoint", "http://localhost:1"}

	for _, tail := range [][]string{
		{"-start", "NaN"}, {"-end", "Inf"}, {"-end", "0"}, {"-end", "1e20"},
		{"-coverage", "101"}, {"-coverage", "NaN"}, {"-endpoint", "ftp://example.com"},
		{"-timeout", "0s"}, {"-unknown"}, {"missing.srt"}, {"file.mp4"}, {},
	} {
		var out, errOut bytes.Buffer

		args := append(append([]string{}, base...), tail...)
		if code := run(args, &out, &errOut); code != 1 || out.Len() != 0 || errOut.Len() == 0 {
			t.Fatalf("args=%v code=%d out=%s err=%s", args, code, &out, &errOut)
		}
	}

	var out, errOut bytes.Buffer
	if run([]string{"-help"}, &out, &errOut) != 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "Usage:") {
		t.Fatal("bad help")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken output") }

func TestOutputFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"lang":"es-ES"}`) }))
	defer server.Close()

	var stderr bytes.Buffer
	if code := run([]string{"-start", "0", "-end", "10", "-coverage", "0", "-endpoint", server.URL, "testdata/sample.vtt"}, brokenWriter{}, &stderr); code != 1 {
		t.Fatal(code)
	}
}
