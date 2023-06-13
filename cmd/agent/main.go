package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/qbee-io/qbee-agent/app/cmd"
)

var defaultUmask int = 0077

func init() {
	// set global umask
	syscall.Umask(defaultUmask)
}

func main() {
	if err := cmd.Main.Execute(os.Args[1:], nil); err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
}
