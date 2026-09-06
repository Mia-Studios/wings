package websocket

import "github.com/pterodactyl/wings/internal/hoststats"

type Event string

const (
	AuthenticationSuccessEvent = Event("auth success")
	TokenExpiringEvent         = Event("token expiring")
	TokenExpiredEvent          = Event("token expired")
	AuthenticationEvent        = Event("auth")
	SetStateEvent              = Event("set state")
	SendServerLogsEvent        = Event("send logs")
	SendCommandEvent           = Event("send command")
	SendStatsEvent             = Event("send stats")
	ErrorEvent                 = Event("daemon error")
	JwtErrorEvent              = Event("jwt error")
	ThrottledEvent             = Event("throttled")
	// HostStatsEvent carries a utilization sample of the machine this server is
	// running on and is only sent to administrators.
	HostStatsEvent = Event(hoststats.StatsEvent)
)

type Message struct {
	// The event to perform.
	Event Event `json:"event"`

	// The data to pass along, only used by power/command currently. Other requests
	// should either omit the field or pass an empty value as it is ignored.
	Args []string `json:"args,omitempty"`
}
