package sender

import (
	"context"
	"sync"
)

// Fake is a Sender that records messages instead of sending them
// (FakeSender of component §15). FailNext makes the next calls fail, one error
// per call; a failed call records nothing.
type Fake struct {
	mu   sync.Mutex
	sent []Message
	errs []error
}

// Send implements Sender.
func (f *Fake) Send(ctx context.Context, chatID int64, text string, kb *Keyboard) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		return err
	}
	f.sent = append(f.sent, Message{ChatID: chatID, Text: text, Keyboard: kb.clone()})
	return nil
}

// FailNext queues errors for the next calls of Send.
func (f *Fake) FailNext(errs ...error) {
	f.mu.Lock()
	f.errs = append(f.errs, errs...)
	f.mu.Unlock()
}

// Sent returns a copy of the messages sent so far, in order.
func (f *Fake) Sent() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Message, len(f.sent))
	for i, m := range f.sent {
		out[i] = Message{ChatID: m.ChatID, Text: m.Text, Keyboard: m.Keyboard.clone()}
	}
	return out
}
