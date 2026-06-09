package main

import (
	"os"

	"github.com/sparsesvn/sparsesvn/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
