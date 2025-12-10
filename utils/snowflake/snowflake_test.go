package snowflake

import (
	"sync"
	"testing"
	"time"
)

func TestNewGenerator(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorType   error
	}{
		{
			name: "valid default configuration",
			config: Config{
				DatacenterID: 1,
				WorkerID:     1,
			},
			expectError: false,
		},
		{
			name: "valid custom configuration",
			config: Config{
				DatacenterID:   1,
				WorkerID:       5,
				WorkerIDBits:   10,
				SequenceBits:   12,
				DatacenterBits: 0,
			},
			expectError: false,
		},
		{
			name: "invalid worker ID - too large",
			config: Config{
				DatacenterID: 1,
				WorkerID:     1024, // Max is 1023 for 10 bits
				WorkerIDBits: 10,
				SequenceBits: 12,
			},
			expectError: true,
			errorType:   ErrInvalidWorkerID,
		},
		{
			name: "invalid worker ID - negative",
			config: Config{
				DatacenterID: 1,
				WorkerID:     -1,
				WorkerIDBits: 10,
				SequenceBits: 12,
			},
			expectError: true,
			errorType:   ErrInvalidWorkerID,
		},
		{
			name: "invalid datacenter ID - too large",
			config: Config{
				DatacenterID:   8, // Max is 7 for 3 bits
				WorkerID:       1,
				WorkerIDBits:   5,
				SequenceBits:   12,
				DatacenterBits: 3,
			},
			expectError: true,
			errorType:   ErrInvalidDatacenterID,
		},
		{
			name: "invalid bit allocation - exceeds 22 bits",
			config: Config{
				DatacenterID:   1,
				WorkerID:       1,
				WorkerIDBits:   15,
				SequenceBits:   15,
				DatacenterBits: 0,
			},
			expectError: true,
			errorType:   ErrInvalidBitAllocation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := NewGenerator(tt.config)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if tt.errorType != nil && err != tt.errorType {
					t.Errorf("expected error %v, got %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if gen == nil {
					t.Errorf("expected generator but got nil")
				}
			}
		})
	}
}

