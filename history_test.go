package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestHistoryLimit(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   int
	}{
		{"unset uses the default", 0, defaultHistoryLength},
		{"positive is used as is", 25, 25},
		{"negative disables", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := historyLimit(&Configuration{CommandHistoryLength: tt.length})
			if got != tt.want {
				t.Errorf("historyLimit() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAppendHistory(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		command  string
		limit    int
		want     []string
	}{
		{"appends to empty history", nil, "login", 10, []string{"login"}},
		{"appends after existing entries", []string{"login"}, "bastion db", 10, []string{"login", "bastion db"}},
		{"trims the oldest entries at the limit", []string{"a", "b", "c"}, "d", 3, []string{"b", "c", "d"}},
		{"skips a repeat of the last entry", []string{"login"}, "login", 10, []string{"login"}},
		{"keeps a repeat of an older entry", []string{"login", "ls"}, "login", 10, []string{"login", "ls", "login"}},
		{"ignores blank commands", []string{"login"}, "   ", 10, []string{"login"}},
		{"never records history commands", []string{"login"}, "history clear", 10, []string{"login"}},
		{"records nothing when disabled", []string{"login"}, "ls", 0, []string{"login"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".awsdo", "history")
			for _, entry := range tt.existing {
				if err := appendHistory(path, entry, 1000); err != nil {
					t.Fatalf("seeding history: %v", err)
				}
			}

			if err := appendHistory(path, tt.command, tt.limit); err != nil {
				t.Fatalf("appendHistory() error = %v", err)
			}

			got := loadHistory(path, 1000)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("history = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppendHistoryRestrictsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".awsdo", "history")
	if err := appendHistory(path, "login", 10); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("history file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestLoadHistoryLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(path, []byte("a\nb\n\nc\n"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		path  string
		limit int
		want  []string
	}{
		{"returns the newest entries", path, 2, []string{"b", "c"}},
		{"skips blank lines", path, 10, []string{"a", "b", "c"}},
		{"missing file is empty", filepath.Join(t.TempDir(), "none"), 10, nil},
		{"zero limit is empty", path, 0, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loadHistory(tt.path, tt.limit)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadHistory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClearHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	if err := appendHistory(path, "login", 10); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"existing file", "already missing file"} {
		if err := clearHistory(path); err != nil {
			t.Errorf("%s: clearHistory() error = %v", name, err)
		}
	}

	if got := loadHistory(path, 10); got != nil {
		t.Errorf("history after clear = %v, want empty", got)
	}
}

func TestHistoryCommandRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	path := historyFilePath()
	for _, entry := range []string{"login -p dev", "BASTION prod"} {
		if err := appendHistory(path, entry, 10); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name        string
		args        []string
		wantErr     bool
		wantCommand string
		wantArgs    []string
	}{
		{"runs an entry by number", []string{"run", "1"}, false, "login", []string{"-p", "dev"}},
		{"lowercases the command", []string{"run", "2"}, false, "bastion", []string{"prod"}},
		{"rejects an out of range number", []string{"run", "3"}, true, "", nil},
		{"rejects a non-number", []string{"run", "x"}, true, "", nil},
		{"rejects a missing number", []string{"run"}, true, "", nil},
		{"rejects an unknown subcommand", []string{"nope"}, true, "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotCommand string
			var gotArgs []string

			err := historyCommand(tt.args, &Configuration{}, func(command string, args []string) {
				gotCommand, gotArgs = command, args
			})

			if (err != nil) != tt.wantErr {
				t.Fatalf("historyCommand() error = %v, wantErr %v", err, tt.wantErr)
			}

			if gotCommand != tt.wantCommand || !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Errorf("ran %q %v, want %q %v", gotCommand, gotArgs, tt.wantCommand, tt.wantArgs)
			}
		})
	}
}

func TestLineEditorPersistsOnlyWhenGivenAPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	newQuestionEditor("name: ").addToHistory("secret answer")
	if got := loadHistory(historyFilePath(), 10); got != nil {
		t.Fatalf("question editor persisted %v", got)
	}

	newLineEditor(&Configuration{}).addToHistory("instances ls")

	reloaded := newLineEditor(&Configuration{})
	if want := []string{"instances ls"}; !reflect.DeepEqual(reloaded.history, want) {
		t.Errorf("reloaded history = %v, want %v", reloaded.history, want)
	}
}
