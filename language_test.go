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
		{name: "english", body: `{"lang":"en-US"}`, want: "en-US", status: 200, bad: false},
		{name: "other", body: `{"lang":"en-GB"}`, want: "en-GB", status: 200, bad: false},
		{name: "extra fields", body: `{"lang":"en-US","score":1}`, want: "en-US", status: 200, bad: false},
		{name: "missing", body: `{}`, want: "", status: 200, bad: true},
		{name: "wrong type", body: `{"lang":123}`, want: "", status: 200, bad: true},
		{name: "empty", body: `{"lang":" "}`, want: "", status: 200, bad: true},
		{name: "null", body: `null`, want: "", status: 200, bad: true},
		{name: "malformed", body: `{`, want: "", status: 200, bad: true},
		{name: "trailing json", body: `{"lang":"en-US"}{}`, want: "", status: 200, bad: true},
		{name: "server error", body: `{}`, want: "", status: 500, bad: true},
		{name: "redirect", body: `{}`, want: "", status: 302, bad: true},
		{name: "too big", body: strings.Repeat(" ", (1<<20)+1), want: "", status: 200, bad: true},
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
