package main

import (
	"fmt"
	"os"

	"github.com/qbee-io/qbee-agent/app/cmd"
)

func main() {
	if err := cmd.Main.Execute(os.Args[1:], nil); err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
}