func TestNextID_BasicFunctionality(t *testing.T) {
	gen, err := NewGenerator(Config{
		DatacenterID: 1,
		WorkerID:     1,
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	id, err := gen.NextID()
	if err != nil {
		t.Fatalf("failed to generate ID: %v", err)
	}

	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}
}

func TestNextID_Uniqueness(t *testing.T) {
	gen, err := NewGenerator(Config{
		DatacenterID: 1,
		WorkerID:     1,
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	// Generate multiple IDs and check for uniqueness
	ids := make(map[int64]bool)
	count := 10000

	for range count {
		id, err := gen.NextID()
		if err != nil {
			t.Fatalf("failed to generate ID: %v", err)
		}

		if ids[id] {
			t.Errorf("duplicate ID generated: %d", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("expected %d unique IDs, got %d", count, len(ids))
	}
}

func TestNextID_ThreadSafety(t *testing.T) {
	gen, err := NewGenerator(Config{
		DatacenterID: 1,
		WorkerID:     1,
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	// Generate IDs concurrently
	idChan := make(chan int64, 1000)
	goroutines := 10
	idsPerGoroutine := 100

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for j := 0; j < idsPerGoroutine; j++ {
				id, err := gen.NextID()
				if err != nil {
					t.Errorf("failed to generate ID: %v", err)
					return
				}
				idChan <- id
			}
		})
	}
	wg.Wait()

	close(idChan)

	// Check for uniqueness
	ids := make(map[int64]bool)
	for id := range idChan {
		if ids[id] {
			t.Errorf("duplicate ID generated in concurrent test: %d", id)
		}
		ids[id] = true
	}

	expectedCount := goroutines * idsPerGoroutine
	if len(ids) != expectedCount {
		t.Errorf("expected %d unique IDs, got %d", expectedCount, len(ids))
	}
}

func TestNextID_MonotonicIncreasing(t *testing.T) {
	gen, err := NewGenerator(Config{
		DatacenterID: 1,
		WorkerID:     1,
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	var lastID int64
	for i := range 1000 {
		id, err := gen.NextID()
		if err != nil {
			t.Fatalf("failed to generate ID: %v", err)
		}

		if i > 0 && id <= lastID {
			t.Errorf("IDs not monotonically increasing: %d <= %d", id, lastID)
		}
		lastID = id
	}
}

func TestParse(t *testing.T) {
	gen, err := NewGenerator(Config{
		DatacenterID:   3,
		WorkerID:       5,
		WorkerIDBits:   10,
		SequenceBits:   12,
		DatacenterBits: 0,
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	id, err := gen.NextID()
	if err != nil {
		t.Fatalf("failed to generate ID: %v", err)
	}

	timestamp, datacenterID, workerID, sequence := gen.Parse(id)

	// Verify worker ID
	if workerID != 5 {
		t.Errorf("expected worker ID 5, got %d", workerID)
	}

	// Verify datacenter ID
	if datacenterID != 0 { // datacenterBits is 0, so should be 0
		t.Errorf("expected datacenter ID 0, got %d", datacenterID)
	}

	// Verify timestamp is reasonable (within last second)
	now := time.Now().UnixNano() / int64(time.Millisecond)
	if timestamp < now-1000 || timestamp > now+1000 {
		t.Errorf("timestamp out of reasonable range: %d (now: %d)", timestamp, now)
	}

	// Verify sequence is within valid range
	maxSequence := int64(1<<gen.sequenceBits) - 1
	if sequence < 0 || sequence > maxSequence {
		t.Errorf("sequence out of range: %d (max: %d)", sequence, maxSequence)
	}
}

func TestGetters(t *testing.T) {
	gen, err := NewGenerator(Config{
		DatacenterID:   2,
		WorkerID:       7,
		WorkerIDBits:   5,
		SequenceBits:   12,
		DatacenterBits: 5,
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	id, err := gen.NextID()
	if err != nil {
		t.Fatalf("failed to generate ID: %v", err)
	}

	// Test individual getters
	workerID := gen.GetWorkerID(id)
	if workerID != 7 {
		t.Errorf("expected worker ID 7, got %d", workerID)
	}

	datacenterID := gen.GetDatacenterID(id)
	if datacenterID != 2 {
		t.Errorf("expected datacenter ID 2, got %d", datacenterID)
	}

	sequence := gen.GetSequence(id)
	maxSequence := int64(1<<gen.sequenceBits) - 1
	if sequence < 0 || sequence > maxSequence {
		t.Errorf("sequence out of range: %d (max: %d)", sequence, maxSequence)
	}

	timestamp := gen.GetTimestamp(id)
	now := time.Now().UnixNano() / int64(time.Millisecond)
	if timestamp < now-1000 || timestamp > now+1000 {
		t.Errorf("timestamp out of reasonable range: %d (now: %d)", timestamp, now)
	}
}

func TestCustomBitAllocation(t *testing.T) {
	tests := []struct {
		name           string
		workerIDBits   uint8
		sequenceBits   uint8
		datacenterBits uint8
		workerID       int64
		datacenterID   int64
	}{
		{
			name:           "8-bit worker, 14-bit sequence",
			workerIDBits:   8,
			sequenceBits:   14,
			datacenterBits: 0,
			workerID:       255, // max for 8 bits
			datacenterID:   0,
		},
		{
			name:           "5-bit datacenter, 5-bit worker, 12-bit sequence",
			workerIDBits:   5,
			sequenceBits:   12,
			datacenterBits: 5,
			workerID:       31, // max for 5 bits
			datacenterID:   31, // max for 5 bits
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := NewGenerator(Config{
				DatacenterID:   tt.datacenterID,
				WorkerID:       tt.workerID,
				WorkerIDBits:   tt.workerIDBits,
				SequenceBits:   tt.sequenceBits,
				DatacenterBits: tt.datacenterBits,
			})
			if err != nil {
				t.Fatalf("failed to create generator: %v", err)
			}

			id, err := gen.NextID()
			if err != nil {
				t.Fatalf("failed to generate ID: %v", err)
			}

			// Verify the parsed values match configuration
			_, datacenterID, workerID, _ := gen.Parse(id)
			if workerID != tt.workerID {
				t.Errorf("expected worker ID %d, got %d", tt.workerID, workerID)
			}
			if datacenterID != tt.datacenterID {
				t.Errorf("expected datacenter ID %d, got %d", tt.datacenterID, datacenterID)
			}
		})
	}
}

func TestSequenceOverflow(t *testing.T) {
	gen, err := NewGenerator(Config{
		DatacenterID: 1,
		WorkerID:     1,
		SequenceBits: 4, // Small sequence for easier overflow testing (max 15)
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	// Generate enough IDs to potentially overflow the sequence
	// This should trigger waiting for the next millisecond
	var lastTimestamp int64
	for i := range 20 {
		id, err := gen.NextID()
		if err != nil {
			t.Fatalf("failed to generate ID: %v", err)
		}

		timestamp := gen.GetTimestamp(id)
		if i > 0 && timestamp < lastTimestamp {
			t.Errorf("timestamp went backwards: %d < %d", timestamp, lastTimestamp)
		}
		lastTimestamp = timestamp
	}
}

func TestClockMovedBackwards(t *testing.T) {
	gen, err := NewGenerator(Config{
		DatacenterID: 1,
		WorkerID:     1,
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	// Generate an ID to set lastTimestamp
	_, err = gen.NextID()
	if err != nil {
		t.Fatalf("failed to generate initial ID: %v", err)
	}

	// Manually set lastTimestamp to a future value to simulate clock moving backwards
	gen.mu.Lock()
	gen.lastTimestamp = gen.currentTimestamp() + 10000 // 10 seconds in the future
	gen.mu.Unlock()

	// Try to generate an ID - should return error
	_, err = gen.NextID()
	if err != ErrClockMovedBackwards {
		t.Errorf("expected ErrClockMovedBackwards, got %v", err)
	}
}

func BenchmarkNextID(b *testing.B) {
	gen, err := NewGenerator(Config{
		DatacenterID: 1,
		WorkerID:     1,
	})
	if err != nil {
		b.Fatalf("failed to create generator: %v", err)
	}

	for b.Loop() {
		_, err := gen.NextID()
		if err != nil {
			b.Fatalf("failed to generate ID: %v", err)
		}
	}
}

func BenchmarkNextID_Parallel(b *testing.B) {
	gen, err := NewGenerator(Config{
		DatacenterID: 1,
		WorkerID:     1,
	})
	if err != nil {
		b.Fatalf("failed to create generator: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := gen.NextID()
			if err != nil {
				b.Fatalf("failed to generate ID: %v", err)
			}
		}
	})
}
