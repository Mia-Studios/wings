package hoststats

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"emperror.dev/errors"
	"github.com/stretchr/testify/assert"

	"github.com/pterodactyl/wings/events"
	"github.com/pterodactyl/wings/remote"
)

// fakePanel records what the notifier sends and lets a test hold a delivery
// open or fail it.
type fakePanel struct {
	mu       sync.Mutex
	requests []remote.HostPressureRequest
	// release, when set, blocks every delivery until the test closes it.
	release chan struct{}
	err     error
	// sent receives once per delivery, after it has been recorded.
	sent chan struct{}
}

func newFakePanel() *fakePanel {
	return &fakePanel{sent: make(chan struct{}, 16)}
}

func (f *fakePanel) SendHostPressure(ctx context.Context, change remote.HostPressureRequest) error {
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	f.requests = append(f.requests, change)
	err := f.err
	f.mu.Unlock()
	f.sent <- struct{}{}
	return err
}

func (f *fakePanel) received() []remote.HostPressureRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]remote.HostPressureRequest(nil), f.requests...)
}

func waitFor(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second * 2):
		t.Fatal("timed out waiting for the notifier")
	}
}

func publishChange(bus *events.Bus, previous, current Level) {
	bus.Publish(PressureEvent, PressureChange{
		Previous: previous,
		Current:  current,
		Snapshot: Snapshot{Pressure: Pressure{Level: current}},
	})
}

func TestNotifierForwardsPressureChanges(t *testing.T) {
	bus := events.NewBus()
	panel := newFakePanel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go NewNotifier(bus, panel).Run(ctx)
	// The bus only reaches subscribers, so give Run a moment to register.
	time.Sleep(time.Millisecond * 20)

	bus.Publish(StatsEvent, Snapshot{})
	publishChange(bus, LevelOK, LevelCritical)
	waitFor(t, panel.sent)

	requests := panel.received()
	if assert.Len(t, requests, 1) {
		assert.Equal(t, "ok", requests[0].Previous)
		assert.Equal(t, "critical", requests[0].Current)
		snapshot, _ := requests[0].Snapshot.(Snapshot)
		assert.Equal(t, LevelCritical, snapshot.Pressure.Level)
	}
}

func TestNotifierOnlySendsTheLatestQueuedChange(t *testing.T) {
	bus := events.NewBus()
	panel := newFakePanel()
	panel.release = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go NewNotifier(bus, panel).Run(ctx)
	time.Sleep(time.Millisecond * 20)

	// The first change is held open by the fake Panel while three more arrive.
	publishChange(bus, LevelOK, LevelWarning)
	time.Sleep(time.Millisecond * 20)
	publishChange(bus, LevelWarning, LevelCritical)
	publishChange(bus, LevelCritical, LevelWarning)
	publishChange(bus, LevelWarning, LevelOK)
	time.Sleep(time.Millisecond * 20)

	close(panel.release)
	waitFor(t, panel.sent)
	waitFor(t, panel.sent)

	// Nothing else should be delivered afterwards.
	select {
	case <-panel.sent:
		t.Fatal("stale pressure changes were delivered")
	case <-time.After(time.Millisecond * 100):
	}

	requests := panel.received()
	if assert.Len(t, requests, 2) {
		assert.Equal(t, "warning", requests[0].Current)
		assert.Equal(t, "ok", requests[1].Current)
	}
}

func TestNotifierKeepsRunningAfterADeliveryError(t *testing.T) {
	bus := events.NewBus()
	panel := newFakePanel()
	panel.err = errors.New("panel unreachable")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go NewNotifier(bus, panel).Run(ctx)
	time.Sleep(time.Millisecond * 20)

	publishChange(bus, LevelOK, LevelWarning)
	waitFor(t, panel.sent)
	publishChange(bus, LevelWarning, LevelOK)
	waitFor(t, panel.sent)

	assert.Len(t, panel.received(), 2)
}

func TestNotifierStopsWhenThePanelDoesNotKnowTheEndpoint(t *testing.T) {
	bus := events.NewBus()
	panel := newFakePanel()
	panel.err = notFound()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		NewNotifier(bus, panel).Run(ctx)
		close(done)
	}()
	time.Sleep(time.Millisecond * 20)

	publishChange(bus, LevelOK, LevelWarning)
	waitFor(t, panel.sent)
	waitFor(t, done)

	publishChange(bus, LevelWarning, LevelOK)
	select {
	case <-panel.sent:
		t.Fatal("a change was delivered after the notifier stopped")
	case <-time.After(time.Millisecond * 100):
	}
}

// notFound builds the error the Panel client returns for a 404, the way the
// client itself does, so that the notifier's detection is exercised for real.
// The client does not retry 4XX responses, so this is quick.
func notFound() error {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
		_, _ = rw.Write([]byte(`{"errors":[{"code":"NotFoundHttpException","status":"404","detail":"Not found"}]}`))
	}))
	defer server.Close()

	c := remote.New(server.URL, remote.WithHttpClient(server.Client()), remote.WithCredentials("id", "token"))
	return c.SendHostPressure(context.Background(), remote.HostPressureRequest{})
}
