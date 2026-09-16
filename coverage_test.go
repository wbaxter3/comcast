package main

import (
	"testing"
	"time"
)

func TestCoverage(t *testing.T) {
	tests := []struct {
		name  string
		spans [][2]int
		want  int
	}{
		{name: "gaps", spans: [][2]int{{0, 3}, {5, 9}}, want: 7},
		{name: "overlap unsorted duplicate", spans: [][2]int{{5, 9}, {0, 6}, {5, 9}}, want: 9},
		{name: "clipped", spans: [][2]int{{0, 3}, {9, 15}}, want: 4},
		{name: "outside", spans: [][2]int{{11, 15}}, want: 0},
		{name: "touching", spans: [][2]int{{0, 5}, {5, 10}}, want: 10},
		{name: "nested", spans: [][2]int{{0, 10}, {2, 3}}, want: 10},
		{name: "empty", spans: nil, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cues []Cue
			for _, s := range tt.spans {
				cues = append(cues, Cue{Start: time.Duration(s[0]) * time.Second, End: time.Duration(s[1]) * time.Second, Text: "text"})
			}

			if got := coverage(cues, 0, 10*time.Second); got != time.Duration(tt.want)*time.Second {
				t.Fatalf("got %v want %ds", got, tt.want)
			}
		})
	}

	if got := coverage([]Cue{{Start: 0, End: 10 * time.Second, Text: ""}}, 0, 10*time.Second); got != 0 {
		t.Fatal(got)
	}

	if got := coverage([]Cue{{Start: 0, End: 10 * time.Second, Text: "text"}}, 2*time.Second, 8*time.Second); got != 6*time.Second {
		t.Fatal(got)
	}
}
