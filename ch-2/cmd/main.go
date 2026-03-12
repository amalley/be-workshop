package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/AMalley/be-workshop/ch-2/api/adapter"
	"github.com/AMalley/be-workshop/ch-2/api/controller"
	"github.com/AMalley/be-workshop/ch-2/api/database"
	"github.com/AMalley/be-workshop/ch-2/api/server"
)

type Args struct {
	Port int
	URL  string
}

func parseArgs() *Args {
	args := &Args{
		Port: 7000,
		URL:  "https://stream.wikimedia.org/v2/stream/recentchange",
	}

	flag.IntVar(&args.Port, "port", args.Port, "Port to listen on")
	flag.StringVar(&args.URL, "url", args.URL, "WikiMedia stream url")
	flag.Parse()

	return args
}

func main() {
	args := parseArgs()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	db := database.NewWikiStatsDB()

	// Initialize the server and its components
	adp := adapter.NewWikiStreamAdapter(logger, db, args.URL)
	ctl := controller.NewController(logger, db)
	svr := server.NewServer(logger, ctl, adp, args.Port)

	// Start the server
	svr.Start()
}
