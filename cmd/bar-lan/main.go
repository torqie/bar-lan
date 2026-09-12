package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/torqie/bar-lan/internal/content"
	"github.com/torqie/bar-lan/internal/install"
	"github.com/torqie/bar-lan/internal/lan"
	"github.com/torqie/bar-lan/internal/recoil"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "bar-lan:", err)
		os.Exit(1)
	}
}

type engineStartedKey struct{}

func run(ctx context.Context, args []string) error { return runWithOutput(ctx, args, os.Stdout) }

// Serialize engine output and companion status writes for all output sinks.
type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}
func runWithOutput(ctx context.Context, args []string, output io.Writer) error {
	output = &lockedWriter{writer: output}
	if len(args) > 0 && args[0] == "content-worker" {
		return content.Worker(args[1:])
	}
	if len(args) == 0 || args[0] == "gui" {
		return serveGUI(ctx)
	}
	if args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(output, "bar-lan: gui | detect | discover | host | join\nUse bar-lan <command> --help for options.")
		return nil
	}
	command := args[0]
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	data := f.String("data", "", "BAR data directory (or BAR_DATA_DIR)")
	engine := f.String("engine", "", "absolute path to matching spring.exe/Recoil executable")
	target := f.String("target", "255.255.255.255", "discovery broadcast, subnet broadcast, or host IPv4")
	duration := f.Duration("timeout", 3*time.Second, "discovery duration (0–30s)")
	port := f.Int("port", 8452, "Recoil UDP port")
	name := f.String("name", "", "your player name (host default: Host; discovered guest default: reserved name)")
	guest := f.String("guest", "Guest", "reserved remote player name")
	game := f.String("game", "", "exact installed game name/archive; pin same version on both PCs")
	mapName := f.String("map", "", "exact installed map name, including .smf")
	hostIP := f.String("host", "", "host IP; skips discovery for join")
	advertise := f.Bool("advertise", true, "advertise legacy CLI host (desktop room owns discovery separately)")
	dry := f.Bool("dry-run", false, "print start script and launch arguments without launching")
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", f.Args())
	}
	if *duration <= 0 || *duration > 30*time.Second {
		return fmt.Errorf("timeout must be greater than zero and at most 30s")
	}
	switch command {
	case "detect":
		found := install.Detect()
		if *data != "" {
			found = []install.Installation{{Data: *data, Engines: install.Engines(*data)}}
		}
		if len(found) == 0 {
			return fmt.Errorf("no install detected; use --data for a custom or portable install")
		}
		return json.NewEncoder(output).Encode(found)
	case "discover":
		sessions, err := lan.Discover(ctx, *target, *duration)
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			return fmt.Errorf("no host found; check firewall or try --target HOST_IP")
		}
		return json.NewEncoder(output).Encode(publicSessions(sessions))
	case "host", "join":
	default:
		return fmt.Errorf("unknown command %q", command)
	}
	d, e, err := install.Resolve(*data, *engine)
	if err != nil {
		return err
	}
	var script string
	var session lan.Session
	if command == "host" {
		if *port == lan.Port {
			return fmt.Errorf("game port must differ from discovery port %d", lan.Port)
		}
		if *name == "" {
			*name = "Host"
		}
		script, err = recoil.Host(*port, *name, *guest, *game, *mapName)
		if err != nil {
			return err
		}
		hash, err := install.Hash(e)
		if err != nil {
			return err
		}
		session = lan.Session{Version: 1, Port: *port, Host: *name, Guest: *guest, Game: *game, Map: *mapName, EngineSHA256: hash}
	} else {
		if *hostIP == "" {
			sessions, err := lan.Discover(ctx, *target, *duration)
			if err != nil {
				return err
			}
			if len(sessions) != 1 {
				return fmt.Errorf("found %d hosts; use --target HOST_IP to select one or --host IP --name RESERVED_NAME for manual join", len(sessions))
			}
			s := sessions[0]
			hash, err := install.Hash(e)
			if err != nil {
				return err
			}
			if hash != s.EngineSHA256 {
				return fmt.Errorf("engine differs from host; select the same installed engine build")
			}
			if *name == "" {
				*name = s.Guest
			}
			if *name != s.Guest {
				return fmt.Errorf("host reserved player name %q; use that name", s.Guest)
			}
			*hostIP = s.IP
			*port = s.Port
			fmt.Fprintf(output, "Host %s, game %q, map %q. Both must already be installed locally.\n", s.IP, s.Game, s.Map)
		} else if *name == "" {
			return fmt.Errorf("manual join requires --name matching the host's --guest")
		}
		script, err = recoil.Client(*hostIP, *port, *name)
		if err != nil {
			return err
		}
	}
	if *dry {
		fmt.Fprint(output, script)
		fmt.Fprintf(output, "Executable: %s\nArguments: %q\n", e, []string{"--isolation", "--write-dir", d, "<temporary-start-script>"})
		return nil
	}
	// Bind before launching so a discovery port conflict fails without spawning a game.
	var listener *net.UDPConn
	if command == "host" && *advertise {
		listener, err = lan.Listen("0.0.0.0")
		if err != nil {
			return fmt.Errorf("discovery UDP %d: %w", lan.Port, err)
		}
		defer listener.Close()
	}
	tmp, err := os.CreateTemp("", "bar-lan-*.txt")
	if err != nil {
		return err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err = tmp.WriteString(script); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, e, "--isolation", "--write-dir", d, path)
	cmd.Dir = d

	cmd.Stdout = output
	cmd.Stderr = output
	if err = cmd.Start(); err != nil {
		return err
	}
	if started, ok := ctx.Value(engineStartedKey{}).(func()); ok {
		started()
	}
	advertiseCtx, stop := context.WithCancel(ctx)
	defer stop()
	if listener != nil {
		go func() {
			if err := lan.Serve(advertiseCtx, listener, session); err != nil && advertiseCtx.Err() == nil {
				fmt.Fprintln(output, "Discovery stopped:", err)
			}
		}()
	}
	fmt.Fprintf(output, "Recoil started. Log: %s\n", filepath.Join(d, "infolog.txt"))
	if command == "host" {
		fmt.Fprintf(output, "Waiting for %q on UDP %d. Discovery UDP %d. Keep this console open.\n", *guest, *port, lan.Port)
	}
	return cmd.Wait()
}

// Include observed IPs in CLI output while never trusting a claimed packet IP.
func publicSessions(sessions []lan.Session) []any {
	r := make([]any, 0, len(sessions))
	for _, s := range sessions {
		r = append(r, struct {
			IP string `json:"ip"`
			lan.Session
		}{s.IP, s})
	}
	return r
}
