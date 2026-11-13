package bot

import (
	"errors"
	"testing"
	"time"
)

type fakeConn struct {
	id    int
	ready bool
}

type fakeDialer struct {
	readyOnAttempt int
	opened         []*fakeConn
	discarded      []*fakeConn
}

func (d *fakeDialer) open() (*fakeConn, error) {
	conn := &fakeConn{id: len(d.opened) + 1}
	conn.ready = conn.id >= d.readyOnAttempt
	d.opened = append(d.opened, conn)
	return conn, nil
}

func (d *fakeDialer) discard(conn *fakeConn) {
	d.discarded = append(d.discarded, conn)
}

func isReady(conn *fakeConn) bool { return conn.ready }

func TestOpenUntilReadyRetriesConnectionThatNeverGetsReady(t *testing.T) {
	dialer := &fakeDialer{readyOnAttempt: 2}

	conn, err := openUntilReady(3, 20*time.Millisecond, dialer.open, isReady, dialer.discard)
	if err != nil {
		t.Fatalf("openUntilReady: %v", err)
	}
	if conn.id != 2 {
		t.Fatalf("got connection %d, want 2", conn.id)
	}
	if len(dialer.discarded) != 1 || dialer.discarded[0].id != 1 {
		t.Fatalf("discarded %v, want only connection 1", dialer.discarded)
	}
}

func TestOpenUntilReadyGivesUpAfterAllAttempts(t *testing.T) {
	dialer := &fakeDialer{readyOnAttempt: 99}

	if _, err := openUntilReady(3, 10*time.Millisecond, dialer.open, isReady, dialer.discard); err == nil {
		t.Fatal("expected error when no attempt gets ready")
	}
	if len(dialer.opened) != 3 || len(dialer.discarded) != 3 {
		t.Fatalf("opened %d, discarded %d, want 3 and 3", len(dialer.opened), len(dialer.discarded))
	}
}

func TestOpenUntilReadyStopsOnOpenError(t *testing.T) {
	openErr := errors.New("boom")
	open := func() (*fakeConn, error) { return nil, openErr }

	_, err := openUntilReady(3, 10*time.Millisecond, open, isReady, func(*fakeConn) {})
	if !errors.Is(err, openErr) {
		t.Fatalf("err = %v, want %v", err, openErr)
	}
}
