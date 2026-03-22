// Package notify sends deployment outcome notifications to configured channels
// (Slack, Discord, generic webhooks). All dispatch is non-blocking — callers
// return immediately and each notifier runs in its own goroutine with an
// independent 10-second timeout.
package notify

import (
	"context"
	"log"
	"time"
)

// Notifier sends a notification for a completed deployment.
type Notifier interface {
	// Send delivers the notification. The context carries the per-notifier
	// timeout; implementations must respect it.
	Send(ctx context.Context, event Event) error
	// Name returns a human-readable identifier used in log messages.
	Name() string
}

// Event carries the deployment outcome that notifiers should report.
type Event struct {
	Service   string
	Status    string // "success", "failed", "rolled_back"
	Image     string
	Duration  time.Duration
	DeployID  string
	ServerURL string // base URL for a logs link — optional
}

// Dispatcher holds all configured notifiers and fires them concurrently.
type Dispatcher struct {
	notifiers []Notifier
}

// NewDispatcher creates a Dispatcher with the given notifiers.
func NewDispatcher(notifiers ...Notifier) *Dispatcher {
	return &Dispatcher{notifiers: notifiers}
}

// Notify fires all notifiers concurrently in a goroutine (non-blocking).
// Each notifier gets an independent 10-second context timeout. If no
// notifiers are configured the call is a no-op.
func (d *Dispatcher) Notify(event Event) {
	if len(d.notifiers) == 0 {
		return
	}
	go func() {
		for _, n := range d.notifiers {
			n := n // capture loop variable
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := n.Send(ctx, event); err != nil {
					log.Printf("Warning: notifier %q failed for deployment %q: %v", n.Name(), event.DeployID, err)
				}
			}()
		}
	}()
}
