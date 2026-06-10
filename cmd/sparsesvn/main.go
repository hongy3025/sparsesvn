package main

import (
	"os"

	"github.com/sparsesvn/sparsesvn/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
