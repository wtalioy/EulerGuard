//go:build !web

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"aegis/pkg/cli"
	"aegis/pkg/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.RunCLI(config.ParseOptions(), ctx); err != nil {
		log.Fatalf("aegis: %v", err)
	}
}
