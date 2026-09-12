// Package room implements a two-person pre-game room on the discovery UDP port.
// It is a trusted-LAN protocol, not an Internet matchmaking or authentication service.
package room

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/torqie/bar-lan/internal/lan"
	"github.com/torqie/bar-lan/internal/recoil"
	"net"
	"strings"
	"sync"
	"time"
)

const Lease = 10 * time.Second

type Request struct {
	Version                                             int
	Action, RoomID, ClientID, Token, Name, EngineSHA256 string
	GameChecksum, MapChecksum                           uint32
}
type Reply struct {
	Session lan.Session
	Token   string
	Error   string
}

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

type Server struct {
	mu                        sync.Mutex
	session                   lan.Session
	conn                      *net.UDPConn
	clientID, clientIP, token string
	lastSeen                  time.Time
	closed                    bool
}

func New(bind string, s lan.Session) (*Server, error) { return newAt(bind, lan.Port, s) }
func newAt(bind string, port int, s lan.Session) (*Server, error) {
	s.Version = 2
	s.RoomID = ID()
	s.Phase = "waiting"
	s.Guest = ""
	if err := s.Validate(); err != nil {
		return nil, err
	}
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP(bind), Port: port})
	if err != nil {
		return nil, err
	}
	return &Server{session: s, conn: c}, nil
}
func (s *Server) expire() {
	if s.session.Phase == "waiting" && s.session.Guest != "" && time.Since(s.lastSeen) > Lease {
		s.session.Guest = ""
		s.token = ""
		s.clientID = ""
		s.clientIP = ""
	}
}
func (s *Server) Snapshot() lan.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	return s.session
}
func (s *Server) Begin() (lan.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	if s.closed || s.session.Phase != "waiting" || s.session.Guest == "" {
		return s.session, fmt.Errorf("wait for a compatible guest to join first")
	}
	s.session.Phase = "starting"
	return s.session, nil
}
func (s *Server) Launched() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed && s.session.Phase == "starting" {
		s.session.Phase = "launch"
	}
}
func (s *Server) Close() { s.mu.Lock(); s.closed = true; s.mu.Unlock(); s.conn.Close() }
func (s *Server) handle(r Request, ip string) Reply {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	reply := Reply{Session: s.session}
	fail := func(text string) Reply { reply.Error = text; return reply }
	if r.Version != 2 || r.RoomID != s.session.RoomID {
		return fail("This room changed. Find games again.")
	}
	if r.Action == "join" {
		if s.session.Phase != "waiting" {
			return fail("This match has already started.")
		}
		if len(r.ClientID) != 32 {
			return fail("Invalid join request.")
		}
		if err := recoil.Value(r.Name); err != nil {
			return fail("Enter a valid player name.")
		}
		if strings.EqualFold(r.Name, s.session.Host) {
			return fail("Choose a different name from the host.")
		}
		if r.EngineSHA256 != s.session.EngineSHA256 {
			return fail("Engine versions differ. Update BAR on both PCs, then reopen BAR LAN.")
		}
		if r.GameChecksum != s.session.GameChecksum || r.MapChecksum != s.session.MapChecksum {
			return fail("Game or map differs. Download the host's game and map using BAR first.")
		}
		if s.session.Guest != "" && (r.ClientID != s.clientID || ip != s.clientIP) {
			return fail("This room already has a guest.")
		}
		if s.session.Guest == "" {
			s.session.Guest = r.Name
			s.clientID = r.ClientID
			s.clientIP = ip
			s.token = ID()
		}
		s.lastSeen = time.Now()
		return Reply{Session: s.session, Token: s.token}
	}
	if r.Token == "" || r.Token != s.token || ip != s.clientIP {
		return fail("Your place in this room expired. Join again.")
	}
	if r.Action == "leave" {
		if s.session.Phase != "waiting" {
			return fail("The match is starting.")
		}
		s.session.Guest = ""
		s.token = ""
		s.clientID = ""
		s.clientIP = ""
	} else if r.Action == "poll" {
		s.lastSeen = time.Now()
	} else {
		return fail("Unknown room request.")
	}
	return Reply{Session: s.session, Token: s.token}
}
func (s *Server) Serve(ctx context.Context) error {
	buf := make([]byte, 8192)
	for ctx.Err() == nil {
		s.conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				continue
			}
			if ctx.Err() != nil || errorsClosed(err) {
				return nil
			}
			return err
		}
		var value any
		if string(buf[:n]) == lan.Query {
			value = s.Snapshot()
		} else {
			var req Request
			if json.Unmarshal(buf[:n], &req) != nil {
				continue
			}
			value = s.handle(req, addr.IP.String())
		}
		b, _ := json.Marshal(value)
		s.conn.WriteToUDP(b, addr)
	}
	return nil
}
func errorsClosed(err error) bool { return strings.Contains(err.Error(), "closed network connection") }

// Exchange retries the same idempotent request; replies are source-filtered by a connected UDP socket.
func Exchange(ctx context.Context, ip string, r Request) (Reply, error) {
	return exchangeAt(ctx, ip, lan.Port, r)
}
func exchangeAt(ctx context.Context, ip string, port int, r Request) (Reply, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return Reply{}, fmt.Errorf("enter the host's IPv4 address")
	}
	c, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: parsed, Port: port})
	if err != nil {
		return Reply{}, err
	}
	defer c.Close()
	b, _ := json.Marshal(r)
	buf := make([]byte, 8192)
	for attempt := 0; attempt < 4 && ctx.Err() == nil; attempt++ {
		c.SetDeadline(time.Now().Add(500 * time.Millisecond))
		if _, err = c.Write(b); err != nil {
			continue
		}
		n, e := c.Read(buf)
		if e != nil {
			err = e
			continue
		}
		var reply Reply
		if e = json.Unmarshal(buf[:n], &reply); e != nil {
			err = e
			continue
		}
		if reply.Error != "" {
			return reply, fmt.Errorf("%s", reply.Error)
		}
		if reply.Session.Validate() != nil || reply.Session.RoomID != r.RoomID {
			err = fmt.Errorf("invalid room reply")
			continue
		}
		return reply, nil
	}
	if ctx.Err() != nil {
		return Reply{}, ctx.Err()
	}
	return Reply{}, fmt.Errorf("host did not respond: %v", err)
}
