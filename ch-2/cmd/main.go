package main

import (
	"log/slog"
	"os"

	"github.com/AMalley/be-workshop/ch-2/api/adapter"
	"github.com/AMalley/be-workshop/ch-2/api/controller"
	"github.com/AMalley/be-workshop/ch-2/api/database"
	"github.com/AMalley/be-workshop/ch-2/api/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	db := database.NewWikiStatsDB()

	// Initialize the server and its components
	adp := adapter.NewWikiStreamAdapter(logger, db)
	ctl := controller.NewController(logger, db)
	svr := server.NewServer(logger, ctl, adp, 7000)

	// Start the server
	svr.Start()
}
