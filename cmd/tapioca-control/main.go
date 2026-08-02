package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/carlos/tapioca/internal/control"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()

	server := control.NewServer(os.Stdin, os.Stdout, nil)
	if err := server.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "tapioca-control:", err)
		os.Exit(1)
	}
}
