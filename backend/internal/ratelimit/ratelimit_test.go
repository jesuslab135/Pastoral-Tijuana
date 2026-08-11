package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestLimit(t *testing.T) {
	l := New(3, time.Hour)
	for i := 0; i < 3; i++ {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Error("4th request in window must be denied")
	}
	if !l.Allow("5.6.7.8") {
		t.Error("different key must not be affected")
	}
}

func TestWindowResets(t *testing.T) {
	l := New(1, 30*time.Millisecond)
	if !l.Allow("k") {
		t.Fatal("first must pass")
	}
	if l.Allow("k") {
		t.Fatal("second must be denied")
	}
	time.Sleep(40 * time.Millisecond)
	if !l.Allow("k") {
		t.Error("after the window the key must be allowed again")
	}
}

func TestConcurrentAllowIsRaceFree(t *testing.T) {
	l := New(100, time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Allow("shared")
		}()
	}
	wg.Wait()
}
