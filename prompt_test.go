package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// pipeStdin replaces os.Stdin with a pipe carrying input, which also makes stdin
// a non-terminal and so exercises the piped-input path.
func pipeStdin(t *testing.T, input string) {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create pipe: %v", err)
	}

	original := os.Stdin
	os.Stdin = reader

	t.Cleanup(func() {
		os.Stdin = original
		reader.Close()
	})

	go func() {
		defer writer.Close()
		io.WriteString(writer, input)
	}()
}

// captureStdout collects everything written to os.Stdout while run executes.
func captureStdout(t *testing.T, run func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create pipe: %v", err)
	}

	original := os.Stdout
	os.Stdout = writer

	collected := make(chan string, 1)
	go func() {
		var builder strings.Builder
		io.Copy(&builder, reader)
		collected <- builder.String()
	}()

	run()

	writer.Close()
	os.Stdout = original

	output := <-collected
	reader.Close()

	return output
}

func TestAskForLineTrimsPipedAnswer(t *testing.T) {
	pipeStdin(t, "  my-db  \nleftover\n")

	got, err := askForLine("Enter bastion name: ")
	if err != nil {
		t.Fatalf("askForLine() returned error: %v", err)
	}

	if got != "my-db" {
		t.Errorf("askForLine() = %q, want %q", got, "my-db")
	}
}

func TestAskForLineCancelsWhenInputEnds(t *testing.T) {
	pipeStdin(t, "")

	got, err := askForLine("Enter bastion name: ")
	if !wasCanceled(err) {
		t.Fatalf("askForLine() = (%q, %v), want errCanceled", got, err)
	}
}

func TestReportCommandResult(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "success prints nothing", err: nil, want: ""},
		{name: "cancellation is not a failure", err: errCanceled, want: "Canceled.\n"},
		{name: "failure prints the message", err: io.ErrUnexpectedEOF, want: "unexpected EOF\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := captureStdout(t, func() {
				reportCommandResult(test.err)
			})

			if got != test.want {
				t.Errorf("reportCommandResult(%v) printed %q, want %q", test.err, got, test.want)
			}
		})
	}
}

// Cancelling a prompt has to travel out of the command as errCanceled so the
// REPL can tell it apart from a failure.
func TestCommandsReturnCancellationFromPrompts(t *testing.T) {
	newConfig := func() *Configuration {
		return &Configuration{
			DefaultProfile: "dev",
			Profiles: map[string]Profile{
				"dev": {
					Name: "dev",
					Bastions: map[string]Bastion{
						"my-db": {Name: "my-db", Profile: "dev", Instance: "i-123", Host: "db.internal", Port: 5432, LocalPort: 7000},
					},
					Instances: map[string]Instance{
						"web": {Name: "web", Profile: "dev", ID: "i-456", Host: "10.0.0.1"},
					},
				},
			},
		}
	}

	tests := []struct {
		name string
		run  func(config *Configuration) error
	}{
		{name: "bastions remove", run: func(c *Configuration) error { return removeBastion(nil, c) }},
		{name: "bastions rename", run: func(c *Configuration) error { return renameBastion(nil, c) }},
		{name: "bastions set", run: func(c *Configuration) error { return setBastion([]string{"--port", "5433"}, c) }},
		{name: "instances remove", run: func(c *Configuration) error { return removeInstance(nil, c) }},
		{name: "instances rename", run: func(c *Configuration) error { return renameInstance(nil, c) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pipeStdin(t, "")

			var err error
			captureStdout(t, func() {
				err = test.run(newConfig())
			})

			if !wasCanceled(err) {
				t.Errorf("%s returned %v, want errCanceled", test.name, err)
			}
		})
	}
}
