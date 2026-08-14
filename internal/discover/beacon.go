package discover

import (
	"encoding/json"
	"time"
)

// BeaconMsg is the JSON broadcast sent over UDP to 255.255.255.255:40404.
type BeaconMsg struct {
	Name string `json:"name"`
	Port int    `json:"port"`
	TLS  bool   `json:"tls,omitempty"`
	TS   int64  `json:"ts"`
}

func beaconJSON(name string, port int, tls bool) []byte {
	b, err := json.Marshal(BeaconMsg{Name: name, Port: port, TLS: tls, TS: time.Now().Unix()})
	if err != nil {
		return []byte("{}")
	}
	return b
}
