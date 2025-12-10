package snowflake

import (
	"errors"
	"sync"
	"time"
)

const (
	// Epoch is the custom epoch (January 1, 2024 00:00:00 UTC)
	Epoch int64 = 1704067200000 // milliseconds

	// Default bit allocations
	DefaultWorkerIDBits uint8 = 10
	DefaultSequenceBits uint8 = 12
)

var (
	ErrInvalidWorkerID      = errors.New("worker ID exceeds maximum value")
	ErrInvalidDatacenterID  = errors.New("datacenter ID exceeds maximum value")
	ErrClockMovedBackwards  = errors.New("clock moved backwards")
	ErrInvalidBitAllocation = errors.New("invalid bit allocation: total bits must not exceed 22")
)

// Generator generates unique IDs using the Snowflake algorithm
type Generator struct {
	mu sync.Mutex

	// Configuration
	epoch          int64
	datacenterID   int64
	workerID       int64
	workerIDBits   uint8
	sequenceBits   uint8
	datacenterBits uint8

	// Bit masks and shifts
	workerIDShift     uint8
	datacenterIDShift uint8
	timestampShift    uint8
	sequenceMask      int64
	workerIDMask      int64
	datacenterIDMask  int64

	// State
	sequence      int64
	lastTimestamp int64
}

// Config holds the configuration for the Snowflake generator
type Config struct {
	Epoch          int64
	DatacenterID   int64
	WorkerID       int64
	WorkerIDBits   uint8
	SequenceBits   uint8
	DatacenterBits uint8
}

// NewGenerator creates a new Snowflake ID generator with the given configuration
func NewGenerator(config Config) (*Generator, error) {
	// Set defaults if not provided
	if config.WorkerIDBits == 0 {
		config.WorkerIDBits = DefaultWorkerIDBits
	}
	if config.SequenceBits == 0 {
		config.SequenceBits = DefaultSequenceBits
	}
	if config.DatacenterBits == 0 {
		config.DatacenterBits = 0 // Optional, can be 0
	}
	if config.Epoch == 0 {
		config.Epoch = Epoch
	}

	// Validate bit allocation (timestamp is 41 bits, total must be 63 bits)
	// 41 (timestamp) + datacenterBits + workerIDBits + sequenceBits = 63
	totalBits := config.DatacenterBits + config.WorkerIDBits + config.SequenceBits
	if totalBits > 22 {
		return nil, ErrInvalidBitAllocation
	}

	g := &Generator{
		epoch:          config.Epoch,
		datacenterID:   config.DatacenterID,
		workerID:       config.WorkerID,
		workerIDBits:   config.WorkerIDBits,
		sequenceBits:   config.SequenceBits,
		datacenterBits: config.DatacenterBits,
	}

	// Calculate bit shifts
	g.workerIDShift = g.sequenceBits
	g.datacenterIDShift = g.sequenceBits + g.workerIDBits
	g.timestampShift = g.sequenceBits + g.workerIDBits + g.datacenterBits

	// Calculate bit masks
	g.sequenceMask = -1 ^ (-1 << g.sequenceBits)
	g.workerIDMask = -1 ^ (-1 << g.workerIDBits)
	g.datacenterIDMask = -1 ^ (-1 << g.datacenterBits)

	// Validate IDs against masks
	if g.workerID > g.workerIDMask || g.workerID < 0 {
		return nil, ErrInvalidWorkerID
	}
	if g.datacenterBits > 0 {
		if g.datacenterID > g.datacenterIDMask || g.datacenterID < 0 {
			return nil, ErrInvalidDatacenterID
		}
	} else {
		// If datacenterBits is 0, datacenterID must be 0
		g.datacenterID = 0
	}

	return g, nil
}

// NextID generates the next unique ID
func (g *Generator) NextID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	timestamp := g.currentTimestamp()

	// Check for clock moving backwards
	if timestamp < g.lastTimestamp {
		return 0, ErrClockMovedBackwards
	}

	// If same millisecond, increment sequence
	if timestamp == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & g.sequenceMask
		// Sequence overflow - wait for next millisecond
		if g.sequence == 0 {
			timestamp = g.waitNextMillis(g.lastTimestamp)
		}
	} else {
		// New millisecond, reset sequence
		g.sequence = 0
	}

	g.lastTimestamp = timestamp

	// Construct the ID
	id := ((timestamp - g.epoch) << g.timestampShift) |
		(g.datacenterID << g.datacenterIDShift) |
		(g.workerID << g.workerIDShift) |
		g.sequence

	return id, nil
}

// currentTimestamp returns the current timestamp in milliseconds
func (g *Generator) currentTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// waitNextMillis waits until the next millisecond
func (g *Generator) waitNextMillis(lastTimestamp int64) int64 {
	timestamp := g.currentTimestamp()
	for timestamp <= lastTimestamp {
		timestamp = g.currentTimestamp()
	}
	return timestamp
}

// Parse extracts the components from a Snowflake ID
func (g *Generator) Parse(id int64) (timestamp int64, datacenterID int64, workerID int64, sequence int64) {
	sequence = id & g.sequenceMask
	workerID = (id >> g.workerIDShift) & g.workerIDMask
	datacenterID = (id >> g.datacenterIDShift) & g.datacenterIDMask
	timestamp = (id >> g.timestampShift) + g.epoch
	return
}

// GetTimestamp extracts just the timestamp from a Snowflake ID
func (g *Generator) GetTimestamp(id int64) int64 {
	return (id >> g.timestampShift) + g.epoch
}

// GetWorkerID extracts just the worker ID from a Snowflake ID
func (g *Generator) GetWorkerID(id int64) int64 {
	return (id >> g.workerIDShift) & g.workerIDMask
}

// GetDatacenterID extracts just the datacenter ID from a Snowflake ID
func (g *Generator) GetDatacenterID(id int64) int64 {
	return (id >> g.datacenterIDShift) & g.datacenterIDMask
}

// GetSequence extracts just the sequence from a Snowflake ID
func (g *Generator) GetSequence(id int64) int64 {
	return id & g.sequenceMask
}
