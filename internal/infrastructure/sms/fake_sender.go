// Package sms contains SMS provider implementations. A real provider only has
// to implement serviceinterface.SMSSender and be selected in the injector.
package sms

import (
	"context"
	"log/slog"
	"sync"

	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/pkg/logger"
	"github.com/tahedig/too-good-to-go/backend/pkg/validation"
)

// Message is one SMS captured by the FakeSender.
type Message struct {
	Phone string
	Body  string
}

// FakeSender is the sandbox provider: nothing leaves the process. Messages
// are kept in memory (for tests) and, only when debugBody is true, the body is
// logged at debug level so developers can read the code locally.
type FakeSender struct {
	log       *slog.Logger
	debugBody bool

	mu   sync.Mutex
	sent []Message
}

// NewFakeSender returns the sandbox sender. Enable debugBody only outside production.
func NewFakeSender(log *slog.Logger, debugBody bool) *FakeSender {
	return &FakeSender{log: log, debugBody: debugBody}
}

var _ serviceinterface.SMSSender = (*FakeSender)(nil)

func (f *FakeSender) Send(ctx context.Context, phone, body string) error {
	f.mu.Lock()
	f.sent = append(f.sent, Message{Phone: phone, Body: body})
	f.mu.Unlock()

	l := logger.FromContext(ctx, f.log).With("provider", "fake", "phone", validation.MaskPhone(phone))
	if f.debugBody {
		l.Debug("fake sms sent", "body", body)
	} else {
		l.Info("fake sms sent")
	}
	return nil
}

// Sent returns a copy of every captured message.
func (f *FakeSender) Sent() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Message(nil), f.sent...)
}
