package main

import "testing"

func TestGameChoices(t *testing.T) {
	g := []string{"Other RTS 100", "Beyond All Reason 9", "Beyond All Reason 10"}
	sortGames(g)
	if g[0] != "Beyond All Reason 10" {
		t.Fatal(g)
	}
	if validatePlayer("") == nil || validatePlayer("Alice") != nil {
		t.Fatal("name validation")
	}
}
