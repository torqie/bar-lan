package room

import (
	"context"
	"github.com/torqie/bar-lan/internal/lan"
	"net"
	"strings"
	"testing"
	"time"
)

func fixture() *Server {
	return &Server{session: lan.Session{Version: 2, RoomID: ID(), Phase: "waiting", Port: 8452, Host: "Alice", Game: "BAR", Map: "Map", EngineSHA256: strings.Repeat("a", 64), GameChecksum: 123, MapChecksum: 456}}
}
func join(s *Server) Request {
	return Request{Version: 2, Action: "join", RoomID: s.session.RoomID, ClientID: ID(), Name: "Bob", EngineSHA256: s.session.EngineSHA256, GameChecksum: 123, MapChecksum: 456}
}
func TestNamesCompatibilityAndStartGate(t *testing.T) {
	s := fixture()
	if _, err := s.Begin(); err == nil {
		t.Fatal("started without guest")
	}
	r := join(s)
	for _, bad := range []Request{func() Request { b := r; b.Name = "alice"; return b }(), func() Request { b := r; b.EngineSHA256 = "wrong"; return b }(), func() Request { b := r; b.MapChecksum = 999; return b }(), func() Request { b := r; b.GameChecksum = 999; return b }()} {
		if s.handle(bad, "127.0.0.1").Error == "" {
			t.Fatal("bad guest accepted")
		}
	}
	reply := s.handle(r, "127.0.0.1")
	if reply.Error != "" || reply.Session.Guest != "Bob" || reply.Token == "" {
		t.Fatal(reply)
	}
	if repeated := s.handle(r, "127.0.0.1"); repeated.Token != reply.Token {
		t.Fatal("retry created a new slot")
	}
	other := join(s)
	other.Name = "Carol"
	if s.handle(other, "127.0.0.2").Error == "" {
		t.Fatal("second guest accepted")
	}
	snap, err := s.Begin()
	if err != nil || snap.Guest != "Bob" || snap.Phase != "starting" {
		t.Fatal(snap, err)
	}
	s.Launched()
	if s.Snapshot().Phase != "launch" {
		t.Fatal("no launch signal")
	}
	if s.handle(r, "127.0.0.1").Error == "" {
		t.Fatal("late guest accepted")
	}
}
func TestGuestLeaveLeaseAndToken(t *testing.T) {
	s := fixture()
	r := join(s)
	reply := s.handle(r, "127.0.0.1")
	r.Action = "poll"
	r.Token = "wrong"
	if s.handle(r, "127.0.0.1").Error == "" {
		t.Fatal("invalid token accepted")
	}
	r.Token = reply.Token
	if s.handle(r, "127.0.0.2").Error == "" {
		t.Fatal("different IP accepted")
	}
	if s.handle(r, "127.0.0.1").Error != "" {
		t.Fatal("heartbeat rejected")
	}
	r.Action = "leave"
	if s.handle(r, "127.0.0.1").Session.Guest != "" {
		t.Fatal("guest remains")
	}
	r = join(s)
	s.handle(r, "127.0.0.1")
	s.lastSeen = time.Now().Add(-Lease - time.Second)
	if s.Snapshot().Guest != "" {
		t.Fatal("expired guest remains")
	}
	if _, err := s.Begin(); err == nil {
		t.Fatal("started with expired guest")
	}
}
func TestRoomUDPExchange(t *testing.T) {
	s, err := newAt("127.0.0.1", 0, fixture().session)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()
	r := join(s)
	reply, err := exchangeAt(ctx, "127.0.0.1", s.conn.LocalAddr().(*net.UDPAddr).Port, r)
	if err != nil || reply.Session.Guest != "Bob" {
		t.Fatal(reply, err)
	}
	r.Action = "poll"
	r.Token = reply.Token
	if _, err := s.Begin(); err != nil {
		t.Fatal(err)
	}
	s.Launched()
	reply, err = exchangeAt(ctx, "127.0.0.1", s.conn.LocalAddr().(*net.UDPAddr).Port, r)
	if err != nil || reply.Session.Phase != "launch" {
		t.Fatal(reply, err)
	}
	cancel()
	s.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
