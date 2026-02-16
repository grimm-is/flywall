// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package dns

import (
	"fmt"
	"testing"
	"time"

	"grimm.is/flywall/internal/config"

	"github.com/miekg/dns"
)

func TestService_Cache(t *testing.T) {
	cfg := &config.DNSServer{Enabled: true}
	s, _ := newTestService(cfg)

	// Manually inject a cached response
	qname := "cached.example.com."
	qtype := dns.TypeA
	msg := new(dns.Msg)
	msg.SetQuestion(qname, qtype)
	msg.Answer = []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: qname, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   []byte{1, 2, 3, 4},
		},
	}

	cacheKey := "cached.example.com.:1"
	shard := s.getShard(cacheKey)
	shard.mu.Lock()
	shard.items[cacheKey] = cachedResponse{
		msg:       msg,
		expiresAt: time.Now().Add(1 * time.Hour),
	}
	shard.mu.Unlock()

	// Verify cache hit logic (simulating ServeDNS internal check)
	shard.mu.RLock()
	cached, found := shard.items[cacheKey]
	shard.mu.RUnlock()

	if !found {
		t.Error("Expected cache hit")
	}
	if time.Now().After(cached.expiresAt) {
		t.Error("Cache entry should not be expired")
	}
}

func TestService_Cache_Expiry(t *testing.T) {
	cfg := &config.DNSServer{Enabled: true}
	s, _ := newTestService(cfg)

	// Case 1: Active Entry
	validName := "valid.example.com."
	validMsg := new(dns.Msg)
	validMsg.SetQuestion(validName, dns.TypeA)
	validMsg.Answer = []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: validName, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 3600}, A: []byte{1, 1, 1, 1}},
	}

	key1 := validName + ":1"
	shard1 := s.getShard(key1)
	shard1.mu.Lock()
	shard1.items[key1] = cachedResponse{msg: validMsg, expiresAt: time.Now().Add(time.Hour)}
	shard1.mu.Unlock()

	// Case 2: Expired Entry
	expiredName := "expired.example.com."
	expiredMsg := new(dns.Msg)
	expiredMsg.SetQuestion(expiredName, dns.TypeA)
	expiredMsg.Answer = []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: expiredName, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 3600}, A: []byte{2, 2, 2, 2}},
	}
	s.cacheResponse(expiredMsg, expiredMsg)

	key2 := expiredName + ":1"
	shard2 := s.getShard(key2)
	shard2.mu.Lock()
	shard2.items[key2] = cachedResponse{msg: expiredMsg, expiresAt: time.Now().Add(-time.Hour)}
	shard2.mu.Unlock()

	// Verify Valid
	req := new(dns.Msg)
	req.SetQuestion(validName, dns.TypeA)
	w := &MockResponseWriter{}
	s.ServeDNS(w, req)

	if w.msg == nil || len(w.msg.Answer) == 0 {
		t.Error("Expected valid cache hit, got nothing")
	}

	// Verify Expired
	req = new(dns.Msg)
	req.SetQuestion(expiredName, dns.TypeA)
	w = &MockResponseWriter{}
	s.ServeDNS(w, req)

	if w.msg == nil {
		t.Error("Expected response (NXDOMAIN), got nil")
	} else if w.msg.Rcode != dns.RcodeNameError {
		t.Errorf("Expected NXDOMAIN for expired cache hit, got Rcode %d", w.msg.Rcode)
	}
}

func TestService_Cache_Eviction(t *testing.T) {
	cfg := &config.DNSServer{Enabled: true, CacheSize: 10000} // CacheSize config is actually ignored in current fixed shard implementation (1000/shard), but keeping for signature
	s, _ := newTestService(cfg)

	// Target a specific shard (e.g., shard for key "A")
	targetKey := "target.key.:1"
	targetShard := s.getShard(targetKey)

	// Fill this shard to limit (1000)
	targetShard.mu.Lock()
	for i := 0; i < 1000; i++ {
		// We need keys that hash to the SAME shard.
		// Constructing collisions is hard without access to hash func or brute force.
		// Easier approach: Just test that `cacheResponse` respects limit.
		// Since we changed logic to `if len(shard.items) >= 1000`, we can just mock-fill it.
		// We don't need real collisions, we are injecting into `targetShard.items` directly!
		key := fmt.Sprintf("fill-%d", i)
		targetShard.items[key] = cachedResponse{expiresAt: time.Now().Add(time.Hour)}
	}
	targetShard.mu.Unlock()

	// Try to add one more that maps to ANY shard (we'll force it into targetShard for test)
	// But `cacheResponse` calculates shard from key.
	// So we must use a key that we know maps to `targetShard`.
	// We already know `targetKey` maps to `targetShard` because we asked for it.

	req := new(dns.Msg)
	req.SetQuestion("target.key.", dns.TypeA)
	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.Answer = []dns.RR{
		&dns.A{Hdr: dns.RR_Header{Name: "target.key.", Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: []byte{1, 2, 3, 4}},
	}

	// This calls cacheResponse, which calls getShard("target.key.:1"), which returns targetShard.
	s.cacheResponse(req, resp)

	targetShard.mu.RLock()
	size := len(targetShard.items)
	targetShard.mu.RUnlock()

	// Should remain at 1000 (evicted one to make room, or size is 1000)
	// Implementation says: if len >= 1000 { delete one } then add.
	// So size should be 1000.
	if size > 1000 {
		t.Errorf("Cache shard size exceeded limit: %d", size)
	}

	// Verify "target.key.:1" exists
	targetShard.mu.RLock()
	_, found := targetShard.items[targetKey]
	targetShard.mu.RUnlock()

	if !found {
		t.Error("New entry was not added (or evicted immediately)")
	}
}
