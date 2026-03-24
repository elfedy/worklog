# Timer CLI

Small Go command-line timer that prints a live countdown and sends a desktop notification when it finishes.

## Usage

```bash
go run . start 25m
go run . start --title "Break over" --message "Back to work." 5m
```

Durations use Go's standard format, for example `10s`, `5m`, or `1h30m`.
