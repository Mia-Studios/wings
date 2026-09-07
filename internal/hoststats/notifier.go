package hoststats

import (
	"context"
	"net/http"
	"time"

	"github.com/apex/log"

	"github.com/pterodactyl/wings/events"
	"github.com/pterodactyl/wings/remote"
)

// PressureSender is the part of the Panel client the notifier needs. It exists
// so that tests can stand in for the Panel without an HTTP server.
type PressureSender interface {
	SendHostPressure(ctx context.Context, change remote.HostPressureRequest) error
}

// Notifier forwards changes of the overall pressure level to the Panel, which
// fans them out to its webhooks.
type Notifier struct {
	bus    *events.Bus
	client PressureSender
	// timeout bounds a single delivery, including the retries the Panel client
	// performs on its own.
	timeout time.Duration
}

// NewNotifier builds a notifier that listens on the given bus. Nothing is sent
// until Run is called.
func NewNotifier(bus *events.Bus, client PressureSender) *Notifier {
	return &Notifier{
		bus:     bus,
		client:  client,
		timeout: time.Minute * 2,
	}
}

// Run forwards pressure changes until the context is canceled or the Panel
// answers that it does not know the endpoint. It blocks and is expected to be
// called in its own routine.
func (n *Notifier) Run(ctx context.Context) {
	ch := make(chan []byte, 8)
	n.bus.On(ch)
	defer n.bus.Off(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case b := <-ch:
			change, ok := decodePressureChange(b)
			if !ok {
				continue
			}
			// Anything that queued up while the previous delivery was in flight
			// is stale: only the level the host is at right now matters.
			change = n.latest(ch, change)
			if !n.send(ctx, change) {
				return
			}
		}
	}
}

// latest drains the channel and returns the most recent pressure change on it,
// or the given one when nothing newer is waiting.
func (n *Notifier) latest(ch <-chan []byte, change PressureChange) PressureChange {
	for {
		select {
		case b := <-ch:
			if newer, ok := decodePressureChange(b); ok {
				change = newer
			}
		default:
			return change
		}
	}
}

// send delivers one change. It returns false when the notifier should stop
// because the Panel does not support the endpoint.
func (n *Notifier) send(ctx context.Context, change PressureChange) bool {
	l := log.WithField("subsystem", "host_monitor").
		WithField("previous", change.Previous).
		WithField("current", change.Current)

	ctx, cancel := context.WithTimeout(ctx, n.timeout)
	defer cancel()

	err := n.client.SendHostPressure(ctx, remote.HostPressureRequest{
		Previous: string(change.Previous),
		Current:  string(change.Current),
		Snapshot: change.Snapshot,
	})
	if err == nil {
		l.Debug("host pressure change reported to the panel")
		return true
	}

	if rerr := remote.AsRequestError(err); rerr != nil && rerr.StatusCode() == http.StatusNotFound {
		l.Warn("the panel does not support host pressure notifications, no further changes will be reported until wings restarts")
		return false
	}

	l.WithField("error", err).Error("failed to report the host pressure change to the panel, dropping it")
	return true
}

// decodePressureChange turns a raw bus message into a pressure change, telling
// the caller when the message was about something else.
func decodePressureChange(b []byte) (PressureChange, bool) {
	var e struct {
		Topic string         `json:"Topic"`
		Data  PressureChange `json:"Data"`
	}
	if err := events.DecodeTo(b, &e); err != nil || e.Topic != PressureEvent {
		return PressureChange{}, false
	}
	return e.Data, true
}
