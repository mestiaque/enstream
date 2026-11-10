// ENStream: a private video streaming server for your local network.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"enstream/internal/app"
)

func main() {
	var o app.Options
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.StringVar(&o.DataDir, "data", "", "data directory (default: per-user data folder, or ENSTREAM_DATA_DIR)")
	flag.StringVar(&o.Host, "host", "", "address to listen on (default from config.json: 0.0.0.0)")
	flag.IntVar(&o.Port, "port", 0, "port to listen on (default from config.json: 8080)")
	flag.BoolVar(&o.Debug, "debug", false, "verbose logging")
	flag.Parse()
	if *showVersion {
		fmt.Println("ENStream", app.Version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, o); err != nil {
		fmt.Fprintln(os.Stderr, "ENStream could not start:", err)
		os.Exit(1)
	}
}
