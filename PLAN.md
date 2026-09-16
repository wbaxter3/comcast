# Caption validator implementation plan

## Approach

Build one small Go CLI using only the standard library. Keep everything in one
`package main`, split by responsibility. No framework, third-party caption parser,
configuration files, database, concurrency, or interface hierarchy.

Target the assessment's 2–3 hour scope. Support ordinary text SRT and WebVTT files;
document the supported syntax instead of attempting a complete caption rendering
or standards-conformance engine.

## CLI contract

```sh
captioncheck -start 0 -end 60 -coverage 80 -endpoint http://localhost:8080/language captions.vtt
```

- `-start` and `-end`: required, finite decimal seconds; `0 <= start < end`.
- `-coverage`: required, finite percentage from 0 through 100.
- `-endpoint`: required HTTP or HTTPS URL for language detection.
- `-timeout`: optional HTTP timeout, default `10s`.
- Exactly one positional file path; flags precede the path, matching Go's `flag` package.
- No format flag. Select the parser from a case-insensitive `.srt` or `.vtt`
  extension, then validate its contents. Reject other extensions with exit code 1.
  Document this filename-based detection as a deliberate simplification.

Use `flag.FlagSet` and a `run(args, stdout, stderr) int` function so CLI behavior
can be tested without starting a subprocess. Only `main` calls `os.Exit`.

## Data and file layout

```text
go.mod
main.go              # Flags, orchestration, output, exit status
captions.go          # Cue type, SRT/WebVTT parsing, timestamp helpers
coverage.go          # Clip and merge cue intervals
language.go          # HTTP request and response handling
*_test.go            # Tests beside the corresponding implementation
testdata/            # Small, readable caption fixtures
Dockerfile
README.md
```

Represent each cue with `Start`, `End` (`time.Duration`) and `Text` (`string`).
Use plain functions and concrete structs. Add abstractions only if the actual
implementation demonstrates a need for them.

## Parsing

1. Read the file with `os.ReadFile`; normalize CRLF and a leading UTF-8 BOM.
   Reject invalid UTF-8 and clearly binary input such as embedded NUL bytes.
2. Split into caption blocks and dispatch to a small format-specific parser.
3. For SRT, handle numeric cue identifiers, timestamp pairs with comma
   milliseconds, and multiline text.
4. For WebVTT, require the `WEBVTT` header; handle optional cue identifiers,
   dot milliseconds, timestamps with or without hours, and trailing cue settings.
   Skip header metadata and `NOTE`, `STYLE`, and `REGION` blocks.
5. Validate timestamp components and require cue end to be after cue start.
   Preserve overlapping and out-of-order cues for the coverage calculation.
6. Return actionable parse errors with a line or block number. Do not silently
   skip malformed cues.

Only cues with non-whitespace text count toward coverage. An empty caption file
or a file with no usable cues is an input error. Keep markup handling minimal:
join cue payload text with newlines and send it as plaintext; document that
embedded caption markup is preserved. Full markup normalization is out of scope.

## Coverage validation

- Clip each usable cue interval to `[start, end]` and discard empty intersections.
- Sort intervals by start time, then merge overlapping or touching intervals.
- Sum merged durations so overlapping captions are never counted twice.
- Compute `actual = covered / (end - start) * 100`.
- Fail only when `actual < required`; compare before rounding for display.

On failure, print one JSON object on stdout:

```json
{"type":"caption_coverage","required_percent":80,"actual_percent":65,"covered_seconds":39,"window_seconds":60,"start_seconds":0,"end_seconds":60}
```

This is a simple `O(n log n)` algorithm with easy-to-test boundary behavior.

## Language validation

- Join the usable text from the entire file, in file order. The time window only
  controls coverage; the assignment asks to send the text of the captions.
- POST it to the configured endpoint with `Content-Type: text/plain; charset=utf-8`.
- Use `net/http` with the configured timeout. No retries.
- Require a successful 2xx status and one valid JSON response object containing a
  nonempty string `lang`. Ignore additional fields for compatibility.
- Bound the response body, for example to 1 MiB, and close it on every path.
- Accept only exact `en-US`.

On a different language, print:

```json
{"type":"incorrect_language","expected":"en-US","actual":"en-GB"}
```

Run language validation even when coverage fails, so a successful run can report
both validation failures. Use `httptest.Server` in tests; no external service is
needed for automated verification.

## Output and errors

| Outcome | stdout | stderr | Exit |
| --- | --- | --- | --- |
| Both validations pass | Empty | Empty | 0 |
| Coverage and/or language fails | One JSON object per failed validation | Empty | 0 |
| Bad arguments, unsupported file, read/parse error | Empty | Clear error | 1 |
| HTTP failure, timeout, invalid server response | Empty | Clear error | 1 |
| Help requested | Empty | Usage | 0 |

Treat malformed caption input as a program/input error because it prevents
validation; document this assumption in the README. Collect validation results
and emit them only once both checks complete, so an operational failure does not
produce a partial validation report. Use `encoding/json.Encoder` for JSON lines
and handle output write errors. Never log caption contents or print stack traces.

## Tests

Use table-driven tests with `testing` and small local fixtures.

- Parsing: both formats, BOM/CRLF, multiline text, WebVTT identifiers/settings and
  non-cue blocks, malformed timestamps, binary input, and empty input.
- Coverage: exact threshold, gaps, overlaps, duplicate and unsorted cues, clipping
  at both boundaries, cues outside the window, and 0%/100% requirements.
- HTTP: exact `en-US`, another language, missing/invalid `lang`, malformed JSON,
  non-2xx responses, excessive response size, and timeout. Verify request method,
  content type, and body.
- CLI: invalid flags and file types exit 1; valid captions are silent; validation
  failures exit 0; both failures produce two JSON lines; diagnostics use stderr.

## Build and Docker

Use a two-stage Dockerfile: a Go builder compiles with `CGO_ENABLED=0`, and a
minimal runtime image contains the binary and CA certificates for HTTPS. Run as
a non-root user and use the binary as the entrypoint. Pin concrete image versions
when implementing.

Document local and Docker usage, including mounting caption files read-only:

```sh
go test ./...
go vet ./...
go build -o captioncheck .
docker build -t captioncheck .
docker run --rm -v "$PWD/testdata:/captions:ro" captioncheck \
  -start 0 -end 60 -coverage 80 \
  -endpoint https://example.com/language /captions/sample.vtt
```

The endpoint above is a placeholder; the README should explain that the service
must be reachable from inside the container.

## Implementation order

1. Establish CLI options, error handling, and shared cue representation.
2. Implement parsers with fixtures and focused parsing tests.
3. Implement interval coverage with table-driven tests.
4. Add the HTTP language check with `httptest` coverage.
5. Wire JSON results and verify the CLI contract end to end.
6. Add Dockerfile and README; run tests, vet, build, and a container smoke test.

Defer content-based format sniffing, full caption markup normalization, streaming
large files, retries, and additional caption formats. List these as possible
extensions only; none is needed for the initial submission.
