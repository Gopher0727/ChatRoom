package snowflake

import (
	"sync"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_SnowflakeIDUniqueness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("all generated IDs are unique", prop.ForAll(
		func(count int) bool {
			// Create a generator with random configuration
			gen, err := NewGenerator(Config{
				DatacenterID: 1,
				WorkerID:     1,
			})
			if err != nil {
				return false
			}

			// Generate multiple IDs
			ids := make(map[int64]bool)
			for range count {
				id, err := gen.NextID()
				if err != nil {
					return false
				}

				// Check for duplicates
				if ids[id] {
					return false
				}
				ids[id] = true
			}

			return len(ids) == count
		},
		gen.IntRange(100, 1000),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProperty_SnowflakeIDUniqueness_Concurrent(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("IDs generated concurrently are unique", prop.ForAll(
		func(goroutines int, idsPerGoroutine int) bool {
			gen, err := NewGenerator(Config{
				DatacenterID: 1,
				WorkerID:     1,
			})
			if err != nil {
				return false
			}

			// Generate IDs concurrently
			idChan := make(chan int64, goroutines*idsPerGoroutine)

			var wg sync.WaitGroup
			for range goroutines {
				wg.Go(func() {
					for range idsPerGoroutine {
						id, err := gen.NextID()
						if err != nil {
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
					return false
				}
				ids[id] = true
			}

			expectedCount := goroutines * idsPerGoroutine
			return len(ids) == expectedCount
		},
		gen.IntRange(5, 20),
		gen.IntRange(50, 200),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProperty_SnowflakeIDUniqueness_MultipleGenerators(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("IDs from different generators with different worker IDs are unique", prop.ForAll(
		func(workerID1 int64, workerID2 int64, count int) bool {
			// Ensure different worker IDs
			if workerID1 == workerID2 {
				return true // Skip this case
			}

			gen1, err := NewGenerator(Config{
				DatacenterID: 1,
				WorkerID:     workerID1,
			})
			if err != nil {
				return false
			}

			gen2, err := NewGenerator(Config{
				DatacenterID: 1,
				WorkerID:     workerID2,
			})
			if err != nil {
				return false
			}

			// Generate IDs from both generators
			ids := make(map[int64]bool)
			for range count {
				id1, err := gen1.NextID()
				if err != nil {
					return false
				}
				if ids[id1] {
					return false
				}
				ids[id1] = true

				id2, err := gen2.NextID()
				if err != nil {
					return false
				}
				if ids[id2] {
					return false
				}
				ids[id2] = true
			}

			return len(ids) == count*2
		},
		gen.Int64Range(0, 1023),
		gen.Int64Range(0, 1023),
		gen.IntRange(50, 200),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProperty_SnowflakeIDTimeOrdering(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("IDs generated in sequence have monotonically increasing timestamps", prop.ForAll(
		func(count int) bool {
			gen, err := NewGenerator(Config{
				DatacenterID: 1,
				WorkerID:     1,
			})
			if err != nil {
				return false
			}

			var lastTimestamp int64
			for i := range count {
				id, err := gen.NextID()
				if err != nil {
					return false
				}

				timestamp := gen.GetTimestamp(id)
				if i > 0 && timestamp < lastTimestamp {
					return false
				}
				lastTimestamp = timestamp
			}
			return true
		},
		gen.IntRange(100, 1000),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProperty_SnowflakeIDMonotonicIncreasing(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("IDs generated in sequence are monotonically increasing", prop.ForAll(
		func(count int) bool {
			gen, err := NewGenerator(Config{
				DatacenterID: 1,
				WorkerID:     1,
			})
			if err != nil {
				return false
			}

			var lastID int64
			for i := range count {
				id, err := gen.NextID()
				if err != nil {
					return false
				}

				if i > 0 && id <= lastID {
					return false
				}
				lastID = id
			}
			return true
		},
		gen.IntRange(100, 1000),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProperty_SnowflakeIDBitConfiguration(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("generated IDs respect custom bit allocation", prop.ForAll(
		func(workerIDBits uint8, sequenceBits uint8, datacenterBits uint8, workerID int64, datacenterID int64) bool {
			// Ensure valid bit allocation (total must not exceed 22)
			if workerIDBits+sequenceBits+datacenterBits > 22 || workerIDBits == 0 || sequenceBits == 0 {
				return true // Skip invalid configurations
			}

			// Calculate max values for the given bit sizes
			maxWorkerID := int64(1<<workerIDBits) - 1
			maxDatacenterID := int64(1<<datacenterBits) - 1

			// Ensure IDs are within valid range
			if workerID < 0 || workerID > maxWorkerID {
				return true // Skip invalid worker IDs
			}
			if datacenterBits > 0 && (datacenterID < 0 || datacenterID > maxDatacenterID) {
				return true // Skip invalid datacenter IDs
			}
			if datacenterBits == 0 {
				datacenterID = 0
			}

			gen, err := NewGenerator(Config{
				DatacenterID:   datacenterID,
				WorkerID:       workerID,
				WorkerIDBits:   workerIDBits,
				SequenceBits:   sequenceBits,
				DatacenterBits: datacenterBits,
			})
			if err != nil {
				return false
			}

			// Generate an ID and parse it
			id, err := gen.NextID()
			if err != nil {
				return false
			}

			parsedWorkerID := gen.GetWorkerID(id)
			parsedDatacenterID := gen.GetDatacenterID(id)
			parsedSequence := gen.GetSequence(id)

			// Verify the parsed values match the configuration
			if parsedWorkerID != workerID {
				return false
			}
			if parsedDatacenterID != datacenterID {
				return false
			}

			// Verify sequence is within valid range
			maxSequence := int64(1<<sequenceBits) - 1
			if parsedSequence < 0 || parsedSequence > maxSequence {
				return false
			}

			return true
		},
		gen.UInt8Range(1, 15),
		gen.UInt8Range(1, 15),
		gen.UInt8Range(0, 10),
		gen.Int64Range(0, 1023),
		gen.Int64Range(0, 1023),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

func TestProperty_SnowflakeIDBitConfiguration_RoundTrip(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("parsing an ID returns the original configuration values", prop.ForAll(
		func(workerIDBits uint8, sequenceBits uint8, datacenterBits uint8) bool {
			// Ensure valid bit allocation
			if workerIDBits+sequenceBits+datacenterBits > 22 || workerIDBits == 0 || sequenceBits == 0 {
				return true // Skip invalid configurations
			}

			// Use maximum valid values for this configuration
			maxWorkerID := int64(1<<workerIDBits) - 1
			maxDatacenterID := int64(1<<datacenterBits) - 1
			if datacenterBits == 0 {
				maxDatacenterID = 0
			}

			gen, err := NewGenerator(Config{
				DatacenterID:   maxDatacenterID,
				WorkerID:       maxWorkerID,
				WorkerIDBits:   workerIDBits,
				SequenceBits:   sequenceBits,
				DatacenterBits: datacenterBits,
			})
			if err != nil {
				return false
			}

			// Generate multiple IDs to test different sequences
			for range 10 {
				id, err := gen.NextID()
				if err != nil {
					return false
				}

				// Parse and verify
				_, parsedDatacenterID, parsedWorkerID, parsedSequence := gen.Parse(id)

				if parsedWorkerID != maxWorkerID {
					return false
				}
				if parsedDatacenterID != maxDatacenterID {
					return false
				}

				// Verify sequence is within valid range
				maxSequence := int64(1<<sequenceBits) - 1
				if parsedSequence < 0 || parsedSequence > maxSequence {
					return false
				}
			}

			return true
		},
		gen.UInt8Range(1, 15),
		gen.UInt8Range(1, 15),
		gen.UInt8Range(0, 10),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
