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
