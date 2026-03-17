package main

import (
	"log/slog"
	"os"

	"github.com/amalley/be-workshop/ch-1/api/adapter"
	"github.com/amalley/be-workshop/ch-1/api/controller"
	"github.com/amalley/be-workshop/ch-1/api/database"
	"github.com/amalley/be-workshop/ch-1/api/server"
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
