package recoil

import (
	"fmt"
	"net"
	"strings"
	"unicode"
)

// Value rejects TDF delimiters instead of attempting undocumented escaping.
func Value(s string) error {
	if strings.TrimSpace(s) == "" || len(s) > 256 {
		return fmt.Errorf("value must contain 1–256 bytes")
	}
	for _, r := range s {
		if unicode.IsControl(r) || strings.ContainsRune(";{}[]=\\\"", r) {
			return fmt.Errorf("unsafe start-script character in %q", s)
		}
	}
	if strings.Contains(s, "//") || strings.Contains(s, "/*") {
		return fmt.Errorf("comments are not allowed in values")
	}
	return nil
}

func Client(ip string, port int, name string) (string, error) {
	if net.ParseIP(ip) == nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("an IP address and port 1–65535 are required")
	}
	if err := Value(name); err != nil {
		return "", err
	}
	return fmt.Sprintf("[GAME]\n{\n HostIP=%s;\n HostPort=%d;\n SourcePort=0;\n MyPlayerName=%s;\n IsHost=0;\n}\n", ip, port, name), nil
}

func Host(port int, host, guest, game, mapName string) (string, error) {
	for _, s := range []string{host, guest, game, mapName} {
		if err := Value(s); err != nil {
			return "", err
		}
	}
	if strings.EqualFold(host, guest) {
		return "", fmt.Errorf("host and guest names must differ")
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("invalid game port")
	}
	return fmt.Sprintf(`[GAME]
{
 HostIP=0.0.0.0;
 HostPort=%d;
 MyPlayerName=%s;
 IsHost=1;
 GameType=%s;
 MapName=%s;
 StartPosType=0;
 NumPlayers=2;
 NumTeams=2;
 NumAllyTeams=2;
 [PLAYER0] { Name=%s; Spectator=0; Team=0; }
 [PLAYER1] { Name=%s; Spectator=0; Team=1; }
 [TEAM0] { TeamLeader=0; AllyTeam=0; Side=Armada; RGBColor=0.2 0.5 1; }
 [TEAM1] { TeamLeader=1; AllyTeam=1; Side=Cortex; RGBColor=1 0.3 0.2; }
 [ALLYTEAM0] { NumAllies=0; }
 [ALLYTEAM1] { NumAllies=0; }
}
`, port, host, game, mapName, host, guest), nil
}
