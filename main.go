package main

/*
	- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	awsdo
	Author: David Alexis

	This application wraps calls to the AWS CLI to simplify certain key tasks, like logging in,
	listing instances that match a given pattern, launching an SSM session, and starting a
	bastion host tunnel.
	- - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
*/

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var Version = "1.0.10"

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func main() {
	exePath, _ := os.Executable()
	configFile := filepath.Join(filepath.Dir(exePath), "awsdo_config.json")

	config, err := loadConfiguration(configFile)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Default to launching the REPL if no command is provided
	if len(os.Args) < 2 {
		startREPL(configFile, &config)
		return
	}

	command := strings.ToLower(os.Args[1])

	switch command {
	case "help":
		if len(os.Args) > 2 {
			showHelp(strings.ToLower(os.Args[2]))
		} else {
			showHelp("")
		}
		return
	case "login":
		login(os.Args[2:], &config)
	case "get-credentials", "credentials":
		if err := getCredentials(os.Args[2:], &config); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		return
	case "instances":
		if len(os.Args) < 3 {
			// Default to 'list' if no subcommand provided
			listInstances([]string{}, &config)
		} else {
			subcommand := strings.ToLower(os.Args[2])
			switch subcommand {
			case "find":
				findInstances(os.Args[3:], &config)
			case "list", "ls":
				listInstances(os.Args[3:], &config)
			case "add":
				addInstance(os.Args[3:], &config)
			case "update":
				updateInstance(os.Args[3:], &config)
			case "remove", "rm":
				removeInstance(os.Args[3:], &config)
			default:
				fmt.Printf("Invalid instances subcommand: %s\n", subcommand)
				fmt.Println("Use 'awsdo instances find' to find instances, 'awsdo instances list' to list configured instances, 'awsdo instances add' to add an instance, 'awsdo instances update' to update an instance, 'awsdo instances remove' to remove an instance, or 'awsdo help instances' for more information.")
				os.Exit(1)
			}
		}
	case "terminal":
		startSSMSession(os.Args[2:], &config)
	case "bastion":
		startBastionTunnel(os.Args[2:], &config)
	case "bastions":
		if len(os.Args) < 3 {
			// Default to 'list' if no subcommand provided
			listBastions([]string{}, &config)
		} else {
			subcommand := strings.ToLower(os.Args[2])
			switch subcommand {
			case "list", "ls":
				listBastions(os.Args[3:], &config)
			case "add":
				addBastion(os.Args[3:], &config)
			case "update", "up":
				updateBastion(os.Args[3:], &config)
			case "remove", "rm":
				removeBastion(os.Args[3:], &config)
			default:
				fmt.Printf("Invalid bastions subcommand: %s\n", subcommand)
				fmt.Println("Use 'awsdo bastions list' to list bastions, 'awsdo bastions add' to add a new bastion, 'awsdo bastions update' to update an existing bastion, or 'awsdo bastions remove' to remove a bastion.")
				os.Exit(1)
			}
		}
	case "docs":
		showDocs()
		return
	case "repl":
		startREPL(configFile, &config)
		return
	case "init":
		initCommand(&config)
	case "ls", "list":
		if len(os.Args) < 3 {
			fmt.Println("Usage: awsdo ls <instances|bastions> [options]")
			fmt.Println("   or: awsdo list <instances|bastions> [options]")
			os.Exit(1)
		}
		object := strings.ToLower(os.Args[2])
		switch object {
		case "instances", "instance":
			listInstances(os.Args[3:], &config)
		case "bastions", "bastion":
			listBastions(os.Args[3:], &config)
		default:
			fmt.Printf("Invalid object: %s\n", object)
			fmt.Println("Use 'awsdo ls instances' or 'awsdo ls bastions'")
			os.Exit(1)
		}
	case "add":
		if len(os.Args) < 3 {
			fmt.Println("Usage: awsdo add <instance|bastion> [options]")
			os.Exit(1)
		}
		object := strings.ToLower(os.Args[2])
		switch object {
		case "instance", "instances":
			addInstance(os.Args[3:], &config)
		case "bastion", "bastions":
			addBastion(os.Args[3:], &config)
		default:
			fmt.Printf("Invalid object: %s\n", object)
			fmt.Println("Use 'awsdo add instance' or 'awsdo add bastion'")
			os.Exit(1)
		}
	case "rm":
		if len(os.Args) < 3 {
			fmt.Println("Usage: awsdo rm <instance|bastion> [options]")
			os.Exit(1)
		}
		object := strings.ToLower(os.Args[2])
		switch object {
		case "instance", "instances":
			removeInstance(os.Args[3:], &config)
		case "bastion", "bastions":
			removeBastion(os.Args[3:], &config)
		default:
			fmt.Printf("Invalid object: %s\n", object)
			fmt.Println("Use 'awsdo rm instance' or 'awsdo rm bastion'")
			os.Exit(1)
		}
	case "find":
		if len(os.Args) < 3 {
			fmt.Println("Usage: awsdo find <instance> [options]")
			os.Exit(1)
		}
		object := strings.ToLower(os.Args[2])
		switch object {
		case "instance", "instances":
			findInstances(os.Args[3:], &config)
		default:
			fmt.Printf("Invalid object: %s\n", object)
			fmt.Println("Use 'awsdo find instance'")
			os.Exit(1)
		}

	case "version":
		fmt.Println("awsdo version", Version)
		return
	default:
		fmt.Printf("Invalid command: %s\n", command)
		fmt.Println("Use 'awsdo help' to see available commands.")
		os.Exit(1)
	}

	saveConfiguration(configFile, &config)
}
