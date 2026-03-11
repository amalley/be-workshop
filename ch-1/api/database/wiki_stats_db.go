package database

import "sync"

type WikiStats map[string]uint64

func (ws WikiStats) Add(stat string) uint64 {
	ws[stat]++
	return ws[stat]
}

func (ws WikiStats) Count() int {
	return len(ws)
}

func (ws WikiStats) Total(id string) uint64 {
	if total, exists := ws[id]; exists {
		return total
	}
	return 0
}

type WikiStatsDB struct {
	mu sync.Mutex

	messages WikiStats
	users    WikiStats
	bots     WikiStats
	servers  WikiStats
}

func NewWikiStatsDB() *WikiStatsDB {
	return &WikiStatsDB{
		messages: make(WikiStats),
		users:    make(WikiStats),
		bots:     make(WikiStats),
		servers:  make(WikiStats),
	}
}

func (db *WikiStatsDB) Insert(id, user, server string, isBot bool) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.messages.Add(id)

	if isBot {
		db.bots.Add(user)
	} else {
		db.users.Add(user)
	}

	db.servers.Add(server)
}

func (db *WikiStatsDB) GetCounts() (int, int, int, int) {
	return db.messages.Count(), db.users.Count(), db.bots.Count(), db.servers.Count()
}

func (db *WikiStatsDB) GetMessageTotal(id string) uint64 {
	return db.messages.Total(id)
}

func (db *WikiStatsDB) GetUsersTotal(id string) uint64 {
	return db.messages.Total(id)
}

func (db *WikiStatsDB) GetBotsTotal(id string) uint64 {
	return db.bots.Total(id)
}

func (db *WikiStatsDB) GetServersTotal(id string) uint64 {
	return db.servers.Total(id)
}
