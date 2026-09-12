package main

import (
	"context"
	"fmt"
	"github.com/torqie/bar-lan/internal/content"
	"github.com/torqie/bar-lan/internal/install"
	"github.com/torqie/bar-lan/internal/lan"
	"github.com/torqie/bar-lan/internal/room"
	"sync"
	"time"
)

type lobbyView struct {
	Active, CanStart     bool
	Status, Peer, Engine string
}
type lobby struct {
	mu         sync.Mutex
	view       lobbyView
	host       *room.Server
	cancel     context.CancelFunc
	game       guiState
	request    gameRequest
	ctx        context.Context
	generation uint64
}

func (l *lobby) snapshot() lobbyView {
	l.mu.Lock()
	defer l.mu.Unlock()
	v := l.view
	if l.host != nil {
		s := l.host.Snapshot()
		v.Peer = s.Guest
		v.CanStart = s.Guest != "" && s.Phase == "waiting"
		if v.CanStart {
			v.Status = "Both players ready — engine, game and map match"
		}
	}
	return v
}
func (l *lobby) begin(parent context.Context) (context.Context, uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.view.Active {
		return nil, 0, fmt.Errorf("leave the current room first")
	}
	l.ctx, l.cancel = context.WithCancel(parent)
	l.generation++
	l.view = lobbyView{Active: true, Status: "Checking installed content..."}
	return l.ctx, l.generation, nil
}
func (l *lobby) fail(g uint64, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if g != l.generation {
		return
	}
	if l.host != nil {
		l.host.Close()
		l.host = nil
	}
	if l.cancel != nil {
		l.cancel()
	}
	l.view.Active = false
	l.view.CanStart = false
	l.view.Status = err.Error()
}
func (l *lobby) hostRoom(parent context.Context, r gameRequest) error {
	ctx, g, err := l.begin(parent)
	if err != nil {
		return err
	}
	go func() {
		hash, err := install.Hash(r.Engine)
		if err != nil {
			l.fail(g, err)
			return
		}
		c, err := content.Scan(ctx, r.Data, r.Engine, "checksums", r.Game, r.Map)
		if err != nil {
			l.fail(g, err)
			return
		}
		server, err := room.New("0.0.0.0", lan.Session{Port: 8452, Host: r.Name, Game: r.Game, Map: r.Map, EngineSHA256: hash, EngineVersion: install.Version(r.Engine), GameChecksum: c.GameChecksum, MapChecksum: c.MapChecksum})
		if err != nil {
			l.fail(g, err)
			return
		}
		l.mu.Lock()
		if ctx.Err() != nil || g != l.generation {
			l.mu.Unlock()
			server.Close()
			return
		}
		l.host = server
		l.request = r
		l.view.Status = "Room open — waiting for your friend to join"
		l.view.Engine = install.Version(r.Engine)
		l.mu.Unlock()
		if err := server.Serve(ctx); err != nil {
			l.fail(g, err)
		}
	}()
	return nil
}
func (l *lobby) startHost() error {
	l.mu.Lock()
	if l.host == nil {
		l.mu.Unlock()
		return fmt.Errorf("open a room first")
	}
	s, err := l.host.Begin()
	if err != nil {
		l.mu.Unlock()
		return err
	}
	r := l.request
	r.Mode = "host"
	r.Guest = s.Guest
	r.Port = s.Port
	r.RoomLaunch = true
	ctx, g, server := l.ctx, l.generation, l.host
	l.view.CanStart = false
	l.view.Status = "Starting game on both PCs..."
	l.mu.Unlock()
	if err = l.game.start(ctx, r); err != nil {
		l.fail(g, err)
		return err
	}
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.game.mu.Lock()
				running, started, status := l.game.Running, l.game.Started, l.game.Status
				l.game.mu.Unlock()
				if !running {
					l.fail(g, fmt.Errorf("%s — see game log below", status))
					return
				}
				if started {
					server.Launched()
					l.mu.Lock()
					if g == l.generation {
						l.view.Status = "Game running — use Recoil's ready / start controls"
					}
					l.mu.Unlock()
				}
			}
		}
	}()
	return nil
}
func (l *lobby) joinRoom(parent context.Context, s lan.Session, r gameRequest) error {
	if s.Version != 2 {
		return fmt.Errorf("this host uses the old prototype; update BAR LAN on both PCs")
	}
	ctx, g, err := l.begin(parent)
	if err != nil {
		return err
	}
	go func() {
		hash, err := install.Hash(r.Engine)
		if err != nil {
			l.fail(g, err)
			return
		}
		if hash != s.EngineSHA256 {
			l.fail(g, fmt.Errorf("Engine mismatch: yours %s / host %s. Update BAR on both PCs or choose the host's build in installation settings.", install.Version(r.Engine), s.EngineVersion))
			return
		}
		c, err := content.Scan(ctx, r.Data, r.Engine, "checksums", s.Game, s.Map)
		if err != nil {
			l.fail(g, err)
			return
		}
		req := room.Request{Version: 2, Action: "join", RoomID: s.RoomID, ClientID: room.ID(), Name: r.Name, EngineSHA256: hash, GameChecksum: c.GameChecksum, MapChecksum: c.MapChecksum}
		reply, err := room.Exchange(ctx, s.IP, req)
		if err != nil {
			l.fail(g, err)
			return
		}
		req.Token = reply.Token
		req.Action = "poll"
		defer func() {
			leave, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			req.Action = "leave"
			_, _ = room.Exchange(leave, s.IP, req)
		}()
		l.mu.Lock()
		if g == l.generation {
			l.view.Peer = s.Host
			l.view.Engine = s.EngineVersion
			l.view.Status = "Versions match — waiting for the host to start"
		}
		l.mu.Unlock()
		for ctx.Err() == nil {
			reply, err = room.Exchange(ctx, s.IP, req)
			if err != nil {
				l.fail(g, fmt.Errorf("Lost the host: %w", err))
				return
			}
			if reply.Session.Phase == "launch" {
				// Recoil's process exists before its UDP game socket; allow initial loading.
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
				r.Mode = "join"
				r.Host = s.IP
				r.Port = s.Port
				if err = l.game.start(ctx, r); err != nil {
					l.fail(g, err)
					return
				}
				l.mu.Lock()
				if g == l.generation {
					l.view.Status = "Game launching — follow Recoil's ready / start controls"
				}
				l.mu.Unlock()
				for ctx.Err() == nil {
					select {
					case <-ctx.Done():
						return
					case <-time.After(300 * time.Millisecond):
					}
					l.game.mu.Lock()
					running, status := l.game.Running, l.game.Status
					l.game.mu.Unlock()
					if !running {
						l.fail(g, fmt.Errorf("%s — see game log below", status))
						return
					}
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
	return nil
}
func (l *lobby) close() {
	l.mu.Lock()
	l.generation++
	if l.cancel != nil {
		l.cancel()
	}
	if l.host != nil {
		l.host.Close()
		l.host = nil
	}
	l.view = lobbyView{Status: "Room closed"}
	l.mu.Unlock()
	l.game.stop()
}
