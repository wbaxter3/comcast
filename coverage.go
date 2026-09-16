package main

import (
	"sort"
	"time"
)

// coverage returns the union of nonempty cue intervals clipped to the window.
func coverage(cues []Cue, start, end time.Duration) time.Duration {
	// Clip copies so sorting for coverage does not change the original text order.
	intervals := make([]Cue, 0, len(cues))
	for _, cue := range cues {
		if cue.Text == "" {
			continue
		}

		if cue.Start < start {
			cue.Start = start
		}

		if cue.End > end {
			cue.End = end
		}

		// Cues entirely outside the window have no positive interval after clipping.
		if cue.End > cue.Start {
			intervals = append(intervals, cue)
		}
	}

	sort.Slice(intervals, func(i, j int) bool { return intervals[i].Start < intervals[j].Start })

	// With starts sorted, until is the furthest end already counted. Add only the
	// portion beyond it, which handles overlaps, duplicates, and nested cues alike.
	var total, until time.Duration

	for _, cue := range intervals {
		a := cue.Start
		if a < until {
			a = until
		}

		if cue.End > a {
			total += cue.End - a
			until = cue.End
		}
	}

	return total
}
