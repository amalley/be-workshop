package database

import (
	"strconv"
	"testing"
)

func TestWikiStatsDBInsert(t *testing.T) {
	db := NewWikiStatsDB()

	db.Insert("MockID1", "User1", "Server1", false)
	msg, user, bots, servers := db.GetCounts()

	if msg != 1 {
		t.Errorf("Expected 1 message, got %s", strconv.Itoa(msg))
	}
	if user != 1 {
		t.Errorf("Expected 1 user, got %s", strconv.Itoa(user))
	}
	if bots != 0 {
		t.Errorf("Expected 0 bots, got %s", strconv.Itoa(bots))
	}
	if servers != 1 {
		t.Errorf("Expected 1 server, got %s", strconv.Itoa(servers))
	}
}
