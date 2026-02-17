package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
)

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// startSSMSession starts a remote SSM terminal session against the specified instance.
func startSSMSession(args []string, config *Configuration) error {
	flagSet := flag.NewFlagSet("terminal", flag.ContinueOnError)
	profile := flagSet.String("profile", "", "--profile <aws cli profile>")
	profileShort := flagSet.String("p", "", "--profile <aws cli profile>")
	instanceHost := flagSet.String("host", "", "--host <instance host>")
	instanceHostShort := flagSet.String("h", "", "--host <instance host>")

	flagSet.Usage = func() {
		fmt.Println("USAGE:")
		fmt.Println("    awsdo terminal [--profile <aws cli profile>] [<instance name>]")
		fmt.Println("    awsdo terminal [--profile <aws cli profile>] [--host <instance host>]")
	}

	if err := flagSet.Parse(args); err != nil {
		return nil
	}

	// Handle instance lookup logic
	var instance Instance
	var currentProfile string
	var err error
	instanceName := ""

	// Check if instance name was provided as positional argument
	if len(flagSet.Args()) > 0 {
		instanceName = flagSet.Args()[0]
	}

	// Handle host lookup
	if *instanceHost != "" || *instanceHostShort != "" {
		host := *instanceHost
		if *instanceHostShort != "" {
			host = *instanceHostShort
		}

		// If profile is specified, look only in that profile
		if *profile != "" || *profileShort != "" {
			currentProfile, err = ensureProfile(config, profile, profileShort)
			if err != nil {
				return err
			}

			profileInfo := config.Profiles[currentProfile]

			if profileInfo.Instances == nil {
				profileInfo.Instances = make(map[string]Instance)
			}

			selectedInstance, err := selectInstanceByHost(profileInfo, host)
			if err != nil {
				return fmt.Errorf("instance with host '%s' not found in profile '%s'", host, currentProfile)
			}

			instance = selectedInstance
		} else {
			// No profile specified - first check default profile, then search all profiles
			found := false

			if config.DefaultProfile != "" {
				// Try default profile first
				if profileInfo, exists := config.Profiles[config.DefaultProfile]; exists {
					if selectedInstance, err := selectInstanceByHost(profileInfo, host); err == nil {
						instance = selectedInstance
						currentProfile = config.DefaultProfile
						found = true
					}
				}
			}

			// If not found in default profile, search all profiles
			if !found {
				if config.Profiles != nil {
					for profileName, profileInfo := range config.Profiles {
						// Skip default profile if we already checked it
						if profileName == config.DefaultProfile {
							continue
						}

						if selectedInstance, err := selectInstanceByHost(profileInfo, host); err == nil {
							instance = selectedInstance

							// Ensure Profile field is set
							if instance.Profile == "" {
								instance.Profile = profileName
							}

							currentProfile = profileName
							found = true
							break
						}
					}
				}

				if !found {
					return fmt.Errorf("instance with host '%s' not found in any profile", host)
				}
			} else {
				// Ensure Profile field is set when found in default profile
				if instance.Profile == "" {
					instance.Profile = currentProfile
				}
			}
		}
	} else if instanceName != "" {
		// Handle instance name lookup
		// If profile is specified, look only in that profile
		if *profile != "" || *profileShort != "" {
			currentProfile, err = ensureProfile(config, profile, profileShort)
			if err != nil {
				return err
			}

			profileInfo := config.Profiles[currentProfile]

			if profileInfo.Instances == nil {
				profileInfo.Instances = make(map[string]Instance)
			}

			selectedInstance, err := selectInstanceByName(profileInfo, instanceName)
			if err != nil {
				return fmt.Errorf("instance '%s' not found in profile '%s'", instanceName, currentProfile)
			}

			instance = selectedInstance
		} else {
			// No profile specified - first check default profile, then search all profiles
			if config.DefaultProfile != "" {
				// Try default profile first
				if profileInfo, exists := config.Profiles[config.DefaultProfile]; exists {
					if selectedInstance, err := selectInstanceByName(profileInfo, instanceName); err == nil {
						instance = selectedInstance
						currentProfile = config.DefaultProfile
					}
				}
			}

			// If not found in default profile, search all profiles (skip default if already checked)
			if instance.ID == "" {
				found := false

				if config.Profiles != nil {
					for profileName, profileInfo := range config.Profiles {
						// Skip default profile if we already checked it
						if profileName == config.DefaultProfile {
							continue
						}

						if selectedInstance, err := selectInstanceByName(profileInfo, instanceName); err == nil {
							instance = selectedInstance

							// Ensure Profile field is set
							if instance.Profile == "" {
								instance.Profile = profileName
							}

							currentProfile = profileName
							found = true
							break
						}
					}
				}

				if !found {
					return fmt.Errorf("instance '%s' not found in any profile", instanceName)
				}
			} else {
				// Ensure Profile field is set when found in default profile
				if instance.Profile == "" {
					instance.Profile = currentProfile
				}
			}
		}
	} else {
		// No name or host specified - use default instance
		currentProfile, err = ensureProfile(config, profile, profileShort)
		if err != nil {
			return err
		}

		profileInfo := config.Profiles[currentProfile]

		if profileInfo.Instances == nil {
			profileInfo.Instances = make(map[string]Instance)
		}

		// Try to get default instance from saved configuration
		selectedInstance, err := selectInstanceByName(profileInfo, "")
		if err == nil {
			instance = selectedInstance
		} else {
			return fmt.Errorf("no default instance configured for profile '%s'", currentProfile)
		}
	}

	// Verify we have an instance ID
	if instance.ID == "" {
		return fmt.Errorf("instance ID must be specified")
	}

	commandArgs := []string{
		"ssm",
		"start-session",
		"--target",
		instance.ID,
	}

	if len(currentProfile) != 0 {
		commandArgs = append(commandArgs, "--profile", currentProfile)
	}

	// Let the user know which instance we're connecting to
	msg := fmt.Sprintf("Connecting to instance %s%s%s (ID: %s) using profile %s%s%s", greenColor, instance.Name, resetColor, instance.ID, greenColor, currentProfile, resetColor)
	fmt.Println("\n-----------------------------------------------------------------------------------------------")
	fmt.Println(msg)
	fmt.Println("-----------------------------------------------------------------------------------------------")

	// Ensure that we're logged in before running the command.
	if !isLoggedIn(currentProfile) {
		loginArgs := []string{}

		if len(currentProfile) != 0 {
			loginArgs = append(loginArgs, "--profile", currentProfile)
		}

		login(loginArgs, config)
	}

	// Let's set up to prevent Ctrl-C from killing the program. Instead, it must
	// be handled with the SSM session.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	defer func() {
		signal.Stop(signalChan)
	}()

	select {
	case <-signalChan:
	default:
	}

	fmt.Println("\nStarting SSM session...")

	command := exec.Command("aws", commandArgs...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin

	if err = command.Run(); err != nil {
		return err
	}

	return nil
}
