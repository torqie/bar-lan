package main

import (
	"context"
	"fmt"
	"github.com/torqie/bar-lan/internal/install"
	"github.com/torqie/bar-lan/internal/lan"
	"github.com/torqie/bar-lan/internal/recoil"
	"net"
	"sync"
)

type gameRequest struct {
	RoomLaunch bool
	Mode       string `json:"mode"`
	Data       string `json:"data"`
	Engine     string `json:"engine"`
	Name       string `json:"name"`
	Guest      string `json:"guest"`
	Game       string `json:"game"`
	Map        string `json:"map"`
	Host       string `json:"host"`
	Target     string `json:"target"`
	Port       int    `json:"port"`
}

func (r gameRequest) arguments() ([]string, error) {
	d, e, err := install.Resolve(r.Data, r.Engine)
	if err != nil {
		return nil, err
	}
	a := []string{r.Mode, "--data", d, "--engine", e}
	switch r.Mode {
	case "host":
		if r.Port == lan.Port {
			return nil, fmt.Errorf("game port conflicts with discovery port %d", lan.Port)
		}
		if _, err := recoil.Host(r.Port, r.Name, r.Guest, r.Game, r.Map); err != nil {
			return nil, err
		}
		if r.RoomLaunch {
			a = append(a, "--advertise=false")
		}
		a = append(a, "--name", r.Name, "--guest", r.Guest, "--game", r.Game, "--map", r.Map, "--port", fmt.Sprint(r.Port))
	case "join":
		if r.Host != "" {
			if _, err := recoil.Client(r.Host, r.Port, r.Name); err != nil {
				return nil, err
			}
			a = append(a, "--host", r.Host, "--port", fmt.Sprint(r.Port), "--name", r.Name)
		} else {
			if net.ParseIP(r.Target) == nil {
				return nil, fmt.Errorf("select a discovered host first")
			}
			a = append(a, "--target", r.Target)
			if r.Name != "" {
				a = append(a, "--name", r.Name)
			}
		}
	default:
		return nil, fmt.Errorf("mode must be host or join")
	}
	return a, nil
}

type guiState struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	Started bool
	Running bool   `json:"running"`
	Status  string `json:"status"`
	Log     string `json:"log"`
}

func (s *guiState) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Log += string(p)
	if len(s.Log) > 32000 {
		s.Log = s.Log[len(s.Log)-32000:]
	}
	return len(p), nil
}
func (s *guiState) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.Status = "Stopping"
	}
}
func (s *guiState) start(ctx context.Context, r gameRequest) error {
	args, err := r.arguments()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Running {
		return fmt.Errorf("a game is already running")
	}
	gameCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.Running = true
	s.Started = false
	gameCtx = context.WithValue(gameCtx, engineStartedKey{}, func() { s.mu.Lock(); s.Started = true; s.mu.Unlock() })
	s.Status = "Starting Recoil"
	s.Log = ""
	go func() {
		err := runWithOutput(gameCtx, args, s)
		stopped := gameCtx.Err() != nil
		cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.Running = false
		s.cancel = nil
		if stopped {
			s.Status = "Stopped"
		} else if err != nil {
			s.Status = "Launch / game error"
			s.Log += "\n" + err.Error()
		} else {
			s.Status = "Game closed"
		}
	}()
	return nil
}
