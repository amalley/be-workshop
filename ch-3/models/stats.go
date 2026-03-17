package models

type GetStatsResponse struct {
	Messages int `json:"messages"`
	Users    int `json:"users"`
	Servers  int `json:"servers"`
	Bots     int `json:"bots"`
}
