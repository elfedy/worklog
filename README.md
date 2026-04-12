# Timer CLI

Small Go command-line timeblock tracker that:

- asks what you want to achieve
- reads optional settings from `~/.worklog/config.toml`
- shows your logged minutes today against your configured daily target
- lets you choose configured time sets or a custom number of minutes
- prints a live countdown
- sends a desktop notification when the block ends
- asks what the result was and saves the session to `~/.worklog/entries/YYYY-MM-DD.json`
- if interrupted, asks why and saves the session with interruption metadata for later resume
- can resume the most recent interrupted timeblock using the saved goal and remaining minutes

## Usage

```bash
go run . help
go run . start
go run . resume
go run . status
go build .
./worklog help
./worklog start
./worklog resume
./worklog status
```

## Config

If `~/.worklog/config.toml` does not exist, the CLI uses:

```toml
minutes_per_day = 300
time_sets = [30, 60, 90]
```

Example custom config:

```toml
minutes_per_day = 240
time_sets = [25, 50, 75, 100]
```
