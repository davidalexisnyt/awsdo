package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
)

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func listBastions(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("bastions list", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo bastions list [--profile <aws cli profile>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	// List all bastions across all profiles
	if config.Profiles == nil {
		fmt.Println("\nNo bastions configured.")
		fmt.Println()
		return nil
	}

	// Collect all bastions grouped by profile
	type bastionRow struct {
		Bastion     Bastion
		BastionName string
		IsDefault   bool
	}

	// Map to group bastions by profile
	profileGroups := make(map[string][]bastionRow)

	for profileName, profileInfo := range config.Profiles {
		// If profile filter is specified, only show that profile
		if *profile != "" || *profileShort != "" {
			targetProfile := *profile

			if *profileShort != "" {
				targetProfile = *profileShort
			}

			if profileName != targetProfile {
				continue
			}
		}

		if len(profileInfo.Bastions) > 0 {
			var bastions []bastionRow
			for name, bastion := range profileInfo.Bastions {
				bastions = append(bastions, bastionRow{
					Bastion:     bastion,
					BastionName: name,
					IsDefault:   profileInfo.DefaultBastion == name,
				})
			}
			// Sort bastions by name within this profile
			sort.Slice(bastions, func(i, j int) bool {
				return bastions[i].BastionName < bastions[j].BastionName
			})
			profileGroups[profileName] = bastions
		}
	}

	if len(profileGroups) == 0 {
		fmt.Println("\nNo bastions configured.")
		fmt.Println()
		return nil
	}

	// Get sorted list of profile names
	var profileNames []string
	for profileName := range profileGroups {
		profileNames = append(profileNames, profileName)
	}
	sort.Strings(profileNames)

	// Calculate maximum column widths from all bastions
	maxNameWidth := len("Name") // Start with header width
	maxHostWidth := len("Host")
	maxInstanceWidth := len("Instance")
	maxPortWidth := len("Port")
	maxLocalPortWidth := len("LPort")

	// Iterate through all bastions to find maximum widths
	for _, bastions := range profileGroups {
		for _, row := range bastions {
			// Calculate name width (including "*" for default)
			name := row.BastionName
			if row.IsDefault {
				name = "*" + name
			}
			if len(name) > maxNameWidth {
				maxNameWidth = len(name)
			}

			// Calculate other column widths
			if len(row.Bastion.Host) > maxHostWidth {
				maxHostWidth = len(row.Bastion.Host)
			}
			if len(row.Bastion.Instance) > maxInstanceWidth {
				maxInstanceWidth = len(row.Bastion.Instance)
			}

			// Port and Local Port as strings
			portStr := strconv.Itoa(row.Bastion.Port)
			if len(portStr) > maxPortWidth {
				maxPortWidth = len(portStr)
			}

			localPortStr := strconv.Itoa(row.Bastion.LocalPort)
			if len(localPortStr) > maxLocalPortWidth {
				maxLocalPortWidth = len(localPortStr)
			}
		}
	}

	// Add 2 characters padding for readability
	const padding = 2
	colNameWidth := maxNameWidth + padding
	colHostWidth := maxHostWidth + padding
	colInstanceWidth := maxInstanceWidth + padding
	colPortWidth := maxPortWidth + padding
	colLocalPortWidth := maxLocalPortWidth + padding

	// Helper function to truncate string to width
	truncate := func(s string, width int) string {
		if len(s) > width {
			return s[:width-3] + "..."
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	// Helper function to format integer to string with padding
	formatInt := func(n int, width int) string {
		s := strconv.Itoa(n)
		if len(s) > width {
			return s[:width-3] + "..."
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	// ANSI escape codes for bold
	bold := "\033[1m"
	reset := "\033[0m"

	fmt.Println()

	// Display each profile group
	for i, profileName := range profileNames {
		bastions := profileGroups[profileName]

		// Print profile header
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%sProfile: %s%s\n", bold, profileName, reset)

		// Print top border
		fmt.Printf("┌%s┬%s┬%s┬%s┬%s┐\n",
			strings.Repeat("─", colNameWidth),
			strings.Repeat("─", colHostWidth),
			strings.Repeat("─", colInstanceWidth),
			strings.Repeat("─", colPortWidth),
			strings.Repeat("─", colLocalPortWidth))

		// Print header row
		fmt.Printf("│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│\n",
			bold, truncate("Name", colNameWidth), reset,
			bold, truncate("Host", colHostWidth), reset,
			bold, truncate("Instance", colInstanceWidth), reset,
			bold, truncate("Port", colPortWidth), reset,
			bold, truncate("LPort", colLocalPortWidth), reset)

		// Print separator between header and data
		fmt.Printf("├%s┼%s┼%s┼%s┼%s┤\n",
			strings.Repeat("─", colNameWidth),
			strings.Repeat("─", colHostWidth),
			strings.Repeat("─", colInstanceWidth),
			strings.Repeat("─", colPortWidth),
			strings.Repeat("─", colLocalPortWidth))

		// Print data rows
		for _, row := range bastions {
			name := row.BastionName
			if row.IsDefault {
				name = "*" + name
			}

			fmt.Printf("│%s│%s│%s│%s│%s│\n",
				truncate(name, colNameWidth),
				truncate(row.Bastion.Host, colHostWidth),
				truncate(row.Bastion.Instance, colInstanceWidth),
				formatInt(row.Bastion.Port, colPortWidth),
				formatInt(row.Bastion.LocalPort, colLocalPortWidth))
		}

		// Print bottom border
		fmt.Printf("└%s┴%s┴%s┴%s┴%s┘\n",
			strings.Repeat("─", colNameWidth),
			strings.Repeat("─", colHostWidth),
			strings.Repeat("─", colInstanceWidth),
			strings.Repeat("─", colPortWidth),
			strings.Repeat("─", colLocalPortWidth))
	}

	fmt.Println()

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func addBastion(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("bastions add", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo bastions add [--profile <aws cli profile>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	currentProfile, err := ensureProfile(config, profile, profileShort)
	if err != nil {
		return err
	}

	// Ensure that we're logged in before running the command
	if !isLoggedIn(currentProfile) {
		loginArgs := []string{"--profile", currentProfile}

		if err := login(loginArgs, config); err != nil {
			return err
		}
	}

	profileInfo := config.Profiles[currentProfile]

	if profileInfo.Bastions == nil {
		profileInfo.Bastions = make(map[string]Bastion)
	}

	reader := bufio.NewReader(os.Stdin)

	// Query RDS databases
	fmt.Println("\nQuerying RDS databases...")
	databases, err := queryRDSDatabases(currentProfile)
	if err != nil {
		return fmt.Errorf("failed to query RDS databases: %v", err)
	}

	if len(databases) == 0 {
		fmt.Println("No RDS databases found.")
	} else {
		// Display databases and let user select
		fmt.Println("\nAvailable RDS databases:")
		for i, db := range databases {
			fmt.Printf("  %d. %s (%s) - %s:%d\n", i+1, db.DBInstanceIdentifier, db.Engine, db.Endpoint, db.Port)
		}
	}

	var selectedDB *RDSDatabase

	if len(databases) > 0 {
		fmt.Print("\nSelect database number (or 0 to skip): ")
		dbSelection, _ := reader.ReadString('\n')
		dbIndex, err := strconv.Atoi(strings.TrimSpace(dbSelection))

		if err != nil || dbIndex < 0 || dbIndex > len(databases) {
			return fmt.Errorf("invalid selection")
		}

		if dbIndex > 0 {
			selectedDB = &databases[dbIndex-1]
		}
	}

	// Query bastion instances
	fmt.Println("\nQuerying bastion instances...")
	bastionInstances, err := queryBastionInstances(currentProfile)

	if err != nil {
		return fmt.Errorf("failed to query bastion instances: %v", err)
	}

	if len(bastionInstances) == 0 {
		return fmt.Errorf("no bastion instances found")
	}

	// Display bastion instances and let user select
	fmt.Println("\nAvailable bastion instances:")

	for i, inst := range bastionInstances {
		fmt.Printf("  %d. %s (%s)\n", i+1, inst.Name, inst.Instance)
	}

	fmt.Print("\nSelect bastion instance number: ")
	instSelection, _ := reader.ReadString('\n')
	instIndex, err := strconv.Atoi(strings.TrimSpace(instSelection))

	if err != nil || instIndex < 1 || instIndex > len(bastionInstances) {
		return fmt.Errorf("invalid selection")
	}

	selectedBastionInstance := bastionInstances[instIndex-1]

	// Get bastion name
	fmt.Print("\nEnter bastion name: ")
	bastionName, _ := reader.ReadString('\n')
	bastionName = strings.TrimSpace(bastionName)

	if bastionName == "" {
		// Generate a default name from database identifier
		if selectedDB != nil {
			bastionName = selectedDB.DBInstanceIdentifier
		} else {
			bastionName = fmt.Sprintf("bastion-%d", len(profileInfo.Bastions)+1)
		}
	}

	// Generate unique ID for the bastion
	bastionID, err := generateBastionID()
	if err != nil {
		return fmt.Errorf("failed to generate bastion ID: %v", err)
	}

	// Create bastion configuration
	newBastion := Bastion{
		ID:       bastionID,
		Name:     bastionName,
		Profile:  currentProfile,
		Instance: selectedBastionInstance.Instance,
	}

	if selectedDB != nil {
		newBastion.Host = selectedDB.Endpoint
		newBastion.Port = selectedDB.Port
	} else {
		// Prompt for host and port
		fmt.Print("Enter remote host: ")
		host, _ := reader.ReadString('\n')
		newBastion.Host = strings.TrimSpace(host)

		fmt.Print("Enter remote port: ")
		portStr, _ := reader.ReadString('\n')

		port, err := strconv.Atoi(strings.TrimSpace(portStr))
		if err != nil {
			return fmt.Errorf("invalid port: %v", err)
		}

		newBastion.Port = port
	}

	// Find available local port
	localPort, err := findAvailableLocalPort(7000)
	if err != nil {
		return fmt.Errorf("failed to find available local port: %v", err)
	}

	fmt.Printf("Using local port: %d\n", localPort)
	fmt.Print("Enter local port (or press Enter to use suggested): ")
	localPortStr, _ := reader.ReadString('\n')
	localPortStr = strings.TrimSpace(localPortStr)

	if localPortStr != "" {
		customPort, err := strconv.Atoi(localPortStr)
		if err == nil {
			localPort = customPort
		}
	}

	newBastion.LocalPort = localPort

	// Save to configuration
	profileInfo.Bastions[bastionName] = newBastion

	if profileInfo.DefaultBastion == "" {
		profileInfo.DefaultBastion = bastionName
	}

	profileInfo.Name = currentProfile
	config.Profiles[currentProfile] = profileInfo

	// Update ID lookup map
	if config.BastionLookup == nil {
		config.BastionLookup = make(map[string]BastionLookup)
	}

	config.BastionLookup[bastionID] = BastionLookup{
		Profile: currentProfile,
		Name:    bastionName,
	}

	fmt.Printf("\nBastion '%s' (ID: %s) configured successfully!\n", bastionName, bastionID)

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func updateBastion(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("bastions update", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")
	bastionName := flagSet.String("name", "", "--name <bastion name>")
	bastionNameShort := flagSet.String("n", "", "--name <bastion name>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo bastions update [--profile <aws cli profile>] [--name <bastion name>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	currentProfile, err := ensureProfile(config, profile, profileShort)
	if err != nil {
		return err
	}

	// Ensure that we're logged in before running the command
	if !isLoggedIn(currentProfile) {
		loginArgs := []string{"--profile", currentProfile}
		if err := login(loginArgs, config); err != nil {
			return err
		}
	}

	profileInfo := config.Profiles[currentProfile]
	if profileInfo.Bastions == nil {
		profileInfo.Bastions = make(map[string]Bastion)
	}

	// Get bastion name
	var targetBastionName string

	switch {
	case *bastionName != "":
		targetBastionName = *bastionName
	case *bastionNameShort != "":
		targetBastionName = *bastionNameShort
	case flagSet.NArg() > 0:
		targetBastionName = flagSet.Arg(0)
	default:
		// Prompt for bastion name
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter bastion name to update: ")
		nameInput, _ := reader.ReadString('\n')
		targetBastionName = strings.TrimSpace(nameInput)

		if targetBastionName == "" {
			return fmt.Errorf("bastion name is required")
		}
	}

	// Check if bastion exists
	existingBastion, exists := profileInfo.Bastions[targetBastionName]

	if !exists {
		return fmt.Errorf("bastion '%s' not found in profile '%s'", targetBastionName, currentProfile)
	}

	// Preserve ID and Profile
	existingBastionID := existingBastion.ID

	if existingBastionID == "" {
		// Generate ID if missing
		newID, err := generateBastionID()
		if err != nil {
			return fmt.Errorf("failed to generate bastion ID: %v", err)
		}

		existingBastionID = newID
	}

	reader := bufio.NewReader(os.Stdin)

	// Query RDS databases
	fmt.Println("\nQuerying RDS databases...")
	databases, err := queryRDSDatabases(currentProfile)
	if err != nil {
		return fmt.Errorf("failed to query RDS databases: %v", err)
	}

	if len(databases) == 0 {
		fmt.Println("No RDS databases found.")
	} else {
		// Display databases and let user select
		fmt.Println("\nAvailable RDS databases:")

		for i, db := range databases {
			fmt.Printf("  %d. %s (%s) - %s:%d\n", i+1, db.DBInstanceIdentifier, db.Engine, db.Endpoint, db.Port)
		}
	}

	var selectedDB *RDSDatabase
	if len(databases) > 0 {
		fmt.Print("\nSelect database number (or 0 to skip): ")
		dbSelection, _ := reader.ReadString('\n')

		dbIndex, err := strconv.Atoi(strings.TrimSpace(dbSelection))
		if err != nil || dbIndex < 0 || dbIndex > len(databases) {
			return fmt.Errorf("invalid selection")
		}

		if dbIndex > 0 {
			selectedDB = &databases[dbIndex-1]
		}
	}

	// Query bastion instances
	fmt.Println("\nQuerying bastion instances...")

	bastionInstances, err := queryBastionInstances(currentProfile)
	if err != nil {
		return fmt.Errorf("failed to query bastion instances: %v", err)
	}

	if len(bastionInstances) == 0 {
		return fmt.Errorf("no bastion instances found")
	}

	// Display bastion instances and let user select
	fmt.Println("\nAvailable bastion instances:")

	for i, inst := range bastionInstances {
		fmt.Printf("  %d. %s (%s)\n", i+1, inst.Name, inst.Instance)
	}

	fmt.Print("\nSelect bastion instance number: ")
	instSelection, _ := reader.ReadString('\n')

	instIndex, err := strconv.Atoi(strings.TrimSpace(instSelection))
	if err != nil || instIndex < 1 || instIndex > len(bastionInstances) {
		return fmt.Errorf("invalid selection")
	}

	selectedBastionInstance := bastionInstances[instIndex-1]

	// Update bastion configuration
	updatedBastion := Bastion{
		ID:       existingBastionID,
		Name:     targetBastionName,
		Profile:  currentProfile,
		Instance: selectedBastionInstance.Instance,
	}

	if selectedDB != nil {
		updatedBastion.Host = selectedDB.Endpoint
		updatedBastion.Port = selectedDB.Port
	} else {
		// Prompt for host and port
		fmt.Print("Enter remote host: ")
		host, _ := reader.ReadString('\n')
		updatedBastion.Host = strings.TrimSpace(host)

		fmt.Print("Enter remote port: ")
		portStr, _ := reader.ReadString('\n')

		port, err := strconv.Atoi(strings.TrimSpace(portStr))
		if err != nil {
			return fmt.Errorf("invalid port: %v", err)
		}

		updatedBastion.Port = port
	}

	// Find available local port
	localPort, err := findAvailableLocalPort(7000)
	if err != nil {
		return fmt.Errorf("failed to find available local port: %v", err)
	}

	fmt.Printf("Using local port: %d\n", localPort)
	fmt.Print("Enter local port (or press Enter to use suggested): ")
	localPortStr, _ := reader.ReadString('\n')
	localPortStr = strings.TrimSpace(localPortStr)

	if localPortStr != "" {
		customPort, err := strconv.Atoi(localPortStr)
		if err == nil {
			localPort = customPort
		}
	}

	updatedBastion.LocalPort = localPort

	// Save to configuration
	profileInfo.Bastions[targetBastionName] = updatedBastion
	profileInfo.Name = currentProfile
	config.Profiles[currentProfile] = profileInfo

	// Update ID lookup map
	if config.BastionLookup == nil {
		config.BastionLookup = make(map[string]BastionLookup)
	}

	config.BastionLookup[existingBastionID] = BastionLookup{
		Profile: currentProfile,
		Name:    targetBastionName,
	}

	fmt.Printf("\nBastion '%s' (ID: %s) updated successfully!\n", targetBastionName, existingBastionID)

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func startBastionTunnel(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("bastion", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")
	bastionNameFull := flagSet.String("name", "", "--name <bastion name>")
	bastionNameShort := flagSet.String("n", "", "-n <bastion name>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:")
		fmt.Println("    awsdo bastion [--profile <aws cli profile>] [--name <bastion name>]")
		fmt.Println("                    [--instance <instance id>] [--host <remote host>]")
		fmt.Println("                    [--port <remote port>] [--local <local port>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	// Handle bastion name lookup logic
	var bastion Bastion
	var currentProfile string
	var err error
	var bastionName string

	switch {
	case flagSet.NArg() > 0:
		bastionName = flagSet.Arg(0)
	case *bastionNameFull != "":
		bastionName = *bastionNameFull
	case *bastionNameShort != "":
		bastionName = *bastionNameShort
	}

	if bastionName != "" {
		// If profile is specified, look only in that profile
		if *profile != "" || *profileShort != "" {
			currentProfile, err = ensureProfile(config, profile, profileShort)
			if err != nil {
				return err
			}

			profileInfo := config.Profiles[currentProfile]

			if profileInfo.Bastions == nil {
				profileInfo.Bastions = make(map[string]Bastion)
			}

			selectedBastion, err := selectBastionByName(profileInfo, bastionName)

			if err != nil {
				return fmt.Errorf("bastion '%s' not found in profile '%s'", bastionName, currentProfile)
			}

			bastion = selectedBastion
		} else {
			// No profile specified - first check default profile, then search all profiles
			if config.DefaultProfile != "" {
				// Try default profile first
				if profileInfo, exists := config.Profiles[config.DefaultProfile]; exists {
					if profileInfo.Bastions != nil {
						if selectedBastion, err := selectBastionByName(profileInfo, bastionName); err == nil {
							bastion = selectedBastion
							currentProfile = config.DefaultProfile
						}
					}
				}
			}

			// If not found in default profile, search all profiles (skip default if already checked)
			if bastion.Instance == "" {
				found := false

				if config.Profiles != nil {
					for profileName, profileInfo := range config.Profiles {
						// Skip default profile if we already checked it
						if profileName == config.DefaultProfile {
							continue
						}

						if profileInfo.Bastions != nil {
							if selectedBastion, err := selectBastionByName(profileInfo, bastionName); err == nil {
								bastion = selectedBastion

								// Ensure Profile field is set
								if bastion.Profile == "" {
									bastion.Profile = profileName
								}

								currentProfile = profileName
								found = true
								break
							}
						}
					}
				}

				if !found {
					return fmt.Errorf("bastion '%s' not found in any profile", bastionName)
				}
			} else {
				// Ensure Profile field is set when found in default profile
				if bastion.Profile == "" {
					bastion.Profile = currentProfile
				}
			}
		}
	} else {
		// No name specified - use existing logic
		currentProfile, err = ensureProfile(config, profile, profileShort)
		if err != nil {
			return err
		}

		profileInfo := config.Profiles[currentProfile]

		if profileInfo.Bastions == nil {
			profileInfo.Bastions = make(map[string]Bastion)
		}

		// Try to get bastion from saved configuration
		if len(profileInfo.Bastions) > 0 {
			selectedBastion, err := selectBastionByName(profileInfo, "")

			if err == nil {
				bastion = selectedBastion
			}
		}
	}

	// Check if Session Manager plugin is installed
	pluginCheck := exec.Command("session-manager-plugin")

	if err := pluginCheck.Run(); err != nil {
		return fmt.Errorf("AWS Session Manager plugin is not installed. Please install it first")
	}

	commandArgs := []string{
		"ssm",
		"start-session",
		"--target",
		bastion.Instance,
		"--document-name",
		"AWS-StartPortForwardingSessionToRemoteHost",
		"--parameters",
		fmt.Sprintf(`host="%s",portNumber="%d",localPortNumber="%d"`, bastion.Host, bastion.Port, bastion.LocalPort),
	}

	// Use profile from bastion if available, otherwise use currentProfile
	bastionProfile := currentProfile

	if bastion.Profile != "" {
		bastionProfile = bastion.Profile
	}

	if len(bastionProfile) != 0 {
		commandArgs = append(commandArgs, "--profile", bastionProfile)
	}

	// Ensure that we're logged in before running the command
	if !isLoggedIn(bastionProfile) {
		args := []string{}

		if len(bastionProfile) != 0 {
			args = append(args, "--profile", bastionProfile)
		}

		login(args, config)
	}

	msg := fmt.Sprintf("Starting bastion tunnel to %s%s%s \non port %d using profile %s%s%s", greenColor, bastion.Host, resetColor, bastion.Port, greenColor, bastion.Profile, resetColor)

	fmt.Println("\n-----------------------------------------------------------------------------------------------")
	fmt.Println(msg)
	fmt.Println("-----------------------------------------------------------------------------------------------")

	fmt.Printf("\nStarting port forwarding session to %s:%d via bastion %s...\n", bastion.Host, bastion.LocalPort, bastion.Instance)
	fmt.Println("Press Ctrl-C to stop the tunnel and return to the REPL.")

	command := exec.Command("aws", commandArgs...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin

	if err := command.Start(); err != nil {
		return fmt.Errorf("failed to start session: %v", err)
	}

	// Set up signal handling to catch Ctrl-C
	signalChan := make(chan os.Signal, 1)
	setupSignalHandler(signalChan)
	defer signal.Stop(signalChan)

	// Wait for command completion or interrupt in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()

	select {
	case <-signalChan:
		// Signal received (Ctrl-C) - kill the command process
		fmt.Println("\nStopping bastion tunnel...")
		if err := command.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %v", err)
		}

		// Wait for the process to actually terminate
		<-done

		// Don't return an error - just return to REPL
		return nil
	case err := <-done:
		// Command completed normally
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				// If the process was terminated by a signal, don't treat it as an error
				if exitErr.ExitCode() == -1 {
					return nil
				}
			}
			return fmt.Errorf("session ended with error: %v", err)
		}

		return nil
	}
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func removeBastion(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("bastions remove", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")
	bastionName := flagSet.String("name", "", "--name <bastion name>")
	bastionNameShort := flagSet.String("n", "", "--name <bastion name>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo bastions remove [--profile <aws cli profile>] [--name <bastion name>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	currentProfile, err := ensureProfile(config, profile, profileShort)
	if err != nil {
		return err
	}

	profileInfo := config.Profiles[currentProfile]

	if len(profileInfo.Bastions) == 0 {
		return fmt.Errorf("no bastions configured for profile '%s'", currentProfile)
	}

	reader := bufio.NewReader(os.Stdin)

	// Get bastion name
	var targetBastionName string

	switch {
	case *bastionName != "":
		targetBastionName = *bastionName
	case *bastionNameShort != "":
		targetBastionName = *bastionNameShort
	case flagSet.NArg() > 0:
		targetBastionName = flagSet.Arg(0)
	default:
		// Prompt for bastion name
		fmt.Print("Enter bastion name to remove: ")
		nameInput, _ := reader.ReadString('\n')
		targetBastionName = strings.TrimSpace(nameInput)

		if targetBastionName == "" {
			return fmt.Errorf("bastion name is required")
		}
	}

	// Check if bastion exists
	existingBastion, exists := profileInfo.Bastions[targetBastionName]
	if !exists {
		return fmt.Errorf("bastion '%s' not found in profile '%s'", targetBastionName, currentProfile)
	}

	// Display bastion information
	fmt.Printf("\nBastion to remove:\n")
	fmt.Printf("  Name:       %s\n", targetBastionName)
	fmt.Printf("  ID:         %s\n", existingBastion.ID)
	fmt.Printf("  Profile:    %s\n", existingBastion.Profile)
	fmt.Printf("  Instance:   %s\n", existingBastion.Instance)
	fmt.Printf("  Host:       %s\n", existingBastion.Host)
	fmt.Printf("  Port:       %d\n", existingBastion.Port)
	fmt.Printf("  Local Port: %d\n", existingBastion.LocalPort)

	// Ask for confirmation
	fmt.Print("\nAre you sure you want to remove this bastion? (yes/no): ")
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.TrimSpace(strings.ToLower(confirmation))

	if confirmation != "yes" && confirmation != "y" {
		fmt.Println("Removal cancelled.")
		return nil
	}

	// Remove from Bastions map
	delete(profileInfo.Bastions, targetBastionName)

	// If this was the default bastion, clear the DefaultBastion field
	if profileInfo.DefaultBastion == targetBastionName {
		profileInfo.DefaultBastion = ""
	}

	// Remove from BastionLookup map if ID exists
	if existingBastion.ID != "" && config.BastionLookup != nil {
		delete(config.BastionLookup, existingBastion.ID)
	}

	// Update profile in config
	profileInfo.Name = currentProfile
	config.Profiles[currentProfile] = profileInfo

	fmt.Printf("\nBastion '%s' removed successfully!\n", targetBastionName)

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func selectBastionByName(profileInfo Profile, name string) (Bastion, error) {
	if len(profileInfo.Bastions) == 0 {
		return Bastion{}, fmt.Errorf("no bastions configured for this profile")
	}

	// If name is provided, use it
	if name != "" {
		if bastion, exists := profileInfo.Bastions[name]; exists {
			return bastion, nil
		}

		return Bastion{}, fmt.Errorf("bastion '%s' not found", name)
	}

	// If no name provided, try default
	if profileInfo.DefaultBastion != "" {
		if bastion, exists := profileInfo.Bastions[profileInfo.DefaultBastion]; exists {
			return bastion, nil
		}
	}

	// If only one bastion exists, use it
	if len(profileInfo.Bastions) == 1 {
		for _, bastion := range profileInfo.Bastions {
			return bastion, nil
		}
	}

	// Multiple bastions exist, need to specify name
	return Bastion{}, fmt.Errorf("multiple bastions available, please specify --name")
}
