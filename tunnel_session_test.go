package main

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func TestParseKeepAliveSetting(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "seconds", value: "30", want: 30},
		{name: "padded seconds", value: " 45 ", want: 45},
		{name: "minimum allowed", value: "5", want: 5},
		{name: "off", value: "off", want: keepAliveOff},
		{name: "off uppercase", value: "OFF", want: keepAliveOff},
		{name: "zero disables", value: "0", want: keepAliveOff},
		{name: "on restores default", value: "on", want: defaultKeepAliveSeconds},
		{name: "below minimum", value: "3", wantErr: true},
		{name: "negative", value: "-10", wantErr: true},
		{name: "not a number", value: "sometimes", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseKeepAliveSetting(test.value)

			if test.wantErr {
				if err == nil {
					t.Fatalf("parseKeepAliveSetting(%q) = %d, want error", test.value, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseKeepAliveSetting(%q) returned error: %v", test.value, err)
			}

			if got != test.want {
				t.Errorf("parseKeepAliveSetting(%q) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestKeepAliveInterval(t *testing.T) {
	tests := []struct {
		name       string
		configured int
		want       time.Duration
	}{
		{name: "unset uses default", configured: 0, want: defaultKeepAliveSeconds * time.Second},
		{name: "explicit seconds", configured: 30, want: 30 * time.Second},
		{name: "disabled", configured: keepAliveOff, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := keepAliveInterval(test.configured); got != test.want {
				t.Errorf("keepAliveInterval(%d) = %s, want %s", test.configured, got, test.want)
			}
		})
	}
}

func TestParseReconnectSetting(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		wantNoRecon bool
		wantErr     bool
	}{
		{name: "on", value: "on", wantNoRecon: false},
		{name: "true", value: "true", wantNoRecon: false},
		{name: "off", value: "off", wantNoRecon: true},
		{name: "no", value: "No", wantNoRecon: true},
		{name: "garbage", value: "maybe", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseReconnectSetting(test.value)

			if test.wantErr {
				if err == nil {
					t.Fatalf("parseReconnectSetting(%q) = %v, want error", test.value, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseReconnectSetting(%q) returned error: %v", test.value, err)
			}

			if got != test.wantNoRecon {
				t.Errorf("parseReconnectSetting(%q) = %v, want %v", test.value, got, test.wantNoRecon)
			}
		})
	}
}

func TestReconnectDelayGrowsAndCaps(t *testing.T) {
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{failures: 0, want: time.Second},
		{failures: 1, want: 2 * time.Second},
		{failures: 3, want: 8 * time.Second},
		{failures: 5, want: maxReconnectDelay},
		{failures: 64, want: maxReconnectDelay},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("failures=%d", test.failures), func(t *testing.T) {
			if got := reconnectDelay(test.failures); got != test.want {
				t.Errorf("reconnectDelay(%d) = %s, want %s", test.failures, got, test.want)
			}
		})
	}
}

func TestPartitionBastionArgsKeepsFlagsAfterName(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantFlags       []string
		wantPositionals []string
	}{
		{
			name:            "flags after positional",
			args:            []string{"my-db", "--keepalive", "30", "-p", "dev"},
			wantFlags:       []string{"--keepalive", "30", "-p", "dev"},
			wantPositionals: []string{"my-db"},
		},
		{
			name:            "flags before positional",
			args:            []string{"--reconnect", "off", "my-db"},
			wantFlags:       []string{"--reconnect", "off"},
			wantPositionals: []string{"my-db"},
		},
		{
			name:            "equals form",
			args:            []string{"--keepalive=45", "my-db"},
			wantFlags:       []string{"--keepalive=45"},
			wantPositionals: []string{"my-db"},
		},
		{
			name:      "no positionals",
			args:      []string{"--profile", "dev"},
			wantFlags: []string{"--profile", "dev"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags, positionals := partitionBastionArgs(test.args)

			if !equalStrings(flags, test.wantFlags) {
				t.Errorf("flags = %v, want %v", flags, test.wantFlags)
			}

			if !equalStrings(positionals, test.wantPositionals) {
				t.Errorf("positionals = %v, want %v", positionals, test.wantPositionals)
			}
		})
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

func TestPingLocalPortUntilStoppedConnectsAndStops(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not open test listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	accepted := make(chan struct{}, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			conn.Close()

			select {
			case accepted <- struct{}{}:
			default:
			}
		}
	}()

	stop := make(chan struct{})
	finished := make(chan struct{})

	go func() {
		pingLocalPortUntilStopped(port, 10*time.Millisecond, stop)
		close(finished)
	}()

	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		close(stop)
		t.Fatal("keepalive pinger never connected to the local port")
	}

	close(stop)

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("keepalive pinger did not exit after stop was closed")
	}
}

func TestWaitBeforeReconnectReportsInterrupt(t *testing.T) {
	signalChan := make(chan os.Signal, 1)
	signalChan <- os.Interrupt

	if !waitBeforeReconnect(time.Minute, signalChan) {
		t.Error("waitBeforeReconnect() = false with a pending signal, want true")
	}

	if waitBeforeReconnect(10*time.Millisecond, make(chan os.Signal)) {
		t.Error("waitBeforeReconnect() = true on timeout, want false")
	}
}

func TestWaitForLocalPortReleaseReturnsWhenFree(t *testing.T) {
	freePort, err := findAvailableLocalPort(7100)
	if err != nil {
		t.Fatalf("could not find a free port: %v", err)
	}

	started := time.Now()
	waitForLocalPortRelease(freePort, 2*time.Second)

	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("waitForLocalPortRelease() on a free port took %s, want a prompt return", elapsed)
	}
}

func TestWaitForLocalPortReleaseWaitsForBoundPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not open test listener: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	started := time.Now()
	waitForLocalPortRelease(port, 600*time.Millisecond)

	if elapsed := time.Since(started); elapsed < 500*time.Millisecond {
		t.Errorf("waitForLocalPortRelease() on a bound port returned after %s, want it to wait for the timeout", elapsed)
	}
}
