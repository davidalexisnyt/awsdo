package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultHistoryLength = 100

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// historyFilePath returns the history file location, or "" when the home
// directory cannot be determined, which leaves history unpersisted.
func historyFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".awsdo", "history")
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// historyLimit resolves commandHistoryLength: unset means the default and a
// negative value turns persistence off.
func historyLimit(config *Configuration) int {
	if config.CommandHistoryLength < 0 {
		return 0
	}

	if config.CommandHistoryLength == 0 {
		return defaultHistoryLength
	}

	return config.CommandHistoryLength
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// loadHistory returns the newest limit entries, oldest first.
func loadHistory(path string, limit int) []string {
	if path == "" || limit <= 0 {
		return nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var entries []string
	for line := range strings.SplitSeq(string(content), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			entries = append(entries, line)
		}
	}

	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	return entries
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// appendHistory adds a command to the history file and trims the file to limit
// entries. The 'history' command itself is never recorded.
func appendHistory(path, command string, limit int) error {
	command = strings.TrimSpace(command)
	if path == "" || limit <= 0 || command == "" || strings.Fields(command)[0] == "history" {
		return nil
	}

	entries := loadHistory(path, limit)
	if len(entries) > 0 && entries[len(entries)-1] == command {
		return nil
	}

	entries = append(entries, command)
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(strings.Join(entries, "\n")+"\n"), 0600)
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func clearHistory(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// historyCommand implements 'history', 'history run <number>', 'history clear'
// and 'history --raw'. The caller supplies run because the CLI and the REPL
// execute commands differently.
func historyCommand(args []string, config *Configuration, run func(command string, args []string)) error {
	path := historyFilePath()
	if path == "" {
		return errors.New("could not determine the home directory for the history file")
	}

	entries := loadHistory(path, historyLimit(config))

	if len(args) == 0 {
		for i, entry := range entries {
			fmt.Printf("%4d  %s\n", i+1, entry)
		}

		return nil
	}

	switch args[0] {
	case "--raw":
		for _, entry := range entries {
			fmt.Println(entry)
		}

		return nil
	case "clear":
		if err := clearHistory(path); err != nil {
			return fmt.Errorf("could not clear history: %v", err)
		}

		fmt.Println("Command history cleared.")
		return nil
	case "run":
		if len(args) != 2 {
			return errors.New("usage: history run <number>")
		}

		number, err := strconv.Atoi(args[1])
		if err != nil || number < 1 || number > len(entries) {
			return fmt.Errorf("no history entry numbered %q; use 'history' to list entries", args[1])
		}

		fields := strings.Fields(entries[number-1])
		run(strings.ToLower(fields[0]), fields[1:])
		return nil
	}

	return fmt.Errorf("invalid history subcommand: %s\nUse 'history', 'history run <number>', or 'history clear'", args[0])
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// runHistoryEntryInCLI re-runs an entry as a child awsdo process, so the entry
// goes through the same dispatch, saving and exit codes as typing it.
func runHistoryEntryInCLI(command string, args []string) {
	exePath, err := os.Executable()
	if err != nil {
		reportCommandResult(err)
		os.Exit(1)
	}

	child := exec.Command(exePath, append([]string{command}, args...)...)
	child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr

	err = child.Run()
	if err == nil {
		return
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}

	reportCommandResult(err)
	os.Exit(1)
}
