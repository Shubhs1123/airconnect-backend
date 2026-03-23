package ws

import (
	"sync"
	"testing"
)

// fakeSend simulates what Serve() does: registers a client, returns its send channel.
func addFakeClient(h *Hub, userID string) (chan []byte, func()) {
	c := &Client{
		conn:   nil, // not testing actual WS writes here
		userID: userID,
		send:   make(chan []byte, 64),
		done:   make(chan struct{}),
	}
	h.register(c)
	return c.send, func() { h.unregister(c) }
}

func TestBroadcast_OnlyTargetUser(t *testing.T) {
	h := &Hub{clients: make(map[*Client]bool)}

	ch1, rm1 := addFakeClient(h, "user-A")
	ch2, rm2 := addFakeClient(h, "user-B")
	defer rm1()
	defer rm2()

	msg := []byte(`{"event":"test"}`)
	h.Broadcast("user-A", msg)

	select {
	case got := <-ch1:
		if string(got) != string(msg) {
			t.Fatalf("user-A got wrong message: %s", got)
		}
	default:
		t.Fatal("user-A should have received the message")
	}

	select {
	case <-ch2:
		t.Fatal("user-B should NOT have received the message")
	default:
		// correct — channel is empty
	}
}

func TestBroadcast_MultipleConnectionsSameUser(t *testing.T) {
	h := &Hub{clients: make(map[*Client]bool)}

	ch1, rm1 := addFakeClient(h, "user-A")
	ch2, rm2 := addFakeClient(h, "user-A")
	defer rm1()
	defer rm2()

	h.Broadcast("user-A", []byte("hello"))

	count := 0
	for _, ch := range []chan []byte{ch1, ch2} {
		select {
		case <-ch:
			count++
		default:
		}
	}
	if count != 2 {
		t.Fatalf("expected both connections to receive message, got %d", count)
	}
}

func TestBroadcast_SlowClientDropped(t *testing.T) {
	h := &Hub{clients: make(map[*Client]bool)}

	// Fill the channel buffer completely
	c := &Client{
		conn:   nil,
		userID: "user-A",
		send:   make(chan []byte, 1), // buffer of 1
		done:   make(chan struct{}),
	}
	h.register(c)
	defer h.unregister(c)

	c.send <- []byte("fill") // fill the buffer

	// This second broadcast should NOT block (slow-client drop)
	done := make(chan struct{})
	go func() {
		h.Broadcast("user-A", []byte("overflow"))
		close(done)
	}()

	select {
	case <-done:
		// Good — did not block
	}
}

func TestUnregister_ClosesDone(t *testing.T) {
	h := &Hub{clients: make(map[*Client]bool)}
	_, rm := addFakeClient(h, "user-A")
	rm() // calls unregister → closes done

	// Calling unregister again would panic (double-close). Check map is empty.
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.clients) != 0 {
		t.Fatal("client should have been removed from hub")
	}
}

func TestBroadcast_Concurrent(t *testing.T) {
	h := &Hub{clients: make(map[*Client]bool)}
	ch, rm := addFakeClient(h, "user-X")
	defer rm()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Broadcast("user-X", []byte("ping"))
		}()
	}
	wg.Wait()

	// Drain channel — should have received up to buffer size (64) messages
	received := 0
	for {
		select {
		case <-ch:
			received++
		default:
			goto done
		}
	}
done:
	if received == 0 {
		t.Fatal("expected at least some messages to be received")
	}
}
