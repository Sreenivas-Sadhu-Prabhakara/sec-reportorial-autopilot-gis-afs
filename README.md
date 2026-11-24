# SEC Reportorial Calendar — General Information Sheet &amp; Audited Financial Statements

A single Go binary that tracks the reportorial deadlines of a Philippine SEC-registered
entity: the **General Information Sheet**, whose deadline is counted from that company's
own annual meeting, and the **audited financial statements**, due on the schedule the
Commission publishes for the season. It runs on your own machine, serves a small
interface on `127.0.0.1`, stores everything in one JSON file beside you, and makes no
outbound network connections of any kind.

**It records and reminds. It does not file anything with the Securities and Exchange
Commission.** There is no eFAST integration, no portal connection, and nothing is
submitted or sent anywhere. Every filing is still made by you, in eFAST.

## What it does

- Keeps a register of companies: registered name, SEC registration number, kind of
  registrant, filer class, fiscal year end and the way its GIS clock is reckoned.
- Derives the **GIS deadline from the company's own annual meeting date** — thirty
  calendar days from the date the meeting was actually held, or from the anniversary of
  the SEC licence for a foreign corporation. Until a meeting is recorded there is no
  deadline, and the tool says so rather than inventing one.
- Applies the **published AFS schedule** for a 31 December fiscal year end, and the
  calendar-day rules for every other year end, each carrying the circular and section
  it came from.
- Shows the **last working day** you can actually file on, because a submission made on
  a Saturday, Sunday or holiday counts as filed on the next working day.
- Escalates reminders — plan, prepare, due soon, urgent, final call, due today, overdue
  — on a ladder you can change.
- Records **what was filed and the reference it came back with**, so the proof of
  filing sits beside the obligation.
- Counts missed filings in the trailing five years against the statutory ground for
  delinquent status, and names what follows it.
- Exports the whole calendar as CSV, including how each date was worked out.

## Who it's for

Philippine corporations, One Person Corporations, non-stock corporations, registered
partnerships, and the branch, representative and regional offices of foreign
corporations — and the bookkeepers, corporate secretaries and small accounting firms who
carry several of them at once.

## The regulation

**Section 177 of the Revised Corporation Code of the Philippines (Republic Act No.
11232)** requires every corporation, domestic or foreign, doing business in the
Philippines to submit its annual financial statements and General Information Sheet
within the period the Commission prescribes. The Commission prescribes that period in a
filing-schedule memorandum circular each year. This build carries the schedule from
**SEC Memorandum Circular No. 9, Series of 2026** (issued 11 February 2026), as revised
by the **SEC Notice dated 14 April 2026**.

## Statutory figures used, and where they come from

Every figure below is a named constant in `internal/secfiling/schedule.go`, in one
place, and every one of them is overridable from the Settings tab without rebuilding.

| Rule | Value | Source |
| --- | --- | --- |
| General Information Sheet | 30 calendar days from the actual annual meeting, or from the anniversary of the SEC licence for a foreign corporation | SEC MC No. 9, s. 2026, Sec. 5 |
| AFS, 31 December 2025 fiscal year end | **15 June 2026**, moved from 29 May 2026 | SEC MC No. 9, s. 2026, Sec. 1, as revised by the SEC Notice dated 14 April 2026 |
| SEC Form 52-AR with AFS (brokers and dealers), 31 December 2025 | **15 May 2026**, moved from 30 April 2026 | same |
| Annual Report on SEC Form 17-A with AFS, 31 December 2025 | **15 May 2026**, moved from 15 April 2026 | same |
| AFS, other fiscal year ends | 120 calendar days from the end of the fiscal year | SEC MC No. 9, s. 2026, Sec. 2(a) |
| SEC Form 52-AR, other fiscal year ends | 110 calendar days after the close of the fiscal year | SEC MC No. 9, s. 2026, Sec. 2(a), citing Rule 52.1.5.2 of the SRC-IRR |
| Form 17-A filers (SRC Sec. 17.2) | 105 calendar days after the end of the fiscal year | SEC MC No. 9, s. 2026, Sec. 2(b) |
| Filing on a non-working day | counts as filed on the next working day | SEC MC No. 9, s. 2026, Sec. 9 |
| Late filing becomes a non-filing | more than one year late (domestic); beyond 60 calendar days (foreign, late beyond 30) | SEC MC No. 6, s. 2024 |
| Per-month component of the fine | suspended 14 May 2026 – 31 December 2026; basic fine unaffected | SEC MC No. 16, s. 2026 |
| Delinquent status | 3 failures to submit, consecutively or intermittently, within 5 years | RA 11232, Sec. 177; guidelines in SEC MC No. 19, s. 2023 |
| Revocation | a 6th offence after notice of delinquency | SEC MC No. 6, s. 2024 |

