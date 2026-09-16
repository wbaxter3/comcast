# Captioncheck

A small, standard-library-only Go CLI that validates caption coverage and US
English language. Supports UTF-8 `.srt` and `.vtt` files. No video is needed.

See [the logical flow diagrams](FLOW.md) for a visual walkthrough of the code.

## Build and run

Requires Go 1.22 or later.

```sh
go build -o captioncheck .
./captioncheck -start 0 -end 10 -coverage 80 \
  -endpoint http://localhost:8080/language testdata/sample.vtt
```

The language endpoint must be supplied separately; this project does not build a
language detection service. `go run .` can replace `./captioncheck`.

| Flag | Meaning |
| --- | --- |
| `-start` | Required start time, in decimal seconds, at least 0 |
| `-end` | Required end time, in decimal seconds, greater than start |
| `-coverage` | Required percentage, from 0 to 100 |
| `-endpoint` | Required HTTP(S) language detection URL |
| `-timeout` | HTTP timeout as a Go duration, default `10s` |

Put flags before the single file path. Use `-help` for usage.

## What it checks

Coverage is the percentage of the requested time window containing at least one
nonempty caption. Intervals are clipped to the window and merged so overlaps
count only once. The two sample files each cover 7 of the first 10 seconds, or
70%. This measures caption presence, not speech accuracy or synchronization.

The tool sends all usable cue text from the file, in file order, as one POST
request with `Content-Type: text/plain; charset=utf-8`. Multiline text is retained
and captions are separated by newlines. Embedded caption markup is preserved.
The endpoint must return a 2xx response with a JSON object such as:

```json
{"lang":"en-US"}
```

Only exact `en-US` passes. Extra JSON fields are ignored. Missing/empty/non-string
`lang`, invalid JSON, responses larger than 1 MiB, HTTP errors, and timeouts are
operational errors. Redirects are not followed and requests are not retried.
The language check still runs when coverage fails.

## Results and exit codes

Successful validations produce no output. Failed validations produce JSON lines
on stdout, one per failed check, and **exit 0**:

```json
{"type":"caption_coverage","required_percent":80,"actual_percent":70,"covered_seconds":7,"window_seconds":10,"start_seconds":0,"end_seconds":10}
{"type":"incorrect_language","expected":"en-US","actual":"en-GB"}
```

Bad arguments, unsupported file extensions, unreadable/malformed files, and
service failures produce an error on stderr and **exit 1**. Results are held
until both checks complete, so an operational failure produces no partial report
(except that an output write failure can interrupt reporting itself). Help exits
0 and prints usage on stderr.

## Caption support and assumptions

- The case-insensitive filename extension selects the parser; no format flag.
- SRT: numeric cue identifiers, `HH:MM:SS,mmm` timestamps, multiline text.
- WebVTT: required `WEBVTT` header, optional header metadata and cue identifiers,
  `MM:SS.mmm` or `HH:MM:SS.mmm` timestamps, trailing cue settings. `NOTE`, `STYLE`,
  and `REGION` blocks are skipped. Cue settings are ignored, not validated.
- UTF-8 BOM, LF, CRLF, and CR line endings are supported.
- Cue blocks must be separated by blank lines. Cue times must be nonnegative and
  end after they start. Overlapping and out-of-order cues are allowed.
- Empty-text cues do not count; a file without any usable cues is an input error.
- Invalid UTF-8, embedded NULs, and malformed cues are input errors, not failed
  coverage/language validations.
- Files are read into memory. This is a practical text-caption parser, not a full
  standards-conformance checker or renderer.

## Tests

```sh
go test ./...
go vet ./...
```

Tests use local fixtures and `httptest` servers, so no live language service is
required. They cover parsing, interval merging, threshold boundaries, HTTP
contracts/failures, JSON output, and exit codes.

## Formatting and linting

Install [golangci-lint v2.13.2](https://golangci-lint.run/docs/welcome/install/local/)
and put its binary on your PATH. This is a development tool only; the application
still has no third-party dependencies.

```sh
make fmt    # Apply whitespace fixes and gofmt
make lint   # Check whitespace and formatting
make test   # Run the Go tests
```

`.golangci.yml` enables `wsl_v5` to enforce blank lines between logical statement
groups and `gofmt` for standard Go formatting. These rules cover both source and
test files.

## Docker

```sh
docker build -t captioncheck .
docker run --rm -v "$PWD/testdata:/captions:ro" captioncheck \
  -start 0 -end 10 -coverage 80 \
  -endpoint http://host.docker.internal:8080/language /captions/sample.vtt
```

This example reaches a service on the Docker Desktop host. On Linux, use a
reachable service URL or configure host networking/name resolution as appropriate.
`localhost` inside the container refers to the container itself. Ensure mounted
caption files are readable by the container's non-root user.

The two-stage build produces a static binary in a scratch image with CA
certificates for HTTPS.

## Structure

`main.go` handles flags and output, `captions.go` parses files, `coverage.go`
merges intervals, and `language.go` calls the service. Everything is one package
with plain functions and structs. No third-party dependencies.

`PLAN.md` records the original plan; `PROMPTS.md` records the user's project
prompts. Future work, if needed: content-based format detection, caption markup
normalization, and streaming very large files.
