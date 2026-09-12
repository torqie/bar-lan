package lan

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLoopbackDiscovery(t *testing.T) {
	c, err := Listen("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	s := Session{Version: 1, Port: 8452, Host: "Alice", Guest: "Bob", Game: "BAR", Map: "Map.smf", EngineSHA256: strings.Repeat("a", 64)}
	go func() { done <- Serve(ctx, c, s) }()
	found, err := Discover(ctx, "127.0.0.1", 500*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].IP != "127.0.0.1" || found[0].Guest != "Bob" {
		t.Fatalf("unexpected %#v", found)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestRejectAdvertisement(t *testing.T) {
	s := Session{Version: 99}
	if s.Validate() == nil {
		t.Fatal("bad version")
	}
}
