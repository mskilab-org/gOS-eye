package server

import (
	"sync"
	"testing"
)

func TestNewBroker_ReturnsNonNil(t *testing.T) {
	b := NewBroker()
	if b == nil {
		t.Fatal("NewBroker() returned nil")
	}
}

func TestNewBroker_SubscribersMapInitialized(t *testing.T) {
	b := NewBroker()
	if b.subscribers == nil {
		t.Fatal("subscribers map is nil; expected initialized empty map")
	}
}

func TestNewBroker_SubscribersMapEmpty(t *testing.T) {
	b := NewBroker()
	if len(b.subscribers) != 0 {
		t.Fatalf("subscribers map has %d entries; expected 0", len(b.subscribers))
	}
}

func TestNewBroker_IndependentInstances(t *testing.T) {
	b1 := NewBroker()
	b2 := NewBroker()
	if b1 == b2 {
		t.Fatal("two NewBroker() calls returned the same pointer")
	}
}

// ---- Publish tests ----

func TestPublish_NoSubscribers(t *testing.T) {
	b := NewBroker()
	// Should not panic with zero subscribers
	b.Publish("hello")
}

func TestPublish_SingleSubscriber(t *testing.T) {
	b := NewBroker()
	ch := make(chan string, 1)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	b.Publish("hello")

	select {
	case got := <-ch:
		if got != "hello" {
			t.Fatalf("got %q, want %q", got, "hello")
		}
	default:
		t.Fatal("expected message on channel, got nothing")
	}
}

func TestPublish_MultipleSubscribers(t *testing.T) {
	b := NewBroker()
	ch1 := make(chan string, 1)
	ch2 := make(chan string, 1)
	ch3 := make(chan string, 1)
	b.mu.Lock()
	b.subscribers[ch1] = struct{}{}
	b.subscribers[ch2] = struct{}{}
	b.subscribers[ch3] = struct{}{}
	b.mu.Unlock()

	b.Publish("update")

	for i, ch := range []chan string{ch1, ch2, ch3} {
		select {
		case got := <-ch:
			if got != "update" {
				t.Fatalf("subscriber %d: got %q, want %q", i, got, "update")
			}
		default:
			t.Fatalf("subscriber %d: expected message, got nothing", i)
		}
	}
}

func TestPublish_SkipsFullChannel(t *testing.T) {
	b := NewBroker()
	full := make(chan string, 1)
	full <- "blocking" // fill the buffer
	healthy := make(chan string, 1)
	b.mu.Lock()
	b.subscribers[full] = struct{}{}
	b.subscribers[healthy] = struct{}{}
	b.mu.Unlock()

	b.Publish("new-data")

	// healthy subscriber should have received the message
	select {
	case got := <-healthy:
		if got != "new-data" {
			t.Fatalf("healthy: got %q, want %q", got, "new-data")
		}
	default:
		t.Fatal("healthy subscriber did not receive message")
	}

	// full subscriber should still have its original blocking message, not the new one
	select {
	case got := <-full:
		if got != "blocking" {
			t.Fatalf("full: got %q, want %q (original)", got, "blocking")
		}
	default:
		t.Fatal("full subscriber lost its original message")
	}
}

func TestPublish_EmptyString(t *testing.T) {
	b := NewBroker()
	ch := make(chan string, 1)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	b.Publish("")

	select {
	case got := <-ch:
		if got != "" {
			t.Fatalf("got %q, want empty string", got)
		}
	default:
		t.Fatal("expected empty string on channel, got nothing")
	}
}

func TestPublish_ConcurrentSafe(t *testing.T) {
	b := NewBroker()
	ch := make(chan string, 100)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Publish("concurrent")
		}()
	}
	wg.Wait()

	count := len(ch)
	if count != 50 {
		t.Fatalf("got %d messages, want 50", count)
	}
}

// ---- Subscribe tests ----

func TestSubscribe_ReturnsNonNilChannel(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	if ch == nil {
		t.Fatal("Subscribe() returned nil channel")
	}
}

func TestSubscribe_ChannelIsBuffered(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	if cap(ch) != 16 {
		t.Fatalf("Subscribe() channel capacity = %d; want 16", cap(ch))
	}
}

func TestSubscribe_AddsToSubscribersMap(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	b.mu.RLock()
	_, ok := b.subscribers[ch]
	n := len(b.subscribers)
	b.mu.RUnlock()
	if !ok {
		t.Fatal("Subscribe() channel not found in subscribers map")
	}
	if n != 1 {
		t.Fatalf("subscribers map has %d entries; want 1", n)
	}
}

func TestSubscribe_MultipleSubscribers(t *testing.T) {
	b := NewBroker()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	ch3 := b.Subscribe()

	b.mu.RLock()
	n := len(b.subscribers)
	b.mu.RUnlock()

	if n != 3 {
		t.Fatalf("subscribers map has %d entries; want 3", n)
	}
	// Each channel must be distinct.
	if ch1 == ch2 || ch2 == ch3 || ch1 == ch3 {
		t.Fatal("Subscribe() returned duplicate channels")
	}
}

func TestSubscribe_ChannelWritable(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	// Should be able to write to the buffered channel without blocking.
	ch <- "hello"
	got := <-ch
	if got != "hello" {
		t.Fatalf("got %q from channel; want %q", got, "hello")
	}
}

func TestSubscribe_ConcurrentSafe(t *testing.T) {
	b := NewBroker()
	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	channels := make([]chan string, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			channels[idx] = b.Subscribe()
		}(i)
	}
	wg.Wait()

	b.mu.RLock()
	got := len(b.subscribers)
	b.mu.RUnlock()
	if got != n {
		t.Fatalf("subscribers map has %d entries after %d concurrent Subscribe() calls; want %d", got, n, n)
	}
}

func TestUnsubscribe_RemovesChannel(t *testing.T) {
	b := NewBroker()
	ch := make(chan string, 1)
	b.subscribers[ch] = struct{}{}

	b.Unsubscribe(ch)

	if _, ok := b.subscribers[ch]; ok {
		t.Fatal("channel still in subscribers map after Unsubscribe")
	}
}

func TestUnsubscribe_ClosesChannel(t *testing.T) {
	b := NewBroker()
	ch := make(chan string, 1)
	b.subscribers[ch] = struct{}{}

	b.Unsubscribe(ch)

	// Writing to a closed channel panics; reading yields zero value + ok=false
	_, ok := <-ch
	if ok {
		t.Fatal("channel not closed after Unsubscribe")
	}
}

func TestUnsubscribe_OnlyRemovesTargetChannel(t *testing.T) {
	b := NewBroker()
	ch1 := make(chan string, 1)
	ch2 := make(chan string, 1)
	b.subscribers[ch1] = struct{}{}
	b.subscribers[ch2] = struct{}{}

	b.Unsubscribe(ch1)

	if len(b.subscribers) != 1 {
		t.Fatalf("expected 1 subscriber remaining, got %d", len(b.subscribers))
	}
	if _, ok := b.subscribers[ch2]; !ok {
		t.Fatal("ch2 should still be in subscribers map")
	}
}

func TestUnsubscribe_NonexistentChannel_NoPanic(t *testing.T) {
	b := NewBroker()
	ch := make(chan string, 1)

	// Should not panic when unsubscribing a channel that was never subscribed
	b.Unsubscribe(ch)

	if len(b.subscribers) != 0 {
		t.Fatalf("expected 0 subscribers, got %d", len(b.subscribers))
	}
}
