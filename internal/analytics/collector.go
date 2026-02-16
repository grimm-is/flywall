// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package analytics

import (
	"sync"
	"time"
)

// Collector handles in-memory aggregation of packets into time-bucketed flow summaries
type Collector struct {
	mu      sync.Mutex
	buckets map[key]*Summary
	store   *Store
	window  time.Duration
}

// Store returns the underlying analytics store
func (c *Collector) Store() *Store {
	return c.store
}

type key struct {
	bucket  int64
	srcMAC  string
	srcIP   string
	dstIP   string
	dstPort int
	proto   string
}

// NewCollector creates a new analytics collector
func NewCollector(store *Store, bucketWindow time.Duration) *Collector {
	if bucketWindow == 0 {
		bucketWindow = 5 * time.Minute
	}
	return &Collector{
		buckets: make(map[key]*Summary),
		store:   store,
		window:  bucketWindow,
	}
}

// IngestPacket records packet data into the current time bucket
func (c *Collector) IngestPacket(pkt Summary) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Calculate bucket start time
	ts := pkt.BucketTime.Unix()
	bucketStart := ts - (ts % int64(c.window.Seconds()))

	k := key{
		bucket:  bucketStart,
		srcMAC:  pkt.SrcMAC,
		srcIP:   pkt.SrcIP,
		dstIP:   pkt.DstIP,
		dstPort: pkt.DstPort,
		proto:   pkt.Protocol,
	}

	s, exists := c.buckets[k]
	if !exists {
		s = &Summary{
			BucketTime: time.Unix(bucketStart, 0),
			SrcMAC:     pkt.SrcMAC,
			SrcIP:      pkt.SrcIP,
			DstIP:      pkt.DstIP,
			DstPort:    pkt.DstPort,
			Protocol:   pkt.Protocol,
		}
		c.buckets[k] = s
	}

	s.Bytes += pkt.Bytes
	s.Packets += pkt.Packets
	if pkt.Class != "" {
		s.Class = pkt.Class
	}
}

// Flush persists all currently aggregated buckets to the store and clears the memory
func (c *Collector) Flush() error {
	c.mu.Lock()
	toFlush := make([]Summary, 0, len(c.buckets))
	for _, s := range c.buckets {
		toFlush = append(toFlush, *s)
	}
	c.buckets = make(map[key]*Summary) // Clear map
	c.mu.Unlock()

	if len(toFlush) == 0 {
		return nil
	}

	return c.store.RecordSummaries(toFlush)
}

// StartBackgroundFlush starts a routine that flushes data to the store at fixed intervals
func (c *Collector) StartBackgroundFlush(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			_ = c.Flush()
		}
	}()
}
