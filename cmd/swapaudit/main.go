//go:build linux

package main

import (
	"os"

	"github.com/jadmadi/swapaudit/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:]))
}
