package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func getCredentials(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("get-credentials", flag.ContinueOnError)
	profileFlag := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:")
		fmt.Println("    awsdo get-credentials [--profile <aws cli profile>]")
		fmt.Println("    awsdo get-credentials [-p <aws cli profile>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return fmt.Errorf("USAGE: awsdo get-credentials [--profile <aws cli profile>]")
	}

	currentProfile, err := ensureProfile(config, profileFlag, profileShort)
	if err != nil {
		return err
	}

	// Ensure that we're logged in before running the command.
	if !isLoggedIn(currentProfile) {
		loginArgs := []string{}
		loginArgs = append(loginArgs, "--profile", currentProfile)
		if err := login(loginArgs, config); err != nil {
			return fmt.Errorf("failed to login: %v", err)
		}
	}

	commandArgs := []string{"configure", "export-credentials", "--profile", currentProfile, "--format", "process"}

	command := exec.Command("aws", commandArgs...)
	command.Stderr = os.Stderr

	outputBytes, err := command.Output()
	if err != nil {
		return fmt.Errorf("failed to export credentials: %v", err)
	}

	output := strings.TrimSpace(string(outputBytes))

	if len(output) == 0 {
		return fmt.Errorf("no credentials returned from AWS CLI")
	}

	// Print the formatted JSON output
	var credentials map[string]any
	if err := json.Unmarshal([]byte(output), &credentials); err != nil {
		return fmt.Errorf("failed to parse credentials: %v", err)
	}

	fmt.Println()
	fmt.Println("Access Key:    ", credentials["AccessKeyId"])
	fmt.Println("--------------------------------")
	fmt.Println("Secret Key:    ", credentials["SecretAccessKey"])
	fmt.Println("--------------------------------")
	fmt.Println("Session Token: ", credentials["SessionToken"])
	fmt.Println("--------------------------------")
	fmt.Println("Expiration:    ", credentials["Expiration"])
	fmt.Println("--------------------------------")
	fmt.Println()

	return nil
}
