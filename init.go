package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// initCommand is the main entry point for the init command
func initCommand(config *Configuration) error {
	fmt.Println("\n=== AWSDO Initialization ===")
	fmt.Println("This command will help you set up AWS CLI, SSM plugin, and configure your first AWS SSO profile.")
	fmt.Println()

	// Check prerequisites
	fmt.Println("Checking prerequisites...")
	awsCLIInstalled := checkAWSCLI()
	ssmPluginInstalled := checkSSMPlugin()
	hasProfiles := checkAWSConfig()

	fmt.Println()

	if awsCLIInstalled {
		fmt.Println("✓ AWS CLI is installed")
	} else {
		fmt.Println("✗ AWS CLI is not installed")
	}

	if ssmPluginInstalled {
		fmt.Println("✓ SSM Plugin is installed")
	} else {
		fmt.Println("✗ SSM Plugin is not installed")
	}

	if hasProfiles {
		fmt.Println("✓ AWS profiles are configured")
	} else {
		fmt.Println("✗ No AWS profiles found")
	}

	fmt.Println()

	// Install AWS CLI if needed
	if !awsCLIInstalled {
		fmt.Println("Installing AWS CLI...")

		if err := installAWSCLI(); err != nil {
			return fmt.Errorf("failed to install AWS CLI: %v", err)
		}

		fmt.Println("✓ AWS CLI installation completed")
		fmt.Println()
	}

	// Install SSM plugin if needed
	if !ssmPluginInstalled {
		fmt.Println("Installing SSM Plugin...")

		if err := installSSMPlugin(); err != nil {
			return fmt.Errorf("failed to install SSM Plugin: %v", err)
		}

		fmt.Println("✓ SSM Plugin installation completed")
		fmt.Println()
	}

	// Set up profile if needed
	if !hasProfiles {
		fmt.Println("Setting up your first AWS SSO profile...")

		if err := setupProfile(config); err != nil {
			return fmt.Errorf("failed to set up profile: %v", err)
		}

		fmt.Println("✓ Profile setup completed")
		fmt.Println()
	}

	fmt.Println("=== Initialization Complete ===")
	fmt.Println()
	fmt.Println("You're all set! You can now use awsdo commands.")

	if !hasProfiles {
		fmt.Println("To log in, use: awsdo login")
	}

	fmt.Println()

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// checkAWSCLI checks if AWS CLI is installed and accessible
func checkAWSCLI() bool {
	cmd := exec.Command("aws", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// checkSSMPlugin checks if SSM plugin is installed and accessible
func checkSSMPlugin() bool {
	cmd := exec.Command("session-manager-plugin", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// checkAWSConfig checks if AWS config file exists and has profiles
func checkAWSConfig() bool {
	configPath := getAWSConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return false
	}

	// Read config file and check for profile sections
	file, err := os.Open(configPath)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[profile ") || strings.HasPrefix(line, "[default]") {
			return true
		}
	}

	return false
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// getAWSConfigPath returns the path to the AWS config file
func getAWSConfigPath() string {
	homeDir := getUserHomeDir()
	awsDir := filepath.Join(homeDir, ".aws")
	configPath := filepath.Join(awsDir, "config")

	// Ensure .aws directory exists
	os.MkdirAll(awsDir, 0755)

	return configPath
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// getUserHomeDir returns the user's home directory
func getUserHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		// Fallback to environment variable
		if runtime.GOOS == "windows" {
			return os.Getenv("USERPROFILE")
		}
		return os.Getenv("HOME")
	}
	return usr.HomeDir
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// installAWSCLI installs AWS CLI based on the operating system
func installAWSCLI() error {
	switch runtime.GOOS {
	case "windows":
		return installAWSCLIWindows()
	case "darwin":
		return installAWSCLIMacOS()
	case "linux":
		return installAWSCLILinux()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// installAWSCLIWindows installs AWS CLI on Windows
func installAWSCLIWindows() error {
	// Check for winget
	wingetCmd := exec.Command("winget", "--version")
	if wingetCmd.Run() == nil {
		fmt.Println("Detected winget. Installing AWS CLI via winget...")
		cmd := exec.Command("winget", "install", "-e", "--id", "Amazon.AWSCLI")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("aws cli installation failed: %v", err)
		}

		// Verify installation
		if !checkAWSCLI() {
			return fmt.Errorf("the AWS CLI installation completed but it was not accessible. Please restart your terminal or add AWS CLI to PATH manually")
		}

		return nil
	}

	// Manual installation guide
	fmt.Println("winget not found. Please install AWS CLI manually:")
	fmt.Println()
	fmt.Println("Option 1: Install with winget (recommended)")
	fmt.Println("  Visit: https://winget.run to install winget")
	fmt.Println("  Then run: winget install -e --id Amazon.AWSCLI")
	fmt.Println()
	fmt.Println("Option 2: Download MSI installer")
	fmt.Println("  Visit: https://awscli.amazonaws.com/AWSCLIV2.msi")
	fmt.Println("  Download and run the installer")
	fmt.Println()
	fmt.Print("Press Enter after you have installed AWS CLI...")

	readUserInput()

	// Verify installation
	if !checkAWSCLI() {
		return fmt.Errorf("the AWS CLI not found. Please ensure it is installed and accessible")
	}

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// installAWSCLIMacOS installs AWS CLI on macOS
func installAWSCLIMacOS() error {
	// Check for Homebrew
	brewCmd := exec.Command("brew", "--version")
	if brewCmd.Run() == nil {
		fmt.Println("Detected Homebrew. Installing AWS CLI via Homebrew...")
		cmd := exec.Command("brew", "install", "awscli")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("homebrew installation failed: %v", err)
		}

		// Verify installation
		if !checkAWSCLI() {
			return fmt.Errorf("aws cli installation completed but not found in PATH. Please restart your terminal")
		}
		return nil
	}

	// Manual installation guide
	fmt.Println("Homebrew not found. Please install AWS CLI manually:")
	fmt.Println()
	fmt.Println("Option 1: Install Homebrew (recommended)")
	fmt.Println("  Visit: https://brew.sh")
	fmt.Println("  Then run: brew install awscli")
	fmt.Println()
	fmt.Println("Option 2: Download installer")
	fmt.Println("  Visit: https://awscli.amazonaws.com/AWSCLIV2.pkg")
	fmt.Println("  Download and run the installer")
	fmt.Println()
	fmt.Print("Press Enter after you have installed AWS CLI...")
	readUserInput()

	// Verify installation
	if !checkAWSCLI() {
		return fmt.Errorf("aws cli not found. Please ensure it is installed and accessible")
	}

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// installAWSCLILinux installs AWS CLI on Linux
func installAWSCLILinux() error {
	// Detect package manager
	var installCmd *exec.Cmd

	// Check for apt (Debian/Ubuntu)
	if cmd := exec.Command("apt", "--version"); cmd.Run() == nil {
		fmt.Println("Detected apt. Installing AWS CLI...")
		// First update package list
		updateCmd := exec.Command("sudo", "apt", "update")
		updateCmd.Stdout = os.Stdout
		updateCmd.Stderr = os.Stderr
		updateCmd.Run()

		installCmd = exec.Command("sudo", "apt", "install", "-y", "awscli")
	} else if cmd := exec.Command("yum", "--version"); cmd.Run() == nil {
		// Check for yum (RHEL/CentOS 7)
		fmt.Println("Detected yum. Installing AWS CLI...")
		installCmd = exec.Command("sudo", "yum", "install", "-y", "awscli")
	} else if cmd := exec.Command("dnf", "--version"); cmd.Run() == nil {
		// Check for dnf (RHEL/CentOS 8+/Fedora)
		fmt.Println("Detected dnf. Installing AWS CLI...")
		installCmd = exec.Command("sudo", "dnf", "install", "-y", "awscli")
	} else if cmd := exec.Command("zypper", "--version"); cmd.Run() == nil {
		// Check for zypper (openSUSE)
		fmt.Println("Detected zypper. Installing AWS CLI...")
		installCmd = exec.Command("sudo", "zypper", "install", "-y", "aws-cli")
	}

	if installCmd != nil {
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("package manager installation failed: %v", err)
		}

		// Verify installation
		if !checkAWSCLI() {
			return fmt.Errorf("aws cli installation completed but not found in PATH. Please restart your terminal")
		}
		return nil
	}

	// Manual installation guide
	fmt.Println("No supported package manager found. Please install AWS CLI manually:")
	fmt.Println()
	fmt.Println("For Debian/Ubuntu:")
	fmt.Println("  sudo apt update && sudo apt install awscli")
	fmt.Println()
	fmt.Println("For RHEL/CentOS/Fedora:")
	fmt.Println("  sudo yum install awscli  # or sudo dnf install awscli")
	fmt.Println()
	fmt.Println("For other distributions, visit:")
	fmt.Println("  https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html")
	fmt.Println()
	fmt.Print("Press Enter after you have installed AWS CLI...")
	readUserInput()

	// Verify installation
	if !checkAWSCLI() {
		return fmt.Errorf("the aws cli not found. Please ensure it is installed and accessible")
	}

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// installSSMPlugin installs SSM plugin based on the operating system
func installSSMPlugin() error {
	switch runtime.GOOS {
	case "windows":
		return installSSMPluginWindows()
	case "darwin":
		return installSSMPluginMacOS()
	case "linux":
		return installSSMPluginLinux()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// installSSMPluginWindows installs SSM plugin on Windows
func installSSMPluginWindows() error {
	// Check for winget
	wingetCmd := exec.Command("winget", "--version")
	if wingetCmd.Run() == nil {
		fmt.Println("Detected winget. Installing SSM Plugin via winget...")
		cmd := exec.Command("winget", "install", "-e", "--id", "Amazon.SessionManagerPlugin")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("ssm plugin installation failed: %v", err)
		}

		// Verify installation
		if !checkSSMPlugin() {
			return fmt.Errorf("the SSM Plugin installation completed but not found in PATH. Please restart your terminal")
		}
		return nil
	}

	// Download and install EXE
	fmt.Println("Downloading SSM Plugin installer...")
	homeDir := getUserHomeDir()
	exePath := filepath.Join(homeDir, "SessionManagerPluginSetup.exe")

	// Download EXE
	url := "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/windows/SessionManagerPluginSetup.exe"
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download SSM Plugin: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download SSM Plugin: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(exePath)
	if err != nil {
		return fmt.Errorf("failed to create installer file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save installer: %v", err)
	}

	fmt.Println("Installing SSM Plugin...")
	fmt.Println("Please follow the installation wizard that will open.")
	// Try silent install first, fall back to interactive if needed
	cmd := exec.Command(exePath, "/S")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Silent install failed, try interactive
		fmt.Println("Silent install failed, trying interactive installation...")
		cmd = exec.Command(exePath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			os.Remove(exePath)
			return fmt.Errorf("installation failed: %v. Please install manually from the downloaded file", err)
		}
	}

	// Clean up installer
	os.Remove(exePath)

	// Verify installation
	if !checkSSMPlugin() {
		return fmt.Errorf("the SSM Plugin installation completed but not found in PATH. Please restart your terminal")
	}

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// installSSMPluginMacOS installs SSM plugin on macOS
func installSSMPluginMacOS() error {
	// Check for Homebrew
	brewCmd := exec.Command("brew", "--version")
	if brewCmd.Run() == nil {
		fmt.Println("Detected Homebrew. Installing SSM Plugin via Homebrew...")
		cmd := exec.Command("brew", "install", "--cask", "session-manager-plugin")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("homebrew installation failed: %v", err)
		}

		// Verify installation
		if !checkSSMPlugin() {
			return fmt.Errorf("the SSM Plugin installation completed but not found in PATH. Please restart your terminal")
		}
		return nil
	}

	// Manual installation guide
	fmt.Println("Homebrew not found. Please install SSM Plugin manually:")
	fmt.Println()
	fmt.Println("Option 1: Install Homebrew (recommended)")
	fmt.Println("  Visit: https://brew.sh")
	fmt.Println("  Then run: brew install --cask session-manager-plugin")
	fmt.Println()
	fmt.Println("Option 2: Download and install manually")
	fmt.Println("  Visit: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	fmt.Println()
	fmt.Print("Press Enter after you have installed SSM Plugin...")
	readUserInput()

	// Verify installation
	if !checkSSMPlugin() {
		return fmt.Errorf("the SSM Plugin not found. Please ensure it is installed and accessible")
	}

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// installSSMPluginLinux installs SSM plugin on Linux
func installSSMPluginLinux() error {
	homeDir := getUserHomeDir()
	pluginDir := filepath.Join(homeDir, ".local", "share", "session-manager-plugin")
	pluginPath := filepath.Join(pluginDir, "bin", "session-manager-plugin")

	// Check if already installed in common location
	if _, err := os.Stat(pluginPath); err == nil {
		// Check if it's in PATH
		if checkSSMPlugin() {
			return nil
		}
		// Add to PATH instruction
		fmt.Printf("SSM Plugin found at %s but not in PATH.\n", pluginPath)
		fmt.Println("Add the following to your ~/.bashrc or ~/.zshrc:")
		fmt.Printf("  export PATH=\"$PATH:%s\"\n", filepath.Join(pluginDir, "bin"))
		fmt.Print("Press Enter after you have updated your PATH...")
		readUserInput()
		if checkSSMPlugin() {
			return nil
		}
	}

	// Detect package manager
	var installCmd *exec.Cmd

	// Check for apt (Debian/Ubuntu)
	if cmd := exec.Command("apt", "--version"); cmd.Run() == nil {
		fmt.Println("Detected apt. Installing SSM Plugin...")
		installCmd = exec.Command("sudo", "apt", "install", "-y", "session-manager-plugin")
	} else if cmd := exec.Command("yum", "--version"); cmd.Run() == nil {
		// Check for yum (RHEL/CentOS 7)
		fmt.Println("Detected yum. Installing SSM Plugin...")
		installCmd = exec.Command("sudo", "yum", "install", "-y", "session-manager-plugin")
	} else if cmd := exec.Command("dnf", "--version"); cmd.Run() == nil {
		// Check for dnf (RHEL/CentOS 8+/Fedora)
		fmt.Println("Detected dnf. Installing SSM Plugin...")
		installCmd = exec.Command("sudo", "dnf", "install", "-y", "session-manager-plugin")
	}

	if installCmd != nil {
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			// Package manager install failed, try manual download
			fmt.Println("Package manager installation failed, trying manual download...")
		} else {
			// Verify installation
			if checkSSMPlugin() {
				return nil
			}
		}
	}

	// Manual download and install
	fmt.Println("Downloading SSM Plugin...")
	os.MkdirAll(pluginDir, 0755)

	// Determine architecture
	arch := runtime.GOARCH
	var downloadURL string
	switch arch {
	case "amd64":
		downloadURL = "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_64bit/session-manager-plugin.deb"
	case "arm64":
		downloadURL = "https://s3.amazonaws.com/session-manager-downloads/plugin/latest/ubuntu_arm64/session-manager-plugin.deb"
	default:
		// Fallback to generic Linux installer
		downloadURL = fmt.Sprintf("https://s3.amazonaws.com/session-manager-downloads/plugin/latest/linux_%s/session-manager-plugin.rpm", arch)
	}

	// Try to download
	resp, err := http.Get(downloadURL)
	if err != nil {
		// Manual installation guide
		fmt.Println("Automatic download failed. Please install SSM Plugin manually:")
		fmt.Println()
		fmt.Println("Visit: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
		fmt.Println()
		fmt.Print("Press Enter after you have installed SSM Plugin...")
		readUserInput()
		if !checkSSMPlugin() {
			return fmt.Errorf("the SSM Plugin not found. Please ensure it is installed and accessible")
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Manual installation guide
		fmt.Println("Download failed. Please install SSM Plugin manually:")
		fmt.Println()
		fmt.Println("Visit: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
		fmt.Println()
		fmt.Print("Press Enter after you have installed SSM Plugin...")
		readUserInput()
		if !checkSSMPlugin() {
			return fmt.Errorf("the SSM Plugin not found. Please ensure it is installed and accessible")
		}
		return nil
	}

	// Save downloaded file
	debPath := filepath.Join(homeDir, "session-manager-plugin.deb")
	out, err := os.Create(debPath)
	if err != nil {
		return fmt.Errorf("failed to create installer file: %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save installer: %v", err)
	}

	// Install using dpkg
	fmt.Println("Installing SSM Plugin...")
	cmd := exec.Command("sudo", "dpkg", "-i", debPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(debPath)
		return fmt.Errorf("installation failed: %v", err)
	}

	// Clean up
	os.Remove(debPath)

	// Verify installation
	if !checkSSMPlugin() {
		return fmt.Errorf("SSM Plugin installation completed but not found in PATH. Please restart your terminal.")
	}

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// setupProfile walks the user through setting up an AWS SSO profile (used by init)
func setupProfile(config *Configuration) error {
	return addProfileWithSSOSetup(config, "")
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// addProfileWithSSOSetup walks the user through setting up an AWS SSO profile.
// If profileName is empty, prompts for it (default "default"). If non-empty, uses it directly.
func addProfileWithSSOSetup(config *Configuration, profileName string) error {
	reader := bufio.NewReader(os.Stdin)

	if profileName != "" {
		fmt.Printf("Adding AWS SSO profile '%s'.\n", profileName)
	} else {
		fmt.Println("Let's set up your first AWS SSO profile.")
	}
	fmt.Println("You'll need the following information from your AWS administrator:")
	fmt.Println("  - SSO start URL")
	fmt.Println("  - SSO region")
	fmt.Println("  - Account ID")
	fmt.Println("  - Role name")
	fmt.Println()

	// Get profile name if not provided
	if profileName == "" {
		fmt.Print("Profile name [default]: ")
		name, _ := reader.ReadString('\n')
		profileName = strings.TrimSpace(name)
		if profileName == "" {
			profileName = "default"
		}
	}

	// Get SSO start URL
	fmt.Print("SSO start URL: ")
	ssoStartURL, _ := reader.ReadString('\n')
	ssoStartURL = strings.TrimSpace(ssoStartURL)
	if ssoStartURL == "" {
		return fmt.Errorf("SSO start URL is required")
	}

	// Get SSO region
	fmt.Print("SSO region [us-east-1]: ")
	ssoRegion, _ := reader.ReadString('\n')
	ssoRegion = strings.TrimSpace(ssoRegion)
	if ssoRegion == "" {
		ssoRegion = "us-east-1"
	}

	// Get account ID
	fmt.Print("Account ID: ")
	accountID, _ := reader.ReadString('\n')
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("Account ID is required")
	}

	// Get role name
	fmt.Print("Role name: ")
	roleName, _ := reader.ReadString('\n')
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return fmt.Errorf("Role name is required")
	}

	// Get default region (optional)
	fmt.Print("Default region [us-east-1]: ")
	defaultRegion, _ := reader.ReadString('\n')
	defaultRegion = strings.TrimSpace(defaultRegion)
	if defaultRegion == "" {
		defaultRegion = "us-east-1"
	}

	// Write to AWS config file
	configPath := getAWSConfigPath()
	if err := appendProfileToConfig(configPath, profileName, ssoStartURL, ssoRegion, accountID, roleName, defaultRegion); err != nil {
		return fmt.Errorf("failed to write profile to config: %v", err)
	}

	fmt.Println()
	fmt.Printf("Profile '%s' has been added to your AWS config.\n", profileName)

	// Update awsdo config
	if config.Profiles == nil {
		config.Profiles = make(map[string]Profile)
	}
	if _, exists := config.Profiles[profileName]; !exists {
		config.Profiles[profileName] = Profile{
			Name: profileName,
		}
	}
	if config.DefaultProfile == "" {
		config.DefaultProfile = profileName
		fmt.Printf("Set '%s' as your default profile.\n", profileName)
	}

	// Test the profile
	fmt.Println()
	fmt.Println("Testing profile configuration...")
	fmt.Println("You will be prompted to log in to AWS SSO.")
	fmt.Println()

	testCmd := exec.Command("aws", "sso", "login", "--profile", profileName)
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr
	testCmd.Stdin = os.Stdin

	if err := testCmd.Run(); err != nil {
		fmt.Println()
		fmt.Printf("Login test failed, but profile has been configured. You can try logging in later with: awsdo login -p %s\n", profileName)
		return nil
	}

	fmt.Println()
	fmt.Println("✓ Profile test successful!")

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// readUserInput reads a line from stdin, handling both Windows and Unix line endings
func readUserInput() {
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// appendProfileToConfig appends a profile section to the AWS config file
func appendProfileToConfig(configPath, profileName, ssoStartURL, ssoRegion, accountID, roleName, defaultRegion string) error {
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check if file is empty or doesn't end with newline
	stat, err := file.Stat()
	if err == nil && stat.Size() > 0 {
		// Read last byte to check if it ends with newline
		file.Seek(-1, io.SeekEnd)
		var lastByte [1]byte
		file.Read(lastByte[:])
		if lastByte[0] != '\n' {
			file.WriteString("\n")
		}
		file.Seek(0, io.SeekEnd)
	}

	// Write profile section
	sectionName := profileName
	if profileName != "default" {
		sectionName = "profile " + profileName
	}

	profileConfig := fmt.Sprintf("\n[%s]\n", sectionName)
	profileConfig += fmt.Sprintf("sso_start_url = %s\n", ssoStartURL)
	profileConfig += fmt.Sprintf("sso_region = %s\n", ssoRegion)
	profileConfig += fmt.Sprintf("sso_account_id = %s\n", accountID)
	profileConfig += fmt.Sprintf("sso_role_name = %s\n", roleName)
	profileConfig += fmt.Sprintf("region = %s\n", defaultRegion)

	_, err = file.WriteString(profileConfig)
	return err
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// removeProfileFromAWSConfig removes a profile section from the AWS config file
func removeProfileFromAWSConfig(configPath, profileName string) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	inTargetSection := false
	found := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this line starts a new section
		if strings.HasPrefix(trimmed, "[profile ") {
			sectionName := strings.TrimPrefix(trimmed, "[profile ")
			sectionName = strings.TrimSuffix(sectionName, "]")
			sectionName = strings.TrimSpace(sectionName)
			if sectionName == profileName {
				inTargetSection = true
				found = true
				continue
			}
			inTargetSection = false
		} else if trimmed == "[default]" {
			if profileName == "default" {
				inTargetSection = true
				found = true
				continue
			}
			inTargetSection = false
		} else if inTargetSection {
			// Skip lines belonging to the target section
			continue
		}

		result = append(result, lines[i])
	}

	if !found {
		return fmt.Errorf("profile '%s' not found in config", profileName)
	}

	// Trim trailing empty lines from result
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	newContent := strings.Join(result, "\n")
	if len(newContent) > 0 && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	return os.WriteFile(configPath, []byte(newContent), 0644)
}
