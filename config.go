package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Configuration struct {
	DefaultProfile string                   `json:"defaultProfile,omitempty"`
	Profiles       map[string]Profile       `json:"profiles,omitempty"`
	BastionLookup  map[string]BastionLookup `json:"-"` // Map of bastion ID to profile and name

	// CommandHistoryLength caps the persisted command history. Absent (0) means
	// defaultHistoryLength; a negative value disables persistence.
	CommandHistoryLength int `json:"commandHistoryLength,omitempty"`
}

type BastionLookup struct {
	Profile string `json:"profile,omitempty"`
	Name    string `json:"name,omitempty"`
}

type Profile struct {
	Name            string              `json:"name,omitempty"`
	DefaultInstance string              `json:"defaultInstance,omitempty"` // Default instance name
	Bastions        map[string]Bastion  `json:"bastions,omitempty"`        // Multiple named bastions
	DefaultBastion  string              `json:"defaultBastion,omitempty"`  // Default bastion name
	Instances       map[string]Instance `json:"instances,omitempty"`       // Named EC2 instances
}

type Instance struct {
	Name    string `json:"name,omitempty"`
	ID      string `json:"id,omitempty"`
	Profile string `json:"profile,omitempty"`
	Host    string `json:"host,omitempty"`
	Filter  string `json:"filter,omitempty"`
}

