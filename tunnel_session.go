package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Session Manager ends a session after an account-wide idle timeout (20 minutes
// by default, 60 at most) and idle WebSocket connections are dropped even sooner
// by many corporate networks and NAT gateways. Neither limit can be raised from
// the command line, so a long-lived tunnel has to be kept busy and restarted
// when it dies anyway.
const (
	defaultKeepAliveSeconds = 60
	minKeepAliveSeconds     = 5
	keepAliveOff            = -1

	keepAliveDialTimeout = 5 * time.Second

	// A session that stayed up this long counts as healthy, so a later drop
	// restarts backoff instead of counting toward the failure limit.
	stableSessionDuration = 60 * time.Second

	maxReconnectDelay  = 30 * time.Second
	maxRapidFailures   = 5
	portReleaseTimeout = 10 * time.Second
)

type tunnelOutcome int

const (
	tunnelStoppedByUser tunnelOutcome = iota
	tunnelTargetNotConnected
	tunnelSessionEnded
)

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// keepAliveInterval resolves a stored per-bastion setting to a ping interval.
// A zero interval means keepalive pings are disabled.
func keepAliveInterval(configuredSeconds int) time.Duration {
	if configuredSeconds < 0 {
		return 0
	}

	if configuredSeconds == 0 {
		return defaultKeepAliveSeconds * time.Second
	}

	return time.Duration(configuredSeconds) * time.Second
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// parseKeepAliveSetting converts a command line value ("45", "off") into the
// form stored in the configuration file.
func parseKeepAliveSetting(value string) (int, error) {
	trimmed := strings.TrimSpace(value)

	switch strings.ToLower(trimmed) {
	case "off", "no", "none", "false", "0":
		return keepAliveOff, nil
	case "on", "yes", "true", "default":
		return defaultKeepAliveSeconds, nil
	}

	seconds, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid keepalive value '%s': use a number of seconds or 'off'", value)
	}

	if seconds < minKeepAliveSeconds {
		return 0, fmt.Errorf("keepalive interval must be at least %d seconds, or 'off'", minKeepAliveSeconds)
	}

	return seconds, nil
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// parseReconnectSetting converts a command line value into the stored
// NoReconnect flag.
func parseReconnectSetting(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "yes", "true", "1":
		return false, nil
	case "off", "no", "false", "0":
		return true, nil
	}

	return false, fmt.Errorf("invalid reconnect value '%s': use 'on' or 'off'", value)
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func describeKeepAlive(interval time.Duration) string {
	if interval <= 0 {
		return "off"
	}

	return fmt.Sprintf("every %s", interval)
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// pingLocalPortUntilStopped opens and immediately closes a connection to the
// forwarded local port on every tick. Connection setup pushes enough traffic
// through the session to keep Session Manager from treating it as idle. The
// remote service may log an aborted connection for each ping.
func pingLocalPortUntilStopped(localPort int, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	address := fmt.Sprintf("127.0.0.1:%d", localPort)

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", address, keepAliveDialTimeout)

			// A failed ping means the tunnel is still starting or already gone.
			// Noticing that is the session runner's job, not the pinger's.
			if err == nil {
				conn.Close()
			}
		}
	}
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// runTunnelSession runs one `aws ssm start-session` process to completion and
// reports why it ended, along with how long it lasted.
func runTunnelSession(commandArgs []string, localPort int, keepAlive time.Duration, signalChan <-chan os.Signal) (tunnelOutcome, time.Duration, error) {
	var stderrBuf bytes.Buffer

	command := exec.Command("aws", commandArgs...)
	command.Stdout = os.Stdout
	command.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	command.Stdin = os.Stdin

	startedAt := time.Now()

	if err := command.Start(); err != nil {
		return tunnelSessionEnded, 0, fmt.Errorf("failed to start session: %v", err)
	}

	if keepAlive > 0 {
		stopPinger := make(chan struct{})
		defer close(stopPinger)

		go pingLocalPortUntilStopped(localPort, keepAlive, stopPinger)
	}

	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()

	select {
	case <-signalChan:
		fmt.Println("\nStopping bastion tunnel...")

		if err := command.Process.Kill(); err != nil {
			return tunnelStoppedByUser, time.Since(startedAt), fmt.Errorf("failed to kill process: %v", err)
		}

		// Wait for the process to actually terminate
		<-done

		return tunnelStoppedByUser, time.Since(startedAt), nil

	case err := <-done:
		elapsed := time.Since(startedAt)

		if err == nil {
			return tunnelSessionEnded, elapsed, nil
		}

		// Ctrl-C reaches the AWS CLI directly through the shared process group,
		// so a signal death is a user stop rather than a failure.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == -1 {
			return tunnelStoppedByUser, elapsed, nil
		}

		if strings.Contains(stderrBuf.String(), "TargetNotConnected") {
			return tunnelTargetNotConnected, elapsed, nil
		}

		return tunnelSessionEnded, elapsed, fmt.Errorf("session ended with error: %v", err)
	}
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func reconnectDelay(consecutiveFailures int) time.Duration {
	delay := time.Duration(1<<consecutiveFailures) * time.Second

	if delay <= 0 || delay > maxReconnectDelay {
		return maxReconnectDelay
	}

	return delay
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// waitBeforeReconnect reports whether the wait was cut short by Ctrl-C.
func waitBeforeReconnect(delay time.Duration, signalChan <-chan os.Signal) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-signalChan:
		return true
	case <-timer.C:
		return false
	}
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// waitForLocalPortRelease gives the previous session's plugin process time to
// release the forwarded port, which the replacement session needs to bind. The
// probe binds localhost specifically because that is where the Session Manager
// plugin listens; binding all interfaces succeeds even while localhost is held.
func waitForLocalPortRelease(port int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))

		if err == nil {
			listener.Close()
			return
		}

		time.Sleep(250 * time.Millisecond)
	}
}
