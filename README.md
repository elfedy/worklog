# Timer CLI

Small Go command-line timeblock tracker that:

- asks what you want to achieve
- lets you choose `30`, `45`, `60`, `90`, or a custom number of minutes
- prints a live countdown
- sends a desktop notification when the block ends
- asks what you achieved and saves the session to `~/.worklog/entries/YYYY-MM-DD.json`
- if interrupted, asks why, saves the session anyway, and appends `[INTERRUPTED]` to the achievement

## Usage

```bash
go run .
go build .
./worklog
```
