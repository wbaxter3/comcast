package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"time"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run keeps process exit and global streams out of the application logic so tests
// can inspect results, diagnostics, and exit codes independently.
func run(args []string, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintf(stderr, "captioncheck: %v\n", err); return 1 }
	flags := flag.NewFlagSet("captioncheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	// NaN distinguishes an omitted numeric flag from a valid explicit zero.
	start := flags.Float64("start", math.NaN(), "start of window in seconds (required)")
	end := flags.Float64("end", math.NaN(), "end of window in seconds (required)")
	required := flags.Float64("coverage", math.NaN(), "required percentage, 0–100 (required)")
	endpoint := flags.String("endpoint", "", "language service HTTP(S) URL (required)")
	timeout := flags.Duration("timeout", 10*time.Second, "language service timeout")

	flags.Usage = func() { fmt.Fprintln(stderr, "Usage: captioncheck <flags> captions-filepath"); flags.PrintDefaults() }
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}

		return 1
	}

	// Reject nonfinite values and overflow before converting seconds to nanoseconds.
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	if !finite(*start) || !finite(*end) || *start < 0 || *end <= *start || *end >= float64(math.MaxInt64)/float64(time.Second) {
		return fail(fmt.Errorf("require 0 <= -start < -end within the supported duration range"))
	}

	if !finite(*required) || *required < 0 || *required > 100 {
		return fail(fmt.Errorf("-coverage must be between 0 and 100"))
	}

	u, err := url.Parse(*endpoint)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fail(fmt.Errorf("-endpoint must be an HTTP(S) URL"))
	}

	if *timeout <= 0 {
		return fail(fmt.Errorf("-timeout must be positive"))
	}

	if flags.NArg() != 1 {
		return fail(fmt.Errorf("provide exactly one caption file after the flags"))
	}

	// Distinct decimal inputs can truncate to the same nanosecond.
	a, b := time.Duration(*start*float64(time.Second)), time.Duration(*end*float64(time.Second))
	if b <= a {
		return fail(fmt.Errorf("time window must be at least one nanosecond"))
	}

	cues, err := readCaptions(flags.Arg(0))
	if err != nil {
		return fail(err)
	}

	covered := coverage(cues, a, b)
	actual := float64(covered) * 100 / float64(b-a)

	// Language applies to the whole file, even if coverage in the window fails.
	lang, err := detectLanguage(*endpoint, captionText(cues), *timeout)
	if err != nil {
		return fail(err)
	}
	// Wait until both checks finish before emitting any validation results.
	results := []any{}
	// Compare before division to avoid rounding an exact threshold down (e.g. 29%).
	if float64(covered)*100 < *required*float64(b-a) {
		results = append(results, struct {
			Type     string  `json:"type"`
			Required float64 `json:"required_percent"`
			Actual   float64 `json:"actual_percent"`
			Covered  float64 `json:"covered_seconds"`
			Window   float64 `json:"window_seconds"`
			Start    float64 `json:"start_seconds"`
			End      float64 `json:"end_seconds"`
		}{"caption_coverage", *required, actual, covered.Seconds(), (b - a).Seconds(), *start, *end})
	}

	if lang != "en-US" {
		results = append(results, struct {
			Type     string `json:"type"`
			Expected string `json:"expected"`
			Actual   string `json:"actual"`
		}{"incorrect_language", "en-US", lang})
	}

	// Encode adds a newline after each object; no failures means no stdout output.
	for _, result := range results {
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			return fail(fmt.Errorf("write results: %w", err))
		}
	}

	// Failed validations are completed checks, not failures to run the program.
	return 0
}
