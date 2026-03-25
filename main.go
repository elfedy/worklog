package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gen2brain/beeep"
)

type timeblockEntry struct {
	Goal            string    `json:"goal"`
	Achievement     string    `json:"achievement"`
	DurationMinutes int       `json:"duration_minutes"`
	DurationLabel   string    `json:"duration_label"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Timeblock")

	fmt.Print("What do you want to achieve in this timeblock? ")
	goal, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read goal: %v\n", err)
		os.Exit(1)
	}

	goal = strings.TrimSpace(goal)
	if goal == "" {
		fmt.Fprintln(os.Stderr, "goal cannot be empty")
		os.Exit(1)
	}

	durationOptions := []int{30, 45, 60, 90}

	fmt.Println()
	fmt.Println("How long do you want to focus?")
	for index, minutes := range durationOptions {
		fmt.Printf("%d. %d minutes\n", index+1, minutes)
	}
	fmt.Printf("%d. Custom\n", len(durationOptions)+1)
	fmt.Print("Choose an option: ")

	choiceInput, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read duration choice: %v\n", err)
		os.Exit(1)
	}

	choice, err := strconv.Atoi(strings.TrimSpace(choiceInput))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid choice %q\n", strings.TrimSpace(choiceInput))
		os.Exit(1)
	}

	durationMinutes := 0
	durationLabel := ""

	if choice >= 1 && choice <= len(durationOptions) {
		durationMinutes = durationOptions[choice-1]
		durationLabel = fmt.Sprintf("%d minutes", durationMinutes)
	} else if choice == len(durationOptions)+1 {
		fmt.Print("Enter custom minutes: ")

		customInput, customErr := reader.ReadString('\n')
		if customErr != nil {
			fmt.Fprintf(os.Stderr, "failed to read custom minutes: %v\n", customErr)
			os.Exit(1)
		}

		customMinutes, customParseErr := strconv.Atoi(strings.TrimSpace(customInput))
		if customParseErr != nil {
			fmt.Fprintf(os.Stderr, "invalid custom minutes %q\n", strings.TrimSpace(customInput))
			os.Exit(1)
		}

		if customMinutes <= 0 {
			fmt.Fprintln(os.Stderr, "custom minutes must be greater than zero")
			os.Exit(1)
		}

		durationMinutes = customMinutes
		durationLabel = fmt.Sprintf("%d minutes", durationMinutes)
	} else {
		fmt.Fprintf(os.Stderr, "choice must be between 1 and %d\n", len(durationOptions)+1)
		os.Exit(1)
	}

	duration := time.Duration(durationMinutes) * time.Minute
	startedAt := time.Now()
	endAt := startedAt.Add(duration)
	ticker := time.NewTicker(time.Second)
	done := time.NewTimer(duration)
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)

	defer ticker.Stop()
	defer done.Stop()
	defer signal.Stop(interrupts)

	fmt.Println()
	fmt.Printf("Starting %s timeblock for: %s\n", durationLabel, goal)
	fmt.Printf("Ends at %s\n", endAt.Format("15:04"))
	fmt.Print("\rRemaining: ", duration.Round(time.Second))

	for {
		select {
		case <-interrupts:
			fmt.Print("\rTimeblock cancelled.                 \n")
			os.Exit(130)
		case <-ticker.C:
			remaining := time.Until(endAt).Round(time.Second)
			if remaining < 0 {
				remaining = 0
			}

			fmt.Printf("\rRemaining: %-24s", remaining)
		case <-done.C:
			fmt.Print("\rRemaining: 0s                      \n")

			notifyErr := beeep.Notify("Timeblock finished", goal, "")
			if notifyErr != nil {
				fmt.Fprintf(os.Stderr, "notification failed: %v\n", notifyErr)
			}

			fmt.Println()
			fmt.Print("What did you achieve? ")

			achievement, readErr := reader.ReadString('\n')
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "failed to read achievement: %v\n", readErr)
				os.Exit(1)
			}

			achievement = strings.TrimSpace(achievement)
			if achievement == "" {
				fmt.Fprintln(os.Stderr, "achievement cannot be empty")
				os.Exit(1)
			}

			entry := timeblockEntry{
				Goal:            goal,
				Achievement:     achievement,
				DurationMinutes: durationMinutes,
				DurationLabel:   durationLabel,
				StartedAt:       startedAt,
				EndedAt:         time.Now(),
			}

			homeDir, homeErr := os.UserHomeDir()
			if homeErr != nil {
				fmt.Fprintf(os.Stderr, "failed to resolve home directory: %v\n", homeErr)
				os.Exit(1)
			}

			worklogDir := filepath.Join(homeDir, ".worklog")
			entriesDir := filepath.Join(worklogDir, "entries")
			mkdirErr := os.MkdirAll(entriesDir, 0755)
			if mkdirErr != nil {
				fmt.Fprintf(os.Stderr, "failed to create %s: %v\n", entriesDir, mkdirErr)
				os.Exit(1)
			}

			entriesPath := filepath.Join(entriesDir, startedAt.Format("2006-01-02")+".json")
			entries := []timeblockEntry{}

			entriesBytes, readEntriesErr := os.ReadFile(entriesPath)
			if readEntriesErr == nil && len(strings.TrimSpace(string(entriesBytes))) > 0 {
				unmarshalErr := json.Unmarshal(entriesBytes, &entries)
				if unmarshalErr != nil {
					fmt.Fprintf(os.Stderr, "failed to parse %s: %v\n", entriesPath, unmarshalErr)
					os.Exit(1)
				}
			} else if readEntriesErr != nil && !os.IsNotExist(readEntriesErr) {
				fmt.Fprintf(os.Stderr, "failed to read %s: %v\n", entriesPath, readEntriesErr)
				os.Exit(1)
			}

			entries = append(entries, entry)

			encodedEntries, marshalErr := json.MarshalIndent(entries, "", "  ")
			if marshalErr != nil {
				fmt.Fprintf(os.Stderr, "failed to encode entries: %v\n", marshalErr)
				os.Exit(1)
			}

			writeErr := os.WriteFile(entriesPath, append(encodedEntries, '\n'), 0644)
			if writeErr != nil {
				fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", entriesPath, writeErr)
				os.Exit(1)
			}

			fmt.Printf("Saved entry to %s\n", entriesPath)
			return
		}
	}
}
