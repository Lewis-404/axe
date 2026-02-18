package main

import (
	"os"

	"github.com/Lewis-404/axe/cmd"
)

func main() {
	cmd.Run(os.Args[1:])
}
