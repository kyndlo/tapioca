package main

import (
	"fmt"
	"os"

	"github.com/carlos/tapioca/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tapioca:", err)
		os.Exit(1)
	}
}
