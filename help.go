package main

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

//go:embed help/general.txt
var helpGeneral string

//go:embed help/init.txt
var helpInit string

//go:embed help/login.txt
var helpLogin string

//go:embed help/get-credentials.txt
var helpGetCredentials string

//go:embed help/instances.txt
var helpInstances string

//go:embed help/terminal.txt
var helpTerminal string

//go:embed help/bastion.txt
var helpBastion string

//go:embed help/bastions.txt
var helpBastions string

//go:embed help/help.txt
var helpHelp string

//go:embed help/docs.txt
var helpDocs string

//go:embed help/repl.txt
var helpRepl string

//go:embed help/rm.txt
var helpRm string

//go:embed help/ls.txt
var helpLs string

//go:embed help/unknown.txt
var helpUnknown string

//go:embed docs/index.html
var docsHTML string

//go:embed docs/styles.css
var docsCSS string

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func showDocs() {
	fmt.Println()

	// Create HTTP handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/styles.css" {
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, docsCSS)
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, docsHTML)
		}
	})

	// Start server on localhost
	port, err := findAvailableLocalPort(8080)
	if err != nil {
		fmt.Printf("Error finding available local port: %v\n", err)
		os.Exit(1)
	}

	url := fmt.Sprintf("http://localhost:%d", port)

	fmt.Printf("Starting documentation server on http://localhost:%d...\n", port)
	fmt.Println("Press Ctrl+C to stop the documentation server.")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	setupSignalHandler(sigChan)

	// Open browser
	go openBrowser(url)

	// Start HTTP server in a goroutine
	server := &http.Server{Addr: fmt.Sprintf(":%d", port)}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Error starting documentation server: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\nShutting down documentation server...")

	// Gracefully shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Printf("Documentation server shutdown error: %v\n", err)
	}
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux and others
		cmd = exec.Command("xdg-open", url)
	}

	cmd.Run()
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func showHelp(command string) {
	fmt.Println()

	if command == "" {
		// General help - list all commands
		fmt.Print(helpGeneral)
		return
	}

	// Command-specific help
	switch command {
	case "init":
		fmt.Print(helpInit)
	case "login":
		fmt.Print(helpLogin)
	case "get-credentials", "credentials":
		fmt.Print(helpGetCredentials)
	case "instances":
		fmt.Print(helpInstances)
	case "instances find":
		fmt.Print(helpInstances)
	case "terminal":
		fmt.Print(helpTerminal)
	case "bastion":
		fmt.Print(helpBastion)
	case "bastions":
		fmt.Print(helpBastions)
	case "bastions list":
		fmt.Print(helpBastions)
	case "bastions add":
		fmt.Print(helpBastions)
	case "docs":
		fmt.Print(helpDocs)
	case "repl":
		fmt.Print(helpRepl)
	case "help":
		fmt.Print(helpHelp)
	case "rm", "remove":
		fmt.Print(helpRm)
	case "ls", "list":
		fmt.Print(helpLs)
	default:
		fmt.Printf(helpUnknown, command)
	}
}
