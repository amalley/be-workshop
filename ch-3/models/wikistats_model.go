package models

// WikiStatsModel represents the stream data we care about
type WikiStatsModel struct {
	ID     string
	User   string
	Server string
	IsBot  bool
}
