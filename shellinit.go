package main

import (
	"embed"
	"fmt"
)

//go:embed shellinit
var shellInitScripts embed.FS

var shellInitFiles = map[string]string{
	"zsh":        "shellinit/zsh.zsh",
	"bash":       "shellinit/bash.sh",
	"fish":       "shellinit/fish.fish",
	"powershell": "shellinit/powershell.ps1",
	"pwsh":       "shellinit/powershell.ps1",
}

// - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
// printShellInit writes the key bindings that let a shell step through awsdo
// history after the user types 'awsdo '.
func printShellInit(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: awsdo shell-init <zsh|bash|fish|powershell>")
	}

	file, ok := shellInitFiles[args[0]]
	if !ok {
		return fmt.Errorf("unsupported shell %q; use zsh, bash, fish, or powershell", args[0])
	}

	script, err := shellInitScripts.ReadFile(file)
	if err != nil {
		return err
	}

	fmt.Print(string(script))
	return nil
}
