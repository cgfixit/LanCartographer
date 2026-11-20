package macdiag

import (
	"cmp"
	"net/netip"
	"strconv"
	"strings"
	"unicode"
)

const maxConnections = 10_000

type Endpoint struct {
	IP       netip.Addr
	Port     uint16
	Wildcard bool // port is unspecified
}

func (e Endpoint) String() string {
	port := strconv.Itoa(int(e.Port))
	if e.Wildcard {
		port = "*"
	}
	switch {
	case e.IP.Is6():
		return "[" + e.IP.String() + "]:" + port
	case e.IP.Is4():
		return e.IP.String() + ":" + port
	default:
		return "*:" + port
	}
}

func (e Endpoint) Compare(other Endpoint) int {
	if n := e.IP.Compare(other.IP); n != 0 {
		return n
	}
	return cmp.Compare(e.Port, other.Port)
}

type Connection struct {
	Process, FD, Family, Protocol, State string
	PID                                  int
	Local, Peer                          Endpoint
	HasPeer                              bool
}

type Snapshot struct {
	Rows    []Connection
	Partial bool
}

func parseEndpoint(value string) (Endpoint, bool) {
	host, port, ok := strings.Cut(value, "]:")
	if ok {
		if !strings.HasPrefix(host, "[") {
			return Endpoint{}, false
		}
		host = host[1:]
	} else {
		host, port, ok = strings.Cut(value, ":")
	}
	if !ok {
		return Endpoint{}, false
	}
	var e Endpoint
	if host != "*" {
		var err error
		e.IP, err = netip.ParseAddr(host)
		if err != nil || strings.IndexFunc(e.IP.Zone(), func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-')
		}) >= 0 {
			return Endpoint{}, false
		}
	}
	if port == "*" {
		e.Wildcard = true
		return e, true
	}
	n, err := strconv.ParseUint(port, 10, 16)
	e.Port = uint16(n)
	return e, err == nil
}

func printable(value string) string {
	return strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) {
			return '�'
		}
		return r
	}, value)
}

// With -F0, NUL terminates fields and a newline terminates each process or file set.
func parseConnections(data []byte) Snapshot {
	var snapshot Snapshot
	var pid int
	var process string
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\x00\n") {
		snapshot.Partial = true
		last := strings.LastIndex(text, "\x00\n")
		if last < 0 {
			return snapshot
		}
		text = text[:last+2]
	}
	for record := range strings.SplitSeq(text, "\x00\n") {
		if record == "" {
			continue
		}
		first, fields, _ := strings.Cut(record, "\x00")
		if first == "" {
			pid, process = 0, ""
			snapshot.Partial = true
			continue
		}
		switch first[0] {
		case 'p':
			pid, _ = strconv.Atoi(first[1:])
			process = ""
			for field := range strings.SplitSeq(fields, "\x00") {
				if strings.HasPrefix(field, "c") {
					process = printable(field[1:])
				}
			}
			snapshot.Partial = snapshot.Partial || pid <= 0
		case 'f':
			row := Connection{PID: pid, Process: process, FD: printable(first[1:])}
			var localOK, peerOK bool
			for field := range strings.SplitSeq(fields, "\x00") {
				if field == "" {
					continue
				}
				switch field[0] {
				case 'P':
					row.Protocol = field[1:]
				case 't':
					row.Family = field[1:]
				case 'n':
					local, peer, hasPeer := strings.Cut(field[1:], "->")
					row.Local, localOK = parseEndpoint(local)
					row.Peer, peerOK = parseEndpoint(peer)
					row.HasPeer = hasPeer
				case 'T':
					if state, ok := strings.CutPrefix(field, "TST="); ok {
						row.State = printable(state)
					}
				}
			}
			if pid <= 0 || row.FD == "" || !localOK || (row.HasPeer && !peerOK) ||
				(row.Protocol != "TCP" && row.Protocol != "UDP") || (row.Family != "IPv4" && row.Family != "IPv6") {
				snapshot.Partial = true
				continue
			}
			if len(snapshot.Rows) == maxConnections {
				snapshot.Partial = true
				return snapshot
			}
			snapshot.Rows = append(snapshot.Rows, row)
		default:
			pid, process = 0, ""
			snapshot.Partial = true
		}
	}
	return snapshot
}