### The registration number does not set the AFS window

Earlier filing seasons staggered the AFS across weeks according to the **last digit of
the SEC registration number**. The circular this build was written against does not:
Section 1 of SEC MC No. 9, s. 2026 gives every corporation with a 31 December fiscal
year end one date, and the Notice of 14 April 2026 moved that one date for all of them
together. The tool records the registration number, shows its last digit, and states
plainly that the digit is not what sets your filing window. If number coding returns, it
will return as a published schedule and belongs in the override table — not in a digit
formula carried over from a season that has ended.

### Penalties are named, not computed

The tool does **not** compute a peso penalty. The scale in SEC MC No. 6, s. 2024 is
tiered by the kind of registrant, by retained earnings or fund balance and by how many
offences have been settled before, and it has been amended since — SEC MC No. 10, s.
2026 for One Person Corporations, SEC MC No. 16, s. 2026 suspending the monthly
component to 31 December 2026. A figure from a tool that has not read your tier and your
offence count would be a guess, and the Commission's assessment governs. What the tool
does instead is name the consequence: on time, late, assessed as a non-filing,
delinquency at three misses in five years, revocation at a sixth offence. If you read
your own tier off the current circular and enter the figures in Settings, it will apply
them and label the result as yours.

### Where a season is not covered

This build holds a dated schedule for one filing season. Ask it for a season it does not
hold and it returns **no date**, with an explanation and a pointer to Settings — rather
than last season's date wearing this season's label. Within the 2026 season alone the
AFS deadline moved from 29 May to 15 June; carrying a date forward is exactly how a
wrong AFS window happens, and a wrong AFS window is a penalty.

## Install and run

```sh
git clone https://github.com/sreenivas-sadhu-prabhakara/sec-reportorial-autopilot-gis-afs.git
cd sec-reportorial-autopilot-gis-afs
go build -o sec-calendar .
./sec-calendar

# or, without building first
go run .

# listens on http://127.0.0.1:8080
# other options
./sec-calendar -addr 127.0.0.1:9000 -data ~/clients/sec-register.json
```

Go 1.25 or later, standard library only. Nothing to download: the interface is compiled
into the binary.

## Where your data is stored

One JSON file, `./sec-reportorial-autopilot-gis-afs-data.json` by default, or wherever
`-data` points. It holds the companies, their meeting dates, the filings you have
recorded with their proof references, and your settings. Saves are atomic — a temporary
file in the same directory, renamed over the old one — and the file is written readable
only by you. The binary binds to loopback, has no account, no sync, no telemetry and no
update check.

## Scope and accuracy

- The dates and figures shipped here were checked against the Commission's own issuances
  on **24 November 2026**. They are indicative.
- The Commission issues a fresh filing-schedule memorandum every season and has revised
  it mid-season by notice. **Confirm the current schedule, the current scale of fines and
  your own tier with the Commission before you rely on anything here.**
- The tool counts calendar days exactly as the circulars state, including across leap
  days and month ends, and tells you how each date was derived so you can check it.
- The last-working-day guidance is this tool's reading of Section 9, shown as guidance
  and not as an extension granted by the Commission.
- It is not legal or accounting advice, and it files nothing with the Commission.

## Tests

```sh
gofmt -l .
go vet ./...
go test ./... -count=1
```

The rules live in `internal/secfiling` as pure functions with table-driven tests at every
deadline boundary: leap days, month-end clamping, the thirty-day GIS count, the
one-year line where a late filing becomes a non-filing, the foreign-corporation
thresholds, the reminder ladder and the delinquency window.

MIT licensed.
