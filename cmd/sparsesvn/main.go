package main

import (
	"os"

	"github.com/hongy3025/sparsesvn/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
