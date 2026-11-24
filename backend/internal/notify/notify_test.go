package notify

import (
	"context"
	"errors"
	"testing"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

func TestNormalizeIsraeliPhone(t *testing.T) {
	cases := map[string]string{
		"050-123-4567":   "+972501234567",
		"+972501234567":  "+972501234567",
		"972 50 1234567": "+972501234567",
		"":               "",
		"abc":            "",
	}
	for in, want := range cases {
		if got := NormalizeIsraeliPhone(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

type memRecorder struct{ got []domain.Notification }

func (m *memRecorder) CreateNotification(_ context.Context, n domain.Notification) (domain.Notification, error) {
	m.got = append(m.got, n)
	return n, nil
}

type fakeChannel struct {
	name string
	err  error
}

func (f fakeChannel) Name() string { return f.name }
func (f fakeChannel) Send(context.Context, Message) (string, error) {
	return "addr", f.err
}

func TestDispatcherRecordsAttemptsAndSkipsMissingAddress(t *testing.T) {
	rec := &memRecorder{}
	d := &Dispatcher{rec: rec, channels: []Channel{
		fakeChannel{name: "ok"},
		fakeChannel{name: "skip", err: ErrNoAddress},
		fakeChannel{name: "broken", err: errors.New("boom")},
	}}
	if err := d.Send(context.Background(), Message{Body: "hi"}); err != nil {
		t.Fatalf("one channel succeeded, Send must not fail: %v", err)
	}
	if len(rec.got) != 2 {
		t.Fatalf("want 2 recorded attempts (ok + broken), got %d", len(rec.got))
	}
	if rec.got[0].Status != "sent" || rec.got[1].Status != "failed" {
		t.Errorf("statuses = %s, %s", rec.got[0].Status, rec.got[1].Status)
	}
}

func TestDispatcherFailsWhenAllFail(t *testing.T) {
	d := &Dispatcher{channels: []Channel{fakeChannel{name: "x", err: errors.New("down")}}}
	if err := d.Send(context.Background(), Message{}); err == nil {
		t.Fatal("expected error when every channel fails")
	}
}
