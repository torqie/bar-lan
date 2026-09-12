// Package lan provides a versioned, bounded UDP request/reply discovery protocol.
// Advertisements are untrusted metadata, never executable paths or start scripts.
package lan

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/torqie/bar-lan/internal/recoil"
	"net"
	"time"
)

const Port = 8453
const Query = "bar-lan/discover/1"

type Session struct {
	Version       int    `json:"version"`
	IP            string `json:"-"`
	Port          int    `json:"port"`
	Host          string `json:"host"`
	Guest         string `json:"guest"`
	Game          string `json:"game"`
	Map           string `json:"map"`
	EngineSHA256  string `json:"engine_sha256"`
	EngineVersion string `json:"engine_version,omitempty"`
	RoomID        string `json:"room_id,omitempty"`
	Phase         string `json:"phase,omitempty"`
	GameChecksum  uint32 `json:"game_checksum,omitempty"`
	MapChecksum   uint32 `json:"map_checksum,omitempty"`
}

func (s Session) Validate() error {
	if (s.Version != 1 && s.Version != 2) || s.Port < 1 || s.Port > 65535 || len(s.EngineSHA256) != 64 {
		return fmt.Errorf("invalid advertisement")
	}
	guest := s.Guest
	if s.Version == 2 {
		if len(s.RoomID) != 32 || s.GameChecksum == 0 || s.MapChecksum == 0 {
			return fmt.Errorf("invalid room metadata")
		}
		if s.Phase != "waiting" && s.Phase != "starting" && s.Phase != "launch" {
			return fmt.Errorf("invalid room state")
		}
		if guest == "" {
			guest = s.Host + "_guest"
		}
	}
	_, err := recoil.Host(s.Port, s.Host, guest, s.Game, s.Map)
	return err
}
func Listen(bind string) (*net.UDPConn, error) {
	return net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(bind), Port: Port})
}
func Serve(ctx context.Context, c *net.UDPConn, s Session) error {
	if err := s.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			return nil
		}
		_ = c.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, addr, err := c.ReadFromUDP(buf)
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				continue
			}
			return err
		}
		if string(buf[:n]) == Query {
			_, _ = c.WriteToUDP(b, addr)
		}
	}
}
func Discover(ctx context.Context, target string, duration time.Duration) ([]Session, error) {
	ip := net.ParseIP(target)
	if ip == nil || ip.To4() == nil {
		return nil, fmt.Errorf("discovery target must be IPv4")
	}
	c, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	addr := &net.UDPAddr{IP: ip, Port: Port}
	end := time.Now().Add(duration)
	next := time.Time{}
	buf := make([]byte, 4096)
	seen := map[string]bool{}
	var sessions []Session
	for time.Now().Before(end) && ctx.Err() == nil {
		if time.Now().After(next) {
			if _, err = c.WriteToUDP([]byte(Query), addr); err != nil {
				return nil, err
			}
			next = time.Now().Add(time.Second)
		}
		_ = c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		n, from, err := c.ReadFromUDP(buf)
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				continue
			}
			return nil, err
		}
		var s Session
		if json.Unmarshal(buf[:n], &s) != nil || s.Validate() != nil {
			continue
		}
		s.IP = from.IP.String()
		key := fmt.Sprintf("%s:%d", s.IP, s.Port)
		if !seen[key] {
			seen[key] = true
			sessions = append(sessions, s)
		}
	}
	return sessions, ctx.Err()
}
