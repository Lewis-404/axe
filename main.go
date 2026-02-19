package main

import (
	"os"

	"github.com/Lewis-404/axe/cmd"
)

var version = "dev"

func main() {
	cmd.Version = version
	cmd.Run(os.Args[1:])
}
