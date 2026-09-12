package recoil

import (
	"strings"
	"testing"
)

func TestHostAndClient(t *testing.T) {
	h, err := Host(8452, "Alice", "Bob", "Beyond All Reason test", "Map v1.smf")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"IsHost=1;", "Name=Bob;", "TeamLeader=1;", "NumPlayers=2;", "Side=Armada;"} {
		if !strings.Contains(h, s) {
			t.Errorf("missing %s", s)
		}
	}
	c, err := Client("192.168.1.2", 8452, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c, "MyPlayerName=Bob;") || strings.Contains(c, "[PLAYER") {
		t.Fatal(c)
	}
}
func TestRejectUnsafe(t *testing.T) {
	for _, s := range []string{"", "Bob;IsHost=1", "a\nb", "x}", "//hi", "a/*b", "a=b"} {
		if Value(s) == nil {
			t.Errorf("accepted %q", s)
		}
	}
	if _, e := Host(8452, "A", "a", "game", "map"); e == nil {
		t.Fatal("duplicate names")
	}
	if _, e := Client("hostname", 8452, "Bob"); e == nil {
		t.Fatal("hostname accepted")
	}
}
