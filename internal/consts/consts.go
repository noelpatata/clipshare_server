// Package consts holds non-configurable application-wide constants.
// User-tunable values belong in config.Config instead.
package consts

const (
	WSPath        = "/ws"
	APIStatusPath = "/status"
	APISendPath   = "/send"
)

const (
	ServiceType = "_clipshare._tcp"
)

const (
	SchemeWS  = "ws"
	SchemeWSS = "wss"
)

const (
	ContentTypeJSON = "application/json"
	Localhost       = "127.0.0.1"
)

const (
	DefaultImageMime = "image/png"
)

const (
	CertsDir    = "certs"
	P12Password = "clipshare"
)
