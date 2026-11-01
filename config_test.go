package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateConfigFile(t *testing.T) {
	tests := []struct {
		name       string
		legacy     string // empty means no legacy file
		current    string // empty means no current file
		wantFile   string // expected content of the config file; empty means absent
		wantLegacy bool   // whether the legacy file should remain
	}{
		{"moves the legacy file", `{"old":1}`, "", `{"old":1}`, false},
		{"keeps an existing config", `{"old":1}`, `{"new":1}`, `{"new":1}`, true},
		{"does nothing without a legacy file", "", "", "", false},
		{"leaves an existing config alone", "", `{"new":1}`, `{"new":1}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			legacyFile := filepath.Join(dir, "exe", "awsdo_config.json")
			configFile := filepath.Join(dir, "home", ".awsdo", "config.json")

			if err := os.MkdirAll(filepath.Dir(legacyFile), 0700); err != nil {
				t.Fatal(err)
			}
			if tt.legacy != "" {
				if err := os.WriteFile(legacyFile, []byte(tt.legacy), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.current != "" {
				if err := os.MkdirAll(filepath.Dir(configFile), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(configFile, []byte(tt.current), 0600); err != nil {
					t.Fatal(err)
				}
			}

			if err := migrateConfigFile(legacyFile, configFile); err != nil {
				t.Fatalf("migrateConfigFile() error = %v", err)
			}

			got, err := os.ReadFile(configFile)
			if tt.wantFile == "" {
				if err == nil {
					t.Errorf("config file exists with %q, want none", got)
				}
			} else {
				if err != nil || string(got) != tt.wantFile {
					t.Errorf("config file = %q (err %v), want %q", got, err, tt.wantFile)
				}
				if info, _ := os.Stat(configFile); info.Mode().Perm() != 0600 {
					t.Errorf("config file mode = %v, want 0600", info.Mode().Perm())
				}
			}

			_, err = os.Stat(legacyFile)
			if legacyExists := err == nil; legacyExists != tt.wantLegacy {
				t.Errorf("legacy file exists = %v, want %v", legacyExists, tt.wantLegacy)
			}
		})
	}
}

func TestSaveConfigurationCreatesDirectory(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), ".awsdo", "config.json")

	saveConfiguration(configFile, &Configuration{DefaultProfile: "dev"})

	config, err := loadConfiguration(configFile)
	if err != nil || config.DefaultProfile != "dev" {
		t.Errorf("reloaded config = %+v, err %v", config, err)
	}
}
