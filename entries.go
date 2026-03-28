package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type timeblockEntry struct {
	Goal            string    `json:"goal"`
	Result          string    `json:"result"`
	DurationMinutes int       `json:"duration_minutes"`
	DurationLabel   string    `json:"duration_label"`
	StartedAt       time.Time `json:"started_at"`
	EndedAt         time.Time `json:"ended_at"`
}

func (entry *timeblockEntry) UnmarshalJSON(data []byte) error {
	type entryJSON struct {
		Goal            string    `json:"goal"`
		Result          string    `json:"result"`
		Achievement     string    `json:"achievement"`
		DurationMinutes int       `json:"duration_minutes"`
		DurationLabel   string    `json:"duration_label"`
		StartedAt       time.Time `json:"started_at"`
		EndedAt         time.Time `json:"ended_at"`
	}

	decoded := entryJSON{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	entry.Goal = decoded.Goal
	entry.Result = decoded.Result
	if entry.Result == "" {
		entry.Result = decoded.Achievement
	}
	entry.DurationMinutes = decoded.DurationMinutes
	entry.DurationLabel = decoded.DurationLabel
	entry.StartedAt = decoded.StartedAt
	entry.EndedAt = decoded.EndedAt
	return nil
}

func readEntries(entriesPath string) ([]timeblockEntry, error) {
	entries := []timeblockEntry{}

	entriesBytes, readErr := os.ReadFile(entriesPath)
	if readErr == nil && len(strings.TrimSpace(string(entriesBytes))) > 0 {
		if unmarshalErr := json.Unmarshal(entriesBytes, &entries); unmarshalErr != nil {
			return nil, unmarshalErr
		}

		return entries, nil
	}

	if readErr != nil && !os.IsNotExist(readErr) {
		return nil, readErr
	}

	return entries, nil
}

func saveEntry(worklogDir string, startedAt time.Time, entry timeblockEntry) error {
	entriesDir := filepath.Join(worklogDir, "entries")
	if mkdirErr := os.MkdirAll(entriesDir, 0755); mkdirErr != nil {
		return fmt.Errorf("failed to create %s: %w", entriesDir, mkdirErr)
	}

	entriesPath := filepath.Join(entriesDir, startedAt.Format("2006-01-02")+".json")
	entries, readErr := readEntries(entriesPath)
	if readErr != nil {
		return fmt.Errorf("failed to read %s: %w", entriesPath, readErr)
	}

	entries = append(entries, entry)

	encodedEntries, marshalErr := json.MarshalIndent(entries, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("failed to encode entries: %w", marshalErr)
	}

	if writeErr := os.WriteFile(entriesPath, append(encodedEntries, '\n'), 0644); writeErr != nil {
		return fmt.Errorf("failed to write %s: %w", entriesPath, writeErr)
	}

	return nil
}
