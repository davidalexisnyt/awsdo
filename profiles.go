package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func profileExistsInAWSConfig(name string) bool {
	configPath := getAWSConfigPath()
	file, err := os.Open(configPath)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[profile ") {
			profileName := strings.TrimPrefix(line, "[profile ")
			profileName = strings.TrimSuffix(profileName, "]")
			profileName = strings.TrimSpace(profileName)
			if profileName == name {
				return true
			}
		} else if line == "[default]" && name == "default" {
			return true
		}
	}
	return false
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func addProfile(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("profiles add", flag.ContinueOnError)
	nameFlag := flagSet.String("name", "", "--name <profile name>")
	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo profiles add <profile name>")
		fmt.Println("   or: awsdo profiles add --name <profile name>")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	var profileName string
	if *nameFlag != "" {
		profileName = *nameFlag
	} else if len(flagSet.Args()) > 0 {
		profileName = strings.TrimSpace(flagSet.Args()[0])
	}

	if profileName == "" {
		fmt.Println("Error: profile name is required")
		fmt.Println("Usage: awsdo profiles add <profile name>")
		return fmt.Errorf("profile name is required")
	}

	// Basic validation: no spaces
	if strings.Contains(profileName, " ") {
		fmt.Println("Error: profile name cannot contain spaces")
		return fmt.Errorf("invalid profile name")
	}

	if profileExistsInAWSConfig(profileName) {
		fmt.Printf("Profile '%s' already exists in ~/.aws/config\n", profileName)
		return fmt.Errorf("profile already exists")
	}

	return addProfileWithSSOSetup(config, profileName)
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func removeProfile(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("profiles remove", flag.ContinueOnError)
	nameFlag := flagSet.String("name", "", "--name <profile name>")
	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo profiles remove <profile name>")
		fmt.Println("   or: awsdo profiles rm <profile name>")
		fmt.Println("   or: awsdo rm profile <profile name>")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	var profileName string
	if *nameFlag != "" {
		profileName = *nameFlag
	} else if len(flagSet.Args()) > 0 {
		profileName = strings.TrimSpace(flagSet.Args()[0])
	}

	if profileName == "" {
		fmt.Println("Error: profile name is required")
		fmt.Println("Usage: awsdo profiles remove <profile name>")
		return fmt.Errorf("profile name is required")
	}

	if strings.Contains(profileName, " ") {
		fmt.Println("Error: profile name cannot contain spaces")
		return fmt.Errorf("invalid profile name")
	}

	if !profileExistsInAWSConfig(profileName) {
		fmt.Printf("Profile '%s' not found in ~/.aws/config\n", profileName)
		return fmt.Errorf("profile not found")
	}

	// Display profile info
	fmt.Printf("\nProfile to remove: %s\n", profileName)
	if config != nil && config.DefaultProfile == profileName {
		fmt.Println("  (This is the awsdo default profile)")
	}

	// Ask for confirmation
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nAre you sure you want to remove this profile? (yes/no): ")
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.TrimSpace(strings.ToLower(confirmation))

	if confirmation != "yes" && confirmation != "y" {
		fmt.Println("Removal cancelled.")
		return nil
	}

	configPath := getAWSConfigPath()
	if err := removeProfileFromAWSConfig(configPath, profileName); err != nil {
		fmt.Printf("Error removing from AWS config: %v\n", err)
		return err
	}

	// Remove from awsdo config
	if config != nil && config.Profiles != nil {
		delete(config.Profiles, profileName)
		if config.DefaultProfile == profileName {
			config.DefaultProfile = ""
		}
	}

	fmt.Printf("\nProfile '%s' removed successfully!\n", profileName)
	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func listProfiles(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("profiles list", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo ls profiles [--profile <aws cli profile>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	configPath := getAWSConfigPath()

	file, err := os.Open(configPath)
	if err != nil {
		fmt.Println("\nNo AWS profiles configured.")
		fmt.Println()
		return nil
	}
	defer file.Close()

	var profiles []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[profile ") {
			// Extract profile name: [profile name] -> name
			name := strings.TrimPrefix(line, "[profile ")
			name = strings.TrimSuffix(name, "]")
			name = strings.TrimSpace(name)
			if name != "" {
				profiles = append(profiles, name)
			}
		} else if line == "[default]" {
			profiles = append(profiles, "default")
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("\nNo AWS profiles configured.")
		fmt.Println()
		return nil
	}

	if len(profiles) == 0 {
		fmt.Println("\nNo AWS profiles configured.")
		fmt.Println()
		return nil
	}

	// Filter by profile if specified
	if *profile != "" || *profileShort != "" {
		targetProfile := *profile
		if *profileShort != "" {
			targetProfile = *profileShort
		}
		found := false
		for _, p := range profiles {
			if p == targetProfile {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("\nProfile '%s' not found in AWS config.\n", targetProfile)
			fmt.Println()
			return nil
		}
		profiles = []string{targetProfile}
	}

	// Sort: default first, then alphabetically
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i] == "default" {
			return true
		}
		if profiles[j] == "default" {
			return false
		}
		return profiles[i] < profiles[j]
	})

	// Calculate column width
	maxProfileWidth := len("Profile")
	for _, p := range profiles {
		displayName := p
		if config != nil && config.DefaultProfile == p {
			displayName = "*" + p
		}
		if len(displayName) > maxProfileWidth {
			maxProfileWidth = len(displayName)
		}
	}

	const padding = 2
	colProfileWidth := maxProfileWidth + padding

	truncate := func(s string, width int) string {
		if len(s) > width {
			return s[:width-3] + "..."
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	bold := "\033[1m"
	reset := "\033[0m"

	fmt.Println()

	// Print top border
	fmt.Printf("┌%s┐\n", strings.Repeat("─", colProfileWidth))

	// Print header row
	fmt.Printf("│%s%s%s│\n", bold, truncate("Profile", colProfileWidth), reset)

	// Print separator
	fmt.Printf("├%s┤\n", strings.Repeat("─", colProfileWidth))

	// Print data rows
	for _, p := range profiles {
		displayName := p
		if config != nil && config.DefaultProfile == p {
			displayName = "*" + p
		}
		fmt.Printf("│%s│\n", truncate(displayName, colProfileWidth))
	}

	// Print bottom border
	fmt.Printf("└%s┘\n", strings.Repeat("─", colProfileWidth))

	fmt.Println()

	return nil
}
