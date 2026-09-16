package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLanguage(t *testing.T) {
	tests := []struct {
		name, body, want string
		status           int
		bad              bool
	}{
		{"english", `{"lang":"en-US"}`, "en-US", 200, false},
		{"other", `{"lang":"en-GB"}`, "en-GB", 200, false},
		{"extra fields", `{"lang":"en-US","score":1}`, "en-US", 200, false},
		{"missing", `{}`, "", 200, true},
		{"wrong type", `{"lang":123}`, "", 200, true},
		{"empty", `{"lang":" "}`, "", 200, true},
		{"null", `null`, "", 200, true},
		{"malformed", `{`, "", 200, true},
		{"trailing json", `{"lang":"en-US"}{}`, "", 200, true},
		{"server error", `{}`, "", 500, true},
		{"redirect", `{}`, "", 302, true},
		{"too big", strings.Repeat(" ", (1<<20)+1), "", 200, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if r.Method != "POST" || r.Header.Get("Content-Type") != "text/plain; charset=utf-8" || string(body) != "Hello\nworld" {
					t.Error("incorrect request")
				}

				w.WriteHeader(tt.status)
				io.WriteString(w, tt.body)
			}))
			defer server.Close()

			got, err := detectLanguage(server.URL, "Hello\nworld", time.Second)
			if (err != nil) != tt.bad || got != tt.want {
				t.Fatalf("got %q err=%v", got, err)
			}
		})
	}
}

func TestLanguageTimeout(t *testing.T) {
	release := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer server.Close()
	defer close(release)

	if _, err := detectLanguage(server.URL, "hello", 20*time.Millisecond); err == nil {
		t.Fatal("expected timeout")
	}
}
