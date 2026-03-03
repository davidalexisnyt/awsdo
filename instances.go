package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func formatLaunchTime(launchTimeStr string) string {
	if launchTimeStr == "" {
		return "(unknown)"
	}

	// Parse ISO8601 timestamp from AWS
	t, err := time.Parse(time.RFC3339, launchTimeStr)
	if err != nil {
		// If parsing fails, return the original string truncated
		if len(launchTimeStr) > 16 {
			return launchTimeStr[:16]
		}

		return launchTimeStr
	}

	// Format as "YYYY-MM-DD HH:MM"
	return t.Format("2006-01-02 15:04")
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func findInstances(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("instances find", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")
	filterFlag := flagSet.String("filter", "", "--filter <filter text>")
	filterShort := flagSet.String("f", "", "--filter <filter text>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo instances find [--profile <aws cli profile>] [--filter <filter text>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	var filter string
	if *filterFlag != "" {
		filter = *filterFlag
	} else if *filterShort != "" {
		filter = *filterShort
	} else {
		// Prompt user for filter text
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter filter text: ")
		filterInput, _ := reader.ReadString('\n')
		filter = strings.TrimSpace(filterInput)

		if filter == "" {
			return fmt.Errorf("filter text cannot be empty")
		}
	}

	currentProfile, err := ensureProfile(config, profile, profileShort)
	if err != nil {
		return err
	}

	commandArgs := []string{
		"ec2",
		"describe-instances",
		"--query",
		"Reservations[*].Instances[*].{Instance:InstanceId,AZ:Placement.AvailabilityZone,Name:Tags[?Key=='Name']|[0].Value,Host:PrivateIpAddress,State:State.Name,Type:InstanceType,PublicIP:PublicIpAddress,LaunchTime:LaunchTime}",
		"--filters",
		fmt.Sprintf("Name=tag:Name,Values=*%s*", filter),
		"--output=json",
	}

	commandArgs = append(commandArgs, "--profile", currentProfile)

	fmt.Printf("\nInstances (%s)\n", currentProfile)

	// Ensure that we're logged in before running the command.
	if !isLoggedIn(currentProfile) {
		args := []string{}
		args = append(args, "--profile", currentProfile)

		login(args, config)
	}

	command := exec.Command("aws", commandArgs...)
	outputStream, err := command.StdoutPipe()
	if err != nil {
		return err
	}

	errorStream, err := command.StderrPipe()
	if err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(errorStream)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()

	err = command.Start()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(outputStream)
	scanner.Split(bufio.ScanLines)
	outputDoc := strings.Builder{}

	for scanner.Scan() {
		outputDoc.WriteString(strings.Trim(scanner.Text(), " "))
	}

	command.Wait()
	output := outputDoc.String()

	/* Output is an array of an array of instance documents like below.
	[
		[
			{
				"Instance": "i-0001",
				"AZ": "us-east-1a",
				"Name": "my-instance-1"
			}
		],
		[
			{
				"Instance": "i-0002",
				"AZ": "us-east-1a",
				"Name": "my-instance-2"
			}
		]
	]
	*/

	if len(output) == 0 {
		fmt.Println("AWS command failed to return data")
	}

	var instanceList [][]map[string]string

	if err := json.Unmarshal([]byte(output), &instanceList); err != nil {
		return err
	}

	// Set the default instance if there is only one instance in the query results
	if len(instanceList) == 1 {
		profileInfo := config.Profiles[currentProfile]
		profileInfo.Name = currentProfile

		// Initialize Instances map if nil
		if profileInfo.Instances == nil {
			profileInfo.Instances = make(map[string]Instance)
		}

		// Create a "default" entry in Instances map
		instanceID := instanceList[0][0]["Instance"]
		instanceName := instanceList[0][0]["Name"]
		if instanceName == "" {
			instanceName = "default"
		}

		// Get host (private IP) from the query result
		host := instanceList[0][0]["Host"]
		if host == "" {
			host = instanceID // Fallback to instance ID if no private IP available
		}

		profileInfo.Instances["default"] = Instance{
			Name:    "default",
			ID:      instanceID,
			Profile: currentProfile,
			Host:    host,
		}

		config.Profiles[currentProfile] = profileInfo
	}

	// Format instances as a table
	if len(instanceList) > 0 {
		// Calculate maximum column widths
		maxNameWidth := len("Name")
		maxInstanceWidth := len("Instance ID")
		maxHostWidth := len("Host")
		maxStateWidth := len("State")
		maxTypeWidth := len("Type")
		maxPublicIPWidth := len("Public IP")
		maxLaunchTimeWidth := len("Launch Time")

		for i := range len(instanceList) {
			name := instanceList[i][0]["Name"]

			if name == "" {
				name = "(no name)"
			}

			if len(name) > maxNameWidth {
				maxNameWidth = len(name)
			}

			instanceID := instanceList[i][0]["Instance"]

			if len(instanceID) > maxInstanceWidth {
				maxInstanceWidth = len(instanceID)
			}

			host := instanceList[i][0]["Host"]

			if host == "" {
				host = "(no host)"
			}

			if len(host) > maxHostWidth {
				maxHostWidth = len(host)
			}

			state := instanceList[i][0]["State"]

			if state == "" {
				state = "(unknown)"
			}

			if len(state) > maxStateWidth {
				maxStateWidth = len(state)
			}

			instanceType := instanceList[i][0]["Type"]

			if instanceType == "" {
				instanceType = "(unknown)"
			}

			if len(instanceType) > maxTypeWidth {
				maxTypeWidth = len(instanceType)
			}

			publicIP := instanceList[i][0]["PublicIP"]

			if publicIP == "" {
				publicIP = "(none)"
			}

			if len(publicIP) > maxPublicIPWidth {
				maxPublicIPWidth = len(publicIP)
			}

			launchTime := formatLaunchTime(instanceList[i][0]["LaunchTime"])

			if len(launchTime) > maxLaunchTimeWidth {
				maxLaunchTimeWidth = len(launchTime)
			}
		}

		// Add 2 characters padding for readability
		const padding = 2
		colNameWidth := maxNameWidth + padding
		colInstanceWidth := maxInstanceWidth + padding
		colHostWidth := maxHostWidth + padding
		colStateWidth := maxStateWidth + padding
		colTypeWidth := maxTypeWidth + padding
		colPublicIPWidth := maxPublicIPWidth + padding
		colLaunchTimeWidth := maxLaunchTimeWidth + padding

		// Helper function to truncate string to width
		truncate := func(s string, width int) string {
			if len(s) > width {
				return s[:width-3] + "..."
			}

			return s + strings.Repeat(" ", width-len(s))
		}

		// ANSI escape codes for bold
		bold := "\033[1m"
		reset := "\033[0m"

		// Print top border
		fmt.Printf("┌%s┬%s┬%s┬%s┬%s┬%s┬%s┐\n",
			strings.Repeat("─", colNameWidth),
			strings.Repeat("─", colInstanceWidth),
			strings.Repeat("─", colHostWidth),
			strings.Repeat("─", colStateWidth),
			strings.Repeat("─", colTypeWidth),
			strings.Repeat("─", colPublicIPWidth),
			strings.Repeat("─", colLaunchTimeWidth))

		// Print header row
		fmt.Printf("│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│\n",
			bold, truncate("Name", colNameWidth), reset,
			bold, truncate("Instance ID", colInstanceWidth), reset,
			bold, truncate("Host", colHostWidth), reset,
			bold, truncate("State", colStateWidth), reset,
			bold, truncate("Type", colTypeWidth), reset,
			bold, truncate("Public IP", colPublicIPWidth), reset,
			bold, truncate("Launch Time", colLaunchTimeWidth), reset)

		// Print separator between header and data
		fmt.Printf("├%s┼%s┼%s┼%s┼%s┼%s┼%s┤\n",
			strings.Repeat("─", colNameWidth),
			strings.Repeat("─", colInstanceWidth),
			strings.Repeat("─", colHostWidth),
			strings.Repeat("─", colStateWidth),
			strings.Repeat("─", colTypeWidth),
			strings.Repeat("─", colPublicIPWidth),
			strings.Repeat("─", colLaunchTimeWidth))

		// Print data rows
		for i := range len(instanceList) {
			name := instanceList[i][0]["Name"]
			if name == "" {
				name = "(no name)"
			}

			instanceID := instanceList[i][0]["Instance"]
			host := instanceList[i][0]["Host"]
			if host == "" {
				host = "(no host)"
			}

			state := instanceList[i][0]["State"]
			if state == "" {
				state = "(unknown)"
			}

			instanceType := instanceList[i][0]["Type"]
			if instanceType == "" {
				instanceType = "(unknown)"
			}

			publicIP := instanceList[i][0]["PublicIP"]
			if publicIP == "" {
				publicIP = "(none)"
			}

			launchTime := formatLaunchTime(instanceList[i][0]["LaunchTime"])

			fmt.Printf("│%s│%s│%s│%s│%s│%s│%s│\n",
				truncate(name, colNameWidth),
				truncate(instanceID, colInstanceWidth),
				truncate(host, colHostWidth),
				truncate(state, colStateWidth),
				truncate(instanceType, colTypeWidth),
				truncate(publicIP, colPublicIPWidth),
				truncate(launchTime, colLaunchTimeWidth))
		}

		// Print bottom border
		fmt.Printf("└%s┴%s┴%s┴%s┴%s┴%s┴%s┘\n",
			strings.Repeat("─", colNameWidth),
			strings.Repeat("─", colInstanceWidth),
			strings.Repeat("─", colHostWidth),
			strings.Repeat("─", colStateWidth),
			strings.Repeat("─", colTypeWidth),
			strings.Repeat("─", colPublicIPWidth),
			strings.Repeat("─", colLaunchTimeWidth))
	}

	fmt.Println()

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func listInstances(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("instances list", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo instances list [--profile <aws cli profile>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	// List all instances across all profiles
	if config.Profiles == nil {
		fmt.Println("\nNo instances configured.")
		fmt.Println()
		return nil
	}

	// Collect all instances grouped by profile
	type instanceRow struct {
		Instance     Instance
		InstanceName string
		InstanceID   string
		IsDefault    bool
	}

	// Map to group instances by profile
	profileGroups := make(map[string][]instanceRow)

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

		if len(profileInfo.Instances) > 0 {
			var instances []instanceRow
			for name, instance := range profileInfo.Instances {
				instances = append(instances, instanceRow{
					Instance:     instance,
					InstanceName: name,
					IsDefault:    profileInfo.DefaultInstance == name,
					InstanceID:   instance.ID,
				})
			}

			// Sort instances by name within this profile
			sort.Slice(instances, func(i, j int) bool {
				return instances[i].InstanceName < instances[j].InstanceName
			})

			profileGroups[profileName] = instances
		}
	}

	if len(profileGroups) == 0 {
		fmt.Println("\nNo instances configured.")
		fmt.Println()
		return nil
	}

	// Get sorted list of profile names
	var profileNames []string

	for profileName := range profileGroups {
		profileNames = append(profileNames, profileName)
	}

	sort.Strings(profileNames)

	// Calculate maximum column widths from all instances
	maxNameWidth := len("Name") // Start with header width
	maxINstanceWidth := len("Instance ID")
	maxHostWidth := len("Host")

	// Iterate through all instances to find maximum widths
	for _, instances := range profileGroups {
		for _, row := range instances {
			// Calculate name width (including "*" for default)
			name := row.InstanceName

			if row.IsDefault {
				name = "*" + name
			}

			if len(name) > maxNameWidth {
				maxNameWidth = len(name)
			}

			// Calculate instance ID width
			if len(row.Instance.ID) > maxINstanceWidth {
				maxINstanceWidth = len(row.Instance.ID)
			}

			// Calculate host width
			if len(row.Instance.Host) > maxHostWidth {
				maxHostWidth = len(row.Instance.Host)
			}
		}
	}

	// Add 2 characters padding for readability
	const padding = 2
	colNameWidth := maxNameWidth + padding
	colInstanceWidth := maxINstanceWidth + padding
	colHostWidth := maxHostWidth + padding

	// Helper function to truncate string to width
	truncate := func(s string, width int) string {
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
		instances := profileGroups[profileName]

		// Print profile header
		if i > 0 {
			fmt.Println()
		}

		fmt.Printf("%sProfile: %s%s\n", bold, profileName, reset)

		// Print top border
		fmt.Printf("┌%s┬%s┬%s┐\n",
			strings.Repeat("─", colNameWidth),
			strings.Repeat("─", colInstanceWidth),
			strings.Repeat("─", colHostWidth))

		// Print header row
		fmt.Printf("│%s%s%s│%s%s%s│%s%s%s│\n",
			bold, truncate("Name", colNameWidth), reset,
			bold, truncate("Instance ID", colInstanceWidth), reset,
			bold, truncate("Host", colHostWidth), reset)

		// Print separator between header and data
		fmt.Printf("├%s┼%s┼%s┤\n",
			strings.Repeat("─", colNameWidth),
			strings.Repeat("─", colInstanceWidth),
			strings.Repeat("─", colHostWidth))

		// Print data rows
		for _, row := range instances {
			name := row.InstanceName

			if row.IsDefault {
				name = "*" + name
			}

			fmt.Printf("│%s│%s│%s│\n",
				truncate(name, colNameWidth),
				truncate(row.Instance.ID, colInstanceWidth),
				truncate(row.Instance.Host, colHostWidth))
		}

		// Print bottom border
		fmt.Printf("└%s┴%s┴%s┘\n",
			strings.Repeat("─", colNameWidth),
			strings.Repeat("─", colInstanceWidth),
			strings.Repeat("─", colHostWidth))
	}

	fmt.Println()

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func addInstance(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("instances add", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")
	instanceName := flagSet.String("name", "", "--name <instance name>")
	instanceNameShort := flagSet.String("n", "", "--name <instance name>")
	filterFlag := flagSet.String("filter", "", "--filter <filter text>")
	filterShort := flagSet.String("f", "", "--filter <filter text>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo instances add [--profile <aws cli profile>] [--name <instance name>] [--filter <filter text>]")
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

	if profileInfo.Instances == nil {
		profileInfo.Instances = make(map[string]Instance)
	}

	reader := bufio.NewReader(os.Stdin)

	// Get filter text
	var filter string
	if *filterFlag != "" {
		filter = *filterFlag
	} else if *filterShort != "" {
		filter = *filterShort
	} else {
		// Prompt user for filter text
		fmt.Print("Enter filter text: ")
		filterInput, _ := reader.ReadString('\n')
		filter = strings.TrimSpace(filterInput)
		if filter == "" {
			return fmt.Errorf("filter text cannot be empty")
		}
	}

	// Query EC2 instances
	fmt.Println("\nQuerying EC2 instances...")
	instances, err := queryEC2Instances(currentProfile, filter)
	if err != nil {
		return fmt.Errorf("failed to query EC2 instances: %v", err)
	}

	if len(instances) == 0 {
		return fmt.Errorf("no EC2 instances found matching filter '%s'", filter)
	}

	// Display instances in a formatted table
	fmt.Println("\nAvailable EC2 instances:")

	// Calculate maximum column widths
	maxNumWidth := len("#")
	maxNameWidth := len("Name")
	maxInstanceWidth := len("Instance ID")
	maxHostWidth := len("Host")
	maxStateWidth := len("State")
	maxTypeWidth := len("Type")
	maxPublicIPWidth := len("Public IP")
	maxLaunchTimeWidth := len("Launch Time")

	for i, inst := range instances {
		// Number width (for selection)
		numStr := strconv.Itoa(i + 1)
		if len(numStr) > maxNumWidth {
			maxNumWidth = len(numStr)
		}

		name := inst.Name
		if name == "" {
			name = "(no name)"
		}

		if len(name) > maxNameWidth {
			maxNameWidth = len(name)
		}

		if len(inst.Instance) > maxInstanceWidth {
			maxInstanceWidth = len(inst.Instance)
		}

		host := inst.Host
		if host == "" {
			host = "(no host)"
		}

		if len(host) > maxHostWidth {
			maxHostWidth = len(host)
		}

		state := inst.State
		if state == "" {
			state = "(unknown)"
		}

		if len(state) > maxStateWidth {
			maxStateWidth = len(state)
		}

		instanceType := inst.InstanceType
		if instanceType == "" {
			instanceType = "(unknown)"
		}

		if len(instanceType) > maxTypeWidth {
			maxTypeWidth = len(instanceType)
		}

		publicIP := inst.PublicIP
		if publicIP == "" {
			publicIP = "(none)"
		}

		if len(publicIP) > maxPublicIPWidth {
			maxPublicIPWidth = len(publicIP)
		}

		launchTime := formatLaunchTime(inst.LaunchTime)
		if len(launchTime) > maxLaunchTimeWidth {
			maxLaunchTimeWidth = len(launchTime)
		}
	}

	// Add 2 characters padding for readability
	const padding = 2
	colNumWidth := maxNumWidth + padding
	colNameWidth := maxNameWidth + padding
	colInstanceWidth := maxInstanceWidth + padding
	colHostWidth := maxHostWidth + padding
	colStateWidth := maxStateWidth + padding
	colTypeWidth := maxTypeWidth + padding
	colPublicIPWidth := maxPublicIPWidth + padding
	colLaunchTimeWidth := maxLaunchTimeWidth + padding

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

	// Print top border
	fmt.Printf("┌%s┬%s┬%s┬%s┬%s┬%s┬%s┬%s┐\n",
		strings.Repeat("─", colNumWidth),
		strings.Repeat("─", colNameWidth),
		strings.Repeat("─", colInstanceWidth),
		strings.Repeat("─", colHostWidth),
		strings.Repeat("─", colStateWidth),
		strings.Repeat("─", colTypeWidth),
		strings.Repeat("─", colPublicIPWidth),
		strings.Repeat("─", colLaunchTimeWidth))

	// Print header row
	fmt.Printf("│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│\n",
		bold, truncate("#", colNumWidth), reset,
		bold, truncate("Name", colNameWidth), reset,
		bold, truncate("Instance ID", colInstanceWidth), reset,
		bold, truncate("Host", colHostWidth), reset,
		bold, truncate("State", colStateWidth), reset,
		bold, truncate("Type", colTypeWidth), reset,
		bold, truncate("Public IP", colPublicIPWidth), reset,
		bold, truncate("Launch Time", colLaunchTimeWidth), reset)

	// Print separator between header and data
	fmt.Printf("├%s┼%s┼%s┼%s┼%s┼%s┼%s┼%s┤\n",
		strings.Repeat("─", colNumWidth),
		strings.Repeat("─", colNameWidth),
		strings.Repeat("─", colInstanceWidth),
		strings.Repeat("─", colHostWidth),
		strings.Repeat("─", colStateWidth),
		strings.Repeat("─", colTypeWidth),
		strings.Repeat("─", colPublicIPWidth),
		strings.Repeat("─", colLaunchTimeWidth))

	// Print data rows
	for i, inst := range instances {
		name := inst.Name
		if name == "" {
			name = "(no name)"
		}

		host := inst.Host
		if host == "" {
			host = "(no host)"
		}

		state := inst.State
		if state == "" {
			state = "(unknown)"
		}

		instanceType := inst.InstanceType
		if instanceType == "" {
			instanceType = "(unknown)"
		}

		publicIP := inst.PublicIP
		if publicIP == "" {
			publicIP = "(none)"
		}

		launchTime := formatLaunchTime(inst.LaunchTime)

		fmt.Printf("│%s│%s│%s│%s│%s│%s│%s│%s│\n",
			formatInt(i+1, colNumWidth),
			truncate(name, colNameWidth),
			truncate(inst.Instance, colInstanceWidth),
			truncate(host, colHostWidth),
			truncate(state, colStateWidth),
			truncate(instanceType, colTypeWidth),
			truncate(publicIP, colPublicIPWidth),
			truncate(launchTime, colLaunchTimeWidth))
	}

	// Print bottom border
	fmt.Printf("└%s┴%s┴%s┴%s┴%s┴%s┴%s┴%s┘\n",
		strings.Repeat("─", colNumWidth),
		strings.Repeat("─", colNameWidth),
		strings.Repeat("─", colInstanceWidth),
		strings.Repeat("─", colHostWidth),
		strings.Repeat("─", colStateWidth),
		strings.Repeat("─", colTypeWidth),
		strings.Repeat("─", colPublicIPWidth),
		strings.Repeat("─", colLaunchTimeWidth))

	fmt.Print("\nSelect instance number: ")
	instSelection, _ := reader.ReadString('\n')
	instIndex, err := strconv.Atoi(strings.TrimSpace(instSelection))

	if err != nil || instIndex < 1 || instIndex > len(instances) {
		return fmt.Errorf("invalid selection")
	}

	selectedInstance := instances[instIndex-1]

	// Get instance name
	var targetInstanceName string
	if *instanceName != "" {
		targetInstanceName = *instanceName
	} else if *instanceNameShort != "" {
		targetInstanceName = *instanceNameShort
	} else {
		fmt.Print("\nEnter instance name: ")
		nameInput, _ := reader.ReadString('\n')
		targetInstanceName = strings.TrimSpace(nameInput)

		if targetInstanceName == "" {
			// Generate a default name from instance name
			targetInstanceName = selectedInstance.Name

			if targetInstanceName == "" {
				targetInstanceName = fmt.Sprintf("instance-%d", len(profileInfo.Instances)+1)
			}
		}
	}

	// Check if name already exists
	if _, exists := profileInfo.Instances[targetInstanceName]; exists {
		return fmt.Errorf("instance '%s' already exists in profile '%s'", targetInstanceName, currentProfile)
	}

	// Get host (private IP)
	host := selectedInstance.Host
	if host == "" {
		// Fallback to instance ID if no private IP available
		host = selectedInstance.Instance
	}

	// Create instance configuration
	newInstance := Instance{
		Name:    targetInstanceName,
		ID:      selectedInstance.Instance,
		Profile: currentProfile,
		Host:    host,
	}

	// Save to configuration
	profileInfo.Instances[targetInstanceName] = newInstance
	profileInfo.Name = currentProfile
	config.Profiles[currentProfile] = profileInfo

	fmt.Printf("\nInstance '%s' (ID: %s) added successfully!\n", targetInstanceName, selectedInstance.Instance)

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func updateInstance(args []string, config *Configuration) {
	flagSet := flag.NewFlagSet("instances update", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")
	instanceName := flagSet.String("name", "", "--name <instance name>")
	instanceNameShort := flagSet.String("n", "", "--name <instance name>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo instances update [--profile <aws cli profile>] [--name <instance name>] [<filter string>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return
	}

	currentProfile, err := ensureProfile(config, profile, profileShort)
	if err != nil {
		return
	}

	// Ensure that we're logged in before running the command
	if !isLoggedIn(currentProfile) {
		loginArgs := []string{"--profile", currentProfile}
		if err := login(loginArgs, config); err != nil {
			return
		}
	}

	profileInfo := config.Profiles[currentProfile]
	if profileInfo.Instances == nil {
		profileInfo.Instances = make(map[string]Instance)
	}

	reader := bufio.NewReader(os.Stdin)

	// Get instance name
	var targetInstanceName string
	if *instanceName != "" {
		targetInstanceName = *instanceName
	} else if *instanceNameShort != "" {
		targetInstanceName = *instanceNameShort
	} else {
		// Prompt for instance name
		fmt.Print("Enter instance name to update: ")
		nameInput, _ := reader.ReadString('\n')
		targetInstanceName = strings.TrimSpace(nameInput)

		if targetInstanceName == "" {
			return
		}
	}

	// Check if instance exists
	existingInstance, exists := profileInfo.Instances[targetInstanceName]
	if !exists {
		return
	}

	// Get filter string (optional - if not provided, prompt for it)
	var filter string
	if len(flagSet.Args()) > 0 {
		filter = flagSet.Args()[0]
	} else {
		// Prompt for filter string
		fmt.Print("Enter instance filter string (or press Enter to use existing instance ID): ")
		filterInput, _ := reader.ReadString('\n')
		filter = strings.TrimSpace(filterInput)

		if filter == "" {
			// Use existing instance ID as default filter
			filter = existingInstance.ID
		}
	}

	// Query EC2 instances
	fmt.Println("\nQuerying EC2 instances...")
	instances, err := queryEC2Instances(currentProfile, filter)
	if err != nil {
		return
	}

	if len(instances) == 0 {
		return
	}

	// Display instances in a formatted table
	fmt.Println("\nAvailable EC2 instances:")

	// Calculate maximum column widths
	maxNumWidth := len("#")
	maxNameWidth := len("Name")
	maxInstanceWidth := len("Instance ID")
	maxHostWidth := len("Host")
	maxStateWidth := len("State")
	maxTypeWidth := len("Type")
	maxPublicIPWidth := len("Public IP")
	maxLaunchTimeWidth := len("Launch Time")

	for i, inst := range instances {
		// Number width (for selection)
		numStr := strconv.Itoa(i + 1)
		if len(numStr) > maxNumWidth {
			maxNumWidth = len(numStr)
		}

		name := inst.Name
		if name == "" {
			name = "(no name)"
		}

		if len(name) > maxNameWidth {
			maxNameWidth = len(name)
		}

		if len(inst.Instance) > maxInstanceWidth {
			maxInstanceWidth = len(inst.Instance)
		}

		host := inst.Host
		if host == "" {
			host = "(no host)"
		}

		if len(host) > maxHostWidth {
			maxHostWidth = len(host)
		}

		state := inst.State
		if state == "" {
			state = "(unknown)"
		}

		if len(state) > maxStateWidth {
			maxStateWidth = len(state)
		}

		instanceType := inst.InstanceType
		if instanceType == "" {
			instanceType = "(unknown)"
		}

		if len(instanceType) > maxTypeWidth {
			maxTypeWidth = len(instanceType)
		}

		publicIP := inst.PublicIP
		if publicIP == "" {
			publicIP = "(none)"
		}

		if len(publicIP) > maxPublicIPWidth {
			maxPublicIPWidth = len(publicIP)
		}

		launchTime := formatLaunchTime(inst.LaunchTime)
		if len(launchTime) > maxLaunchTimeWidth {
			maxLaunchTimeWidth = len(launchTime)
		}
	}

	// Add 2 characters padding for readability
	const padding = 2
	colNumWidth := maxNumWidth + padding
	colNameWidth := maxNameWidth + padding
	colInstanceWidth := maxInstanceWidth + padding
	colHostWidth := maxHostWidth + padding
	colStateWidth := maxStateWidth + padding
	colTypeWidth := maxTypeWidth + padding
	colPublicIPWidth := maxPublicIPWidth + padding
	colLaunchTimeWidth := maxLaunchTimeWidth + padding

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

	// Print top border
	fmt.Printf("┌%s┬%s┬%s┬%s┬%s┬%s┬%s┬%s┐\n",
		strings.Repeat("─", colNumWidth),
		strings.Repeat("─", colNameWidth),
		strings.Repeat("─", colInstanceWidth),
		strings.Repeat("─", colHostWidth),
		strings.Repeat("─", colStateWidth),
		strings.Repeat("─", colTypeWidth),
		strings.Repeat("─", colPublicIPWidth),
		strings.Repeat("─", colLaunchTimeWidth))

	// Print header row
	fmt.Printf("│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│%s%s%s│\n",
		bold, truncate("#", colNumWidth), reset,
		bold, truncate("Name", colNameWidth), reset,
		bold, truncate("Instance ID", colInstanceWidth), reset,
		bold, truncate("Host", colHostWidth), reset,
		bold, truncate("State", colStateWidth), reset,
		bold, truncate("Type", colTypeWidth), reset,
		bold, truncate("Public IP", colPublicIPWidth), reset,
		bold, truncate("Launch Time", colLaunchTimeWidth), reset)

	// Print separator between header and data
	fmt.Printf("├%s┼%s┼%s┼%s┼%s┼%s┼%s┼%s┤\n",
		strings.Repeat("─", colNumWidth),
		strings.Repeat("─", colNameWidth),
		strings.Repeat("─", colInstanceWidth),
		strings.Repeat("─", colHostWidth),
		strings.Repeat("─", colStateWidth),
		strings.Repeat("─", colTypeWidth),
		strings.Repeat("─", colPublicIPWidth),
		strings.Repeat("─", colLaunchTimeWidth))

	// Print data rows
	for i, inst := range instances {
		name := inst.Name
		if name == "" {
			name = "(no name)"
		}

		host := inst.Host
		if host == "" {
			host = "(no host)"
		}

		state := inst.State
		if state == "" {
			state = "(unknown)"
		}

		instanceType := inst.InstanceType
		if instanceType == "" {
			instanceType = "(unknown)"
		}

		publicIP := inst.PublicIP
		if publicIP == "" {
			publicIP = "(none)"
		}

		launchTime := formatLaunchTime(inst.LaunchTime)

		fmt.Printf("│%s│%s│%s│%s│%s│%s│%s│%s│\n",
			formatInt(i+1, colNumWidth),
			truncate(name, colNameWidth),
			truncate(inst.Instance, colInstanceWidth),
			truncate(host, colHostWidth),
			truncate(state, colStateWidth),
			truncate(instanceType, colTypeWidth),
			truncate(publicIP, colPublicIPWidth),
			truncate(launchTime, colLaunchTimeWidth))
	}

	// Print bottom border
	fmt.Printf("└%s┴%s┴%s┴%s┴%s┴%s┴%s┴%s┘\n",
		strings.Repeat("─", colNumWidth),
		strings.Repeat("─", colNameWidth),
		strings.Repeat("─", colInstanceWidth),
		strings.Repeat("─", colHostWidth),
		strings.Repeat("─", colStateWidth),
		strings.Repeat("─", colTypeWidth),
		strings.Repeat("─", colPublicIPWidth),
		strings.Repeat("─", colLaunchTimeWidth))

	var selectedInstance EC2Instance

	// Auto-select instance if only one result and no filter provided
	if len(instances) == 1 {
		selectedInstance = instances[0]
		fmt.Printf("\nAutomatically selected instance '%s' (ID: %s) since it's the only result.\n", selectedInstance.Name, selectedInstance.Instance)
	} else {
		fmt.Print("\nSelect instance number: ")
		instSelection, _ := reader.ReadString('\n')
		instIndex, err := strconv.Atoi(strings.TrimSpace(instSelection))

		if err != nil || instIndex < 1 || instIndex > len(instances) {
			return
		}

		selectedInstance = instances[instIndex-1]
	}

	// Update instance configuration
	// Preserve Name and Profile, update ID and Host
	updatedInstance := Instance{
		Name:    targetInstanceName,
		ID:      selectedInstance.Instance,
		Profile: currentProfile,
		Host:    selectedInstance.Host,
	}

	// If Host is empty, use instance ID as fallback
	if updatedInstance.Host == "" {
		updatedInstance.Host = selectedInstance.Instance
	}

	// Save to configuration
	profileInfo.Instances[targetInstanceName] = updatedInstance
	profileInfo.Name = currentProfile
	config.Profiles[currentProfile] = profileInfo

	fmt.Printf("\nInstance '%s' (ID: %s) updated successfully!\n", targetInstanceName, selectedInstance.Instance)

	return
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func removeInstance(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("instances remove", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")
	instanceName := flagSet.String("name", "", "--name <instance name>")
	instanceNameShort := flagSet.String("n", "", "--name <instance name>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:\n    awsdo instances remove [--profile <aws cli profile>] [--name <instance name>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	currentProfile, err := ensureProfile(config, profile, profileShort)
	if err != nil {
		return err
	}

	profileInfo := config.Profiles[currentProfile]

	if len(profileInfo.Instances) == 0 {
		return fmt.Errorf("no instances configured for profile '%s'", currentProfile)
	}

	reader := bufio.NewReader(os.Stdin)

	// Get instance name
	var targetInstanceName string

	switch {
	case *instanceName != "":
		targetInstanceName = *instanceName
	case *instanceNameShort != "":
		targetInstanceName = *instanceNameShort
	case flagSet.NArg() == 1:
		targetInstanceName = flagSet.Arg(0)
	default:
		// Prompt for instance name
		fmt.Print("Enter instance name to remove: ")
		nameInput, _ := reader.ReadString('\n')
		targetInstanceName = strings.TrimSpace(nameInput)

		if targetInstanceName == "" {
			return fmt.Errorf("instance name is required")
		}
	}

	// Check if instance exists
	existingInstance, exists := profileInfo.Instances[targetInstanceName]
	if !exists {
		return fmt.Errorf("instance '%s' not found in profile '%s'", targetInstanceName, currentProfile)
	}

	// Display instance information
	fmt.Printf("\nInstance to remove:\n")
	fmt.Printf("  Name:    %s\n", targetInstanceName)
	fmt.Printf("  ID:      %s\n", existingInstance.ID)
	fmt.Printf("  Profile: %s\n", existingInstance.Profile)
	fmt.Printf("  Host:    %s\n", existingInstance.Host)

	// Ask for confirmation
	fmt.Print("\nAre you sure you want to remove this instance? (yes/no): ")
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.TrimSpace(strings.ToLower(confirmation))

	if confirmation != "yes" && confirmation != "y" {
		fmt.Println("Removal cancelled.")
		return nil
	}

	// Remove from Instances map
	delete(profileInfo.Instances, targetInstanceName)

	// If this was the default instance, clear the DefaultInstance field
	if profileInfo.DefaultInstance == targetInstanceName {
		profileInfo.DefaultInstance = ""
	}

	// Update profile in config
	profileInfo.Name = currentProfile
	config.Profiles[currentProfile] = profileInfo

	fmt.Printf("\nInstance '%s' removed successfully!\n", targetInstanceName)

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func ensureProfile(config *Configuration, profile *string, profileShort *string) (string, error) {
	// Initialize Profiles map if nil
	if config.Profiles == nil {
		config.Profiles = make(map[string]Profile)
	}

	currentProfile := config.DefaultProfile

	if len(*profile) != 0 {
		currentProfile = *profile
	} else if len(*profileShort) != 0 {
		currentProfile = *profileShort
	}

	// Set the default profile in the configuration if currentProfile is not empty
	// Otherewise fail with an error
	if len(currentProfile) != 0 && currentProfile != config.DefaultProfile {
		config.DefaultProfile = currentProfile

		// Ensure profile exists in map
		if _, exists := config.Profiles[currentProfile]; !exists {
			config.Profiles[currentProfile] = Profile{
				Name:      currentProfile,
				Bastions:  make(map[string]Bastion),
				Instances: make(map[string]Instance),
			}
		}
	} else if len(config.DefaultProfile) == 0 {
		return "", fmt.Errorf("must specify the target profile")
	}

	return currentProfile, nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func selectInstanceByName(profileInfo Profile, name string) (Instance, error) {
	if len(profileInfo.Instances) == 0 {
		return Instance{}, fmt.Errorf("no instances configured for this profile")
	}

	// If name is provided, use it
	if name != "" {
		if instance, exists := profileInfo.Instances[name]; exists {
			return instance, nil
		}

		return Instance{}, fmt.Errorf("instance '%s' not found", name)
	}

	// If no name provided, try default
	if profileInfo.DefaultInstance != "" {
		if instance, exists := profileInfo.Instances[profileInfo.DefaultInstance]; exists {
			return instance, nil
		}
	}

	// If only one instance exists, use it
	if len(profileInfo.Instances) == 1 {
		for _, instance := range profileInfo.Instances {
			return instance, nil
		}
	}

	// Multiple instances exist, need to specify name
	return Instance{}, fmt.Errorf("multiple instances available, please specify instance name")
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func selectInstanceByHost(profileInfo Profile, host string) (Instance, error) {
	if len(profileInfo.Instances) == 0 {
		return Instance{}, fmt.Errorf("no instances configured for this profile")
	}

	// Search for instance with matching host
	for _, instance := range profileInfo.Instances {
		if instance.Host == host {
			return instance, nil
		}
	}

	return Instance{}, fmt.Errorf("instance with host '%s' not found", host)
}
