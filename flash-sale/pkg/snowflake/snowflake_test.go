package snowflake

import (
	"sync"
	"testing"
)

func TestNewNode(t *testing.T) {
	_, err := NewNode(1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	_, err = NewNode(-1)
	if err == nil {
		t.Fatal("expected error for negative node ID")
	}

	_, err = NewNode(nodeMax + 1)
	if err == nil {
		t.Fatal("expected error for node ID exceeding max")
	}
}

func TestGenerateUnique(t *testing.T) {
	node, _ := NewNode(1)
	ids := make(map[int64]bool)

	for i := 0; i < 10000; i++ {
		id := node.Generate()
		if ids[id] {
			t.Fatalf("duplicate ID generated: %d", id)
		}
		ids[id] = true
	}
}

func TestGenerateConcurrent(t *testing.T) {
	node, _ := NewNode(1)
	var mu sync.Mutex
	ids := make(map[int64]bool)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				id := node.Generate()
				mu.Lock()
				if ids[id] {
					t.Errorf("duplicate ID: %d", id)
				}
				ids[id] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
}
