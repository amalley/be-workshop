package models

// WikiStatsModel represents the stream data we care about
type WikiStatsModel struct {
	Message string
	User    string
	Server  string
	IsBot   bool
}

// WikiStatCount represents the count of each state we care about
type WikiStatsCounts struct {
	Messages int
	Users    int
	Servers  int
	Bots     int
}
