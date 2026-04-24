# Worklog 

Small Go command-line timeblock tracker that:

- asks what you want to achieve
- reads optional settings from `~/.worklog/config.toml`
- shows your logged minutes today
- lets you choose configured time sets or a custom number of minutes
- prints a live countdown
- sends a desktop notification when the block ends
- asks what the result was and saves the session to the configured entries directory as `YYYY-MM-DD.json`
- if interrupted, asks why and saves the session with interruption metadata for later resume
- can resume the most recent interrupted timeblock using the saved goal and remaining minutes

## Usage

```bash
go run . help
go run . add 30 "Write release notes"
go run . add 30 "Write release notes" "Finished draft for v1.2"
go run . start
go run . resume
go run . status
go run . summary week
go run . summary week deploy
go run . summary 2026-04-01 2026-04-12
go run . summary 2026-04-01 2026-04-12 deploy
go build .
./worklog help
./worklog add 30 "Write release notes"
./worklog add 30 "Write release notes" "Finished draft for v1.2"
./worklog start
./worklog resume
./worklog status
./worklog summary month
./worklog summary month deploy
./worklog summary 2026-04-01 2026-04-12
./worklog summary 2026-04-01 2026-04-12 deploy
```

`add` accepts:

- a required positive integer minute count
- a required `goal`
- an optional `result`; if omitted, the CLI stores the `goal` text as the `result`

`summary` accepts:

- `week`, `month`, or `year`
- two inclusive ISO dates in `YYYY-MM-DD` format
- an optional text filter that matches `goal` or `result`

## Config

If `~/.worklog/config.toml` does not exist, the CLI uses:

```toml
time_sets = [30, 60, 90]
entries_dir = "entries"
```

Example custom config:

```toml
time_sets = [25, 50, 75, 100]
entries_dir = "/path/to/worklog-entries"
```

`entries_dir` is optional. If it is a relative path, it is resolved relative to `~/.worklog`. If it is absolute, it is used as-is.
