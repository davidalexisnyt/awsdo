package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func validateFilter(f string) error {
	if len(f) > 128 {
		return fmt.Errorf("filter too long")
	}

	if f == "" {
		return nil
	}

	for _, r := range f {
		if !(r == '*' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return fmt.Errorf("filter contains invalid character")
		}
	}

	return nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func findAvailableLocalPort(startPort int) (int, error) {
	for port := startPort; port < startPort+1000; port++ {
		// Bind only to loopback to avoid exposing the port network-wide
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			// Port is available - close the listener immediately
			listener.Close()

			// Small delay to ensure port is fully released (especially on Windows)
			time.Sleep(10 * time.Millisecond)

			return port, nil
		}
	}

	return 0, fmt.Errorf("could not find available port starting from %d", startPort)
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func generateBastionID() (string, error) {
	bytes := make([]byte, 8)

	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// confirmPrompt displays a yes/no question and returns true if the user answers yes.
// When stdin is a terminal, accepts a single keypress (y/n) without requiring Enter.
// Falls back to line input when stdin is not a terminal (pipes, scripts).
func confirmPrompt(question string) bool {
	fmt.Printf("%s [y/n]: ", question)

	fd := int(os.Stdin.Fd())

	if term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err == nil {
			defer term.Restore(fd, oldState)
		}

		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				fmt.Println()
				return false
			}

			key := buf[0]

			switch {
			case key == 'y' || key == 'Y':
				fmt.Println("y")
				return true
			case key == 'n' || key == 'N' || key == 13 || key == '\r' || key == '\n':
				fmt.Println("n")
				return false
			case key == 3 || key == 27: // Ctrl-C or Escape
				fmt.Println("n")
				return false
			}

			// Ignore any other key and wait for a valid response
		}
	}

	// Non-terminal fallback (piped input, scripts)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))

	return line == "y" || line == "yes"
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// setupSignalHandler sets up signal handling for Ctrl+C that works on both Windows and Unix systems.
// On Windows, it uses console control handlers to catch Ctrl+C events.
// On Unix systems, it uses standard signal handling.
func setupSignalHandler(sigChan chan os.Signal) {
	if runtime.GOOS == "windows" {
		setupSignalHandlerWindows(sigChan)
	} else {
		// On Unix systems, standard signal handling works fine
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	}
}
