package snowflake_test

import (
	"fmt"
	"log"

	"github.com/Gopher0727/ChatRoom/utils/snowflake"
)

func ExampleGenerator_NextID() {
	// Create a new Snowflake ID generator with default configuration
	gen, err := snowflake.NewGenerator(snowflake.Config{
		DatacenterID: 1,
		WorkerID:     1,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Generate a new ID
	id, err := gen.NextID()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated ID: %d\n", id)
	// Output will vary, but ID will be a positive int64
}

func ExampleGenerator_Parse() {
	// Create a generator
	gen, err := snowflake.NewGenerator(snowflake.Config{
		DatacenterID:   2,
		WorkerID:       5,
		WorkerIDBits:   5,
		SequenceBits:   12,
		DatacenterBits: 5,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Generate an ID
	id, err := gen.NextID()
	if err != nil {
		log.Fatal(err)
	}

	// Parse the ID to extract its components
	timestamp, datacenterID, workerID, sequence := gen.Parse(id)

	fmt.Printf("Timestamp: %d\n", timestamp)
	fmt.Printf("Datacenter ID: %d\n", datacenterID)
	fmt.Printf("Worker ID: %d\n", workerID)
	fmt.Printf("Sequence: %d\n", sequence)
}

func ExampleNewGenerator_customBitAllocation() {
	// Create a generator with custom bit allocation
	// 8 bits for worker ID (max 255), 14 bits for sequence (max 16383)
	gen, err := snowflake.NewGenerator(snowflake.Config{
		DatacenterID:   0,
		WorkerID:       100,
		WorkerIDBits:   8,
		SequenceBits:   14,
		DatacenterBits: 0,
	})
	if err != nil {
		log.Fatal(err)
	}

	id, err := gen.NextID()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Generated ID with custom bit allocation: %d\n", id)
}
