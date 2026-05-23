package elo

import "testing"

func TestRatings_WinsMoveRating(t *testing.T) {
	r := New()
	r.Match("a", "b", OutcomeA)
	if r.Get("a") <= 1500 || r.Get("b") >= 1500 {
		t.Fatalf("expected a>1500,b<1500 — got a=%f b=%f", r.Get("a"), r.Get("b"))
	}
}

func TestLeaderboard_SortsByRating(t *testing.T) {
	r := New()
	r["x"] = 1700
	r["y"] = 1500
	r["z"] = 1600
	board := r.Leaderboard()
	if board[0] != "x" || board[1] != "z" || board[2] != "y" {
		t.Fatalf("unexpected leaderboard %v", board)
	}
}
