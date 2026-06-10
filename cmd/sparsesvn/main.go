package main

import (
	"os"

	"github.com/hongy3025/sparsesvn/internal/cli"
)

var version = "0.1.0-dev"

func main() {
	os.Exit(cli.Execute(version))
}
