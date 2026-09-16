# How captioncheck works

Read the caption file, measure how long captions appear, ask the language service
what language the text uses, then report any failed checks. No video is read.

## Main flow

```mermaid
flowchart TD
    Start([Run captioncheck]) --> Args["main.go: read command-line flags"]
    Args --> Help{"Help requested?"}
    Help -->|Yes| Usage["Print usage to stderr • exit 0"]
    Help -->|No| Valid{"Flags and file path valid?"}
    Valid -->|No| Error["Print error to stderr • exit 1"]
    Valid -->|Yes| Parse["captions.go: read .srt or .vtt file<br/>Convert caption blocks to cues: start, end, text"]
    Parse --> Parsed{"Supported, readable file<br/>with valid cues and text?"}
    Parsed -->|No| Error
    Parsed -->|Yes| Coverage["coverage.go: calculate covered time<br/>inside the requested window"]
    Coverage --> Language["language.go: POST all caption text<br/>to the configured language service"]
    Language --> Response{"Successful HTTP response<br/>with valid JSON and a language?"}
    Response -->|No| Error
    Response -->|Yes| Checks["main.go: collect failed checks<br/>Coverage below target? Add caption_coverage<br/>Language differs from en-US? Add incorrect_language"]
    Checks --> Failures{"Any failed checks?"}
    Failures -->|No| Silent["Print nothing • exit 0"]
    Failures -->|Yes| Output["Print one JSON line per failed check to stdout"]
    Output --> Written{"Output written successfully?"}
    Written -->|Yes| Done["Exit 0"]
    Written -->|No| Error

    classDef success fill:#dcfce7,stroke:#15803d,color:#14532d;
    classDef error fill:#fee2e2,stroke:#b91c1c,color:#7f1d1d;
    classDef report fill:#fef3c7,stroke:#b45309,color:#78350f;
    class Silent,Done,Usage success;
    class Error error;
    class Output report;
```

**A failed check still exits 0:** the program successfully determined that the
captions did not meet the requirements. **Exit 1** means it could not complete
the job, such as an unreadable file or an unavailable language service.

The language request happens even when coverage is too low. Results are printed
only after both checks complete; a service failure produces no partial report.
An output write failure can interrupt the report itself.

## How coverage is calculated

```mermaid
flowchart LR
    Cues["Cues with text"] --> Clip["Clip times to the<br/>requested window"]
    Clip --> Discard["Discard intervals<br/>outside the window"]
    Discard --> Sort["Sort by start time"]
    Sort --> Merge["Count overlapping time<br/>only once"]
    Merge --> Sum["Sum covered seconds"]
    Sum --> Percent["Coverage % = covered time<br/>÷ window length × 100"]
```

Example: within **0–10 seconds**, captions at **0–3** and **5–9** cover
**7 seconds**, or **70%**. A requirement of 80% fails. Two captions visible at the
same time do not count twice.

The coverage window limits the time calculation. The language service receives
text from the **whole file**, preserving file order.

## Where to look in the code

| File | Job |
| --- | --- |
| [main.go](main.go) | Organize the checks, output, and exit status |
| [captions.go](captions.go) | Turn SRT/WebVTT text into shared cue structs |
| [coverage.go](coverage.go) | Count covered time without double-counting overlaps |
| [language.go](language.go) | Send caption text and read the language response |

These diagrams use Mermaid and render directly when viewing this file on GitHub.
