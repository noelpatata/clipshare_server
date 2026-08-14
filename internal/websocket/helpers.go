package websocket

import (
	"net"
	"strconv"
	"strings"
	"sync"
)

var idMu sync.Mutex
var idSeq uint64

func newID() string {
	idMu.Lock()
	defer idMu.Unlock()
	idSeq++
	return strconv.FormatUint(idSeq, 36)
}

// clientIP extracts the IP from a net/http RemoteAddr string.
func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return strings.Trim(host, "[]")
}
