package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/pelletier/go-toml/v2"
)

type worklogConfig struct {
	MinutesPerDay int   `toml:"minutes_per_day"`
	TimeSets      []int `toml:"time_sets"`
}

func defaultWorklogConfig() worklogConfig {
	return worklogConfig{
		MinutesPerDay: 300,
		TimeSets:      []int{30, 60, 90},
	}
}

func loadWorklogConfig(worklogDir string) (worklogConfig, error) {
	config := defaultWorklogConfig()
	configPath := filepath.Join(worklogDir, "config.toml")

	configBytes, readErr := os.ReadFile(configPath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return config, nil
		}

		return config, readErr
	}

	parsed := worklogConfig{}
	if unmarshalErr := toml.Unmarshal(configBytes, &parsed); unmarshalErr != nil {
		return config, unmarshalErr
	}

	if parsed.MinutesPerDay != 0 {
		if parsed.MinutesPerDay <= 0 {
			return config, fmt.Errorf("minutes_per_day must be greater than zero")
		}

		config.MinutesPerDay = parsed.MinutesPerDay
	}

	if len(parsed.TimeSets) > 0 {
		timeSets := []int{}
		for _, minutes := range parsed.TimeSets {
			if minutes <= 0 {
				return config, fmt.Errorf("time_sets values must be greater than zero")
			}

			if !slices.Contains(timeSets, minutes) {
				timeSets = append(timeSets, minutes)
			}
		}

		if len(timeSets) == 0 {
			return config, fmt.Errorf("time_sets must contain at least one value")
		}

		slices.Sort(timeSets)
		config.TimeSets = timeSets
	}

	return config, nil
}
