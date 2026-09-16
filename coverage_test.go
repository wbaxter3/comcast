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
		{"gaps", [][2]int{{0, 3}, {5, 9}}, 7},
		{"overlap unsorted duplicate", [][2]int{{5, 9}, {0, 6}, {5, 9}}, 9},
		{"clipped", [][2]int{{0, 3}, {9, 15}}, 4},
		{"outside", [][2]int{{11, 15}}, 0},
		{"touching", [][2]int{{0, 5}, {5, 10}}, 10},
		{"nested", [][2]int{{0, 10}, {2, 3}}, 10},
		{"empty", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cues []Cue
			for _, s := range tt.spans {
				cues = append(cues, Cue{time.Duration(s[0]) * time.Second, time.Duration(s[1]) * time.Second, "text"})
			}

			if got := coverage(cues, 0, 10*time.Second); got != time.Duration(tt.want)*time.Second {
				t.Fatalf("got %v want %ds", got, tt.want)
			}
		})
	}

	if got := coverage([]Cue{{0, 10 * time.Second, ""}}, 0, 10*time.Second); got != 0 {
		t.Fatal(got)
	}

	if got := coverage([]Cue{{0, 10 * time.Second, "text"}}, 2*time.Second, 8*time.Second); got != 6*time.Second {
		t.Fatal(got)
	}
}