type Bastion struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Profile   string `json:"profile,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	LocalPort int    `json:"localPort,omitempty"`
	Filter    string `json:"filter,omitempty"`

	// KeepAliveSeconds is the interval between keepalive pings through the
	// tunnel. Absent (0) means defaultKeepAliveSeconds; keepAliveOff disables it.
	KeepAliveSeconds int `json:"keepAliveSeconds,omitempty"`

	// NoReconnect is stored inverted so that existing configurations, which
	// have no such field, reconnect by default.
	NoReconnect bool `json:"noReconnect,omitempty"`
}

type RDSDatabase struct {
	DBInstanceIdentifier string `json:"ID"`
	Endpoint             string `json:"Endpoint"`
	Port                 int    `json:"Port"`
	Engine               string `json:"Engine"`
}

type EC2Instance struct {
	Instance     string `json:"Instance"`
	Name         string `json:"Name"`
	AZ           string `json:"AZ"`
	Host         string `json:"Host"`
	State        string `json:"State"`
	InstanceType string `json:"Type"`
	PublicIP     string `json:"PublicIP"`
	LaunchTime   string `json:"LaunchTime"`
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// configFilePath returns ~/.awsdo/config.json, first moving over a legacy
// awsdo_config.json found next to the executable.
func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine the home directory for the config file: %v", err)
	}

	configFile := filepath.Join(home, ".awsdo", "config.json")

	exePath, err := os.Executable()
	if err != nil {
		return configFile, nil
	}

	legacyFile := filepath.Join(filepath.Dir(exePath), "awsdo_config.json")

	return configFile, migrateConfigFile(legacyFile, configFile)
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// migrateConfigFile moves legacyFile to configFile, unless configFile already
// exists or there is nothing to migrate.
func migrateConfigFile(legacyFile, configFile string) error {
	_, err := os.Stat(configFile)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not check %s: %v", configFile, err)
	}

	content, err := os.ReadFile(legacyFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not read %s for migration: %v", legacyFile, err)
	}

	if err := os.MkdirAll(filepath.Dir(configFile), 0700); err != nil {
		return fmt.Errorf("could not create %s: %v", filepath.Dir(configFile), err)
	}

	if err := os.WriteFile(configFile, content, 0600); err != nil {
		return fmt.Errorf("could not migrate %s to %s: %v", legacyFile, configFile, err)
	}

	// The copy is already in place, so a leftover legacy file only means the
	// old location is ignored from now on.
	if err := os.Remove(legacyFile); err != nil {
		fmt.Printf("Migrated the config file to %s, but could not remove %s: %v\n", configFile, legacyFile, err)
		return nil
	}

	fmt.Printf("Migrated the config file from %s to %s\n", legacyFile, configFile)
	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func loadConfiguration(fileName string) (Configuration, error) {
	fileInfo, err := os.Stat(fileName)
	if err != nil {
		return Configuration{}, nil
	}

	// Ensure that the configuration file is accessible only to the current user
	// First, check the current permissions of the file
	// and then set it to be accessible only to the current user
	if fileInfo.Mode().Perm() != 0600 {
		if err := os.Chmod(fileName, 0600); err != nil {
			return Configuration{}, fmt.Errorf("could not set config file permissions: %v", err)
		}
	}

	// Read the configuration file
	var config Configuration
	configBytes, err := os.ReadFile(fileName)

	if err != nil {
		return Configuration{}, fmt.Errorf("could not read config.json file")
	}

	if err := json.Unmarshal(configBytes, &config); err != nil {
		return Configuration{}, fmt.Errorf("could not read config.json file")
	}

	// Initialize BastionLookup if nil
	if config.BastionLookup == nil {
		config.BastionLookup = make(map[string]BastionLookup)
	}

	// Populate Profile field in each bastion and build ID lookup map
	if config.Profiles != nil {
		for profileName, profile := range config.Profiles {
			// Initialize Instances map if nil
			if profile.Instances == nil {
				profile.Instances = make(map[string]Instance)
			}

			// Populate Profile field in each instance
			for instanceName, instance := range profile.Instances {
				// Set Profile field if not already set
				if instance.Profile == "" {
					instance.Profile = profileName
				}

				// Update instance in profile
				profile.Instances[instanceName] = instance
			}

			if profile.Bastions != nil {
				for bastionName, bastion := range profile.Bastions {
					// Set Profile field if not already set
					if bastion.Profile == "" {
						bastion.Profile = profileName
					}

					// Generate ID if not present
					if bastion.ID == "" {
						newID, err := generateBastionID()

						if err != nil {
							return Configuration{}, fmt.Errorf("failed to generate bastion ID: %v", err)
						}

						bastion.ID = newID
					}

					// Add to lookup map
					config.BastionLookup[bastion.ID] = BastionLookup{
						Profile: profileName,
						Name:    bastionName,
					}

					// Update bastion in profile
					profile.Bastions[bastionName] = bastion
				}
			}

			config.Profiles[profileName] = profile
		}
	}

	return config, nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func saveConfiguration(fileName string, config *Configuration) {
	// Rebuild BastionLookup map before saving
	rebuildBastionLookup(config)

	// Save the configuration file with permissions to only the current user
	configBytes, _ := json.MarshalIndent(config, "", "    ")
	if err := os.MkdirAll(filepath.Dir(fileName), 0700); err != nil {
		fmt.Printf("Could not create %s: %v\n", filepath.Dir(fileName), err)
		return
	}

	if err := os.WriteFile(fileName, configBytes, 0600); err != nil {
		fmt.Printf("Could not save %s: %v\n", fileName, err)
	}
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func rebuildBastionLookup(config *Configuration) {
	// Initialize lookup map if nil
	if config.BastionLookup == nil {
		config.BastionLookup = make(map[string]BastionLookup)
	}

	// Clear existing lookup
	config.BastionLookup = make(map[string]BastionLookup)

	// Rebuild lookup from all profiles
	if config.Profiles != nil {
		for profileName, profile := range config.Profiles {
			if profile.Bastions != nil {
				for bastionName, bastion := range profile.Bastions {
					// Ensure Profile field is set
					if bastion.Profile == "" {
						bastion.Profile = profileName
					}

					// Generate ID if not present
					if bastion.ID == "" {
						newID, err := generateBastionID()

						if err == nil {
							bastion.ID = newID
						}
					}

					// Add to lookup map
					if bastion.ID != "" {
						config.BastionLookup[bastion.ID] = BastionLookup{
							Profile: profileName,
							Name:    bastionName,
						}
					}

					// Update bastion in profile
					profile.Bastions[bastionName] = bastion
				}
			}

			config.Profiles[profileName] = profile
		}
	}
}
