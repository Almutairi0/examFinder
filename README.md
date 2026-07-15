# examsFinder

A fast, terminal-based CLI written in Go that scrapes, filters, and extracts specific exam schedules from a raw university exam-schedule PDF — then either prints them straight to your terminal or exports them as a `.ics` calendar file, complete with a 2-hour-before reminder alarm.

## Features

- Parses university exam-schedule PDFs directly — no manual copy-pasting
- Filter down to just the courses you care about
- Two output modes:
  - **Terminal** — color-coded, at-a-glance view with a live "days left" countdown
  - **ICS** — a single `.ics` file with all matched exams, ready to import into Google Calendar, Apple Calendar, Outlook, etc., each with a built-in 2-hour reminder
- Works two ways:
  - **Flags**, for scripting or a one-liner
  - **Interactive prompts**, if you'd rather be walked through it

## Requirements

- Go 1.22+

## Installation

```bash
git clone https://github.com/Almutairi0/examsFinder.git
cd examsFinder
go build -o examsFinder .
```

## Usage

### Non-interactive (flags)

```bash
examsFinder --file assets/exams252.pdf --courses MATH101,CS211 --format ics
```

| Flag        | Required | Description                                                        |
|-------------|----------|----------------------------------------------------------------------|
| `--file`    | Yes      | Path to the exam schedule PDF                                     |
| `--courses` | Yes      | Comma-separated course codes to match, e.g. `MATH101,CS211`         |
| `--format`  | No       | `text` (default) for terminal output, or `ics` to export a calendar file |

This produces `my_exam_schedule.ics` in the current directory when `--format ics` is used.

### Interactive

Just run it with no flags:

```bash
examsFinder
```

It will:
1. Auto-detect PDF(s) in the current folder (or let you pick, if there's more than one)
2. Ask which course codes to look for
3. Ask whether you want terminal output or an `.ics` file

## Example

```bash
$ examsFinder --file Exams.pdf --courses MATH101 --format text

------
Code:   MATH101
Title:  Calculus I
Date:   2026-08-14 (FRIDAY) [30 days left]
Time:   09:00-11:00 AM
Place:  College of Science
```

## How it works

The PDF is parsed to plain text, then scanned token-by-token: whenever a target course code is found, everything up to the next location marker (`College` / `Online`) is classified as a date, weekday, time, AM/PM marker, or exam title — no fixed column offsets, so it's resilient to minor formatting differences across PDF exports.

## License

MIT
