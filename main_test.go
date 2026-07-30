package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpPointsToCommandSpecificHelp(t *testing.T) {
	output := bytes.Buffer{}
	printRootHelp(&output)
	helpText := output.String()

	expectedText := []string{
		"worklog <command> [arguments]",
		"worklog help <command>",
		`Run "worklog help <command>" for command-specific arguments and examples.`,
	}
	for _, expected := range expectedText {
		if !strings.Contains(helpText, expected) {
			t.Fatalf("expected root help to contain %q, got:\n%s", expected, helpText)
		}
	}

	for _, command := range worklogCommands {
		if !strings.Contains(helpText, command.name) {
			t.Fatalf("expected root help to list command %q, got:\n%s", command.name, helpText)
		}
	}
}

func TestCommandHelpDocumentsArguments(t *testing.T) {
	tests := []struct {
		command      string
		expectedText []string
	}{
		{
			command: "add",
			expectedText: []string{
				"worklog add <minutes> <goal> [result]",
				"<minutes>",
				"<goal>",
				"[result]",
			},
		},
		{
			command: "start",
			expectedText: []string{
				"worklog start",
				"Arguments:",
				"None.",
				"Prompts:",
				"goal",
				"duration",
				"result",
			},
		},
		{
			command: "resume",
			expectedText: []string{
				"worklog resume",
				"Arguments:",
				"None.",
				"remaining minutes",
			},
		},
		{
			command: "status",
			expectedText: []string{
				"worklog status",
				"Arguments:",
				"None.",
			},
		},
		{
			command: "summary",
			expectedText: []string{
				"worklog summary <week|month|year> [filter]",
				"worklog summary <YYYY-MM-DD> <YYYY-MM-DD> [filter]",
				"<week|month|year>",
				"<YYYY-MM-DD>",
				"[filter]",
			},
		},
		{
			command: "help",
			expectedText: []string{
				"worklog help <command>",
				"worklog <command> --help",
				"[command]",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			output := bytes.Buffer{}
			if !printCommandHelp(test.command, &output) {
				t.Fatalf("expected help for command %q", test.command)
			}

			helpText := output.String()
			for _, expected := range test.expectedText {
				if !strings.Contains(helpText, expected) {
					t.Fatalf("expected %s help to contain %q, got:\n%s", test.command, expected, helpText)
				}
			}
		})
	}
}

func TestCommandHelpReportsUnknownCommand(t *testing.T) {
	output := bytes.Buffer{}
	if printCommandHelp("missing", &output) {
		t.Fatal("expected missing command help to return false")
	}

	if output.Len() != 0 {
		t.Fatalf("expected missing command help to leave output empty, got:\n%s", output.String())
	}
}
