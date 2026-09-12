package main

import (
	"fmt"
	"github.com/torqie/bar-lan/internal/install"
	"github.com/torqie/bar-lan/internal/recoil"
	"sort"
	"strings"
)

func validatePlayer(name string) error {
	if err := recoil.Value(name); err != nil {
		return fmt.Errorf("enter your player name (without punctuation such as ; or brackets)")
	}
	return nil
}
func sortGames(g []string) {
	sort.SliceStable(g, func(i, j int) bool {
		a, b := strings.Contains(strings.ToLower(g[i]), "beyond all reason"), strings.Contains(strings.ToLower(g[j]), "beyond all reason")
		if a != b {
			return a
		}
		return install.Newer(g[i], g[j])
	})
}
