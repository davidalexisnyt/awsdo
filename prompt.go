package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// errCanceled reports that the user abandoned a command at one of its prompts.
// Commands return it unchanged so the REPL can show its prompt again instead of
// printing a failure.
var errCanceled = errors.New("canceled")

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
func wasCanceled(err error) bool {
	return errors.Is(err, errCanceled)
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// waitForEnter shows a message and waits for Enter, returning errCanceled if the
// user presses Ctrl-C instead.
func waitForEnter(message string) error {
	_, err := askForLine(message)

	return err
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// reportCommandResult prints the outcome of a command, keeping a cancellation
// distinct from a failure.
func reportCommandResult(err error) {
	if err == nil {
		return
	}

	if wasCanceled(err) {
		fmt.Println("Canceled.")
		return
	}

	fmt.Println(err.Error())
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// askForLine prints a question, reads one line of input, and returns it trimmed.
// It returns errCanceled if the user presses Ctrl-C or Ctrl-D instead of
// answering.
//
// The line is read in raw mode so that Ctrl-C arrives as a keystroke. The REPL
// puts the terminal back into cooked mode before running a command, and in
// cooked mode Ctrl-C raises SIGINT, which has no handler during a prompt and so
// kills the whole process instead of just the command being run.
func askForLine(question string) (string, error) {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		// Piped input and scripts have no keystrokes to interpret.
		fmt.Print(question)

		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return "", errCanceled
		}

		return strings.TrimSpace(line), nil
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, oldState)

	// Raw mode sends line feeds without a carriage return, so any newline in
	// the question needs the carriage return added back.
	fmt.Print(strings.ReplaceAll(question, "\n", "\r\n"))

	// Redrawing reprints from column 1, so it must reprint only the part of the
	// question that shares the cursor's line.
	promptText := question
	if index := strings.LastIndexAny(promptText, "\r\n"); index >= 0 {
		promptText = promptText[index+1:]
	}

	answer, err := readLineWithEditing(bufio.NewReader(os.Stdin), newQuestionEditor(promptText))

	fmt.Print("\r\n")

	if err != nil {
		// Ctrl-C arrives as errCanceled and Ctrl-D as io.EOF. Both mean the user
		// is finished with this command.
		if wasCanceled(err) || errors.Is(err, io.EOF) {
			return "", errCanceled
		}

		return "", err
	}

	return strings.TrimSpace(answer), nil
}
