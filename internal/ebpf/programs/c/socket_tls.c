#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_endian.h"
#include "common.h"

// BPF flags
#ifndef BPF_ANY
#define BPF_ANY 0
#endif

#define TLS_RECORD_HANDSHAKE 0x16
#define TLS_HANDSHAKE_CLIENT_HELLO 0x01
#define MAX_SNI_LEN 64

// Maps for tracking TLS handshakes
struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 65536);
	__type(key, struct tls_key);
	__type(value, struct tls_handshake_info);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} tls_handshakes SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(max_entries, 10);
	__type(key, __u32);
	__type(value, __u64);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} tls_stats SEC(".maps");

// Ring buffer for events
struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 1024 * 256);
} tls_events SEC(".maps");

// Statistics indices
enum {
	STAT_HANDSHAKES_OBSERVED = 0,
	STAT_CERTIFICATES_VALID = 1,
	STAT_CERTIFICATES_INVALID = 2,
	STAT_ERRORS = 3,
	STAT_MAX,
};

// Helper function to increment statistics
static __always_inline void increment_stat(__u32 stat_idx) {
	__u64 *count = bpf_map_lookup_elem(&tls_stats, &stat_idx);
	if (count) {
		__sync_fetch_and_add(count, 1);
	}
}

// Simple 128-bit hash for JA3-like fingerprinting in eBPF
static __always_inline void produce_ja3_hash(struct tls_handshake_info *info) {
	// JA3 is MD5(Version,Ciphers,Extensions,EllipticCurves,EllipticCurveFormats)
	// Since full MD5 is complex, we use a simple XOR/mix of the available fields
	// This will be replaced with a more robust hash if needed.
	info->ja3_hash[0] = info->version;
	info->ja3_hash[1] = info->cipher_suite;
	info->ja3_hash[2] = 0;
	info->ja3_hash[3] = 0;
	for (int i = 0; i < MAX_SNI_LEN; i++) {
		info->ja3_hash[2] ^= (info->sni[i] << (i % 24));
		info->ja3_hash[3] ^= (info->sni[i] << ((i + 13) % 24));
	}
}

// Send TLS event to userspace
static __always_inline void send_tls_event(struct __sk_buff *skb,
                                          const struct tls_key *key,
                                          const struct tls_handshake_info *info) {
	struct tls_event *event = bpf_ringbuf_reserve(&tls_events, sizeof(*event), 0);
	if (!event) {
		increment_stat(STAT_ERRORS);
		return;
	}

	event->timestamp = bpf_ktime_get_ns();
	event->pid = bpf_get_current_pid_tgid() >> 32;
	event->tid = (__u32)bpf_get_current_pid_tgid();
	event->src_ip = key->src_ip;
	event->dst_ip = key->dst_ip;
	event->src_port = key->src_port;
	event->dst_port = key->dst_port;
	event->version = info->version;
	event->cipher_suite = info->cipher_suite;
	event->packet_size = skb->len;
	__builtin_memcpy(event->sni, info->sni, MAX_SNI_LEN);
	__builtin_memcpy(event->ja3_hash, info->ja3_hash, 16);
	__builtin_memset(event->pad, 0, 6);

	bpf_ringbuf_submit(event, 0);
}

// Main socket filter program
SEC("socket")
int tls_socket_filter(struct __sk_buff *skb) {
	void *data_end = (void *)(long)skb->data_end;
	void *data = (void *)(long)skb->data;

	// Check minimum packet size (Ethernet + IP + TCP)
	if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) + sizeof(struct tcphdr) > data_end) {
		return 0; // Pass packet
	}

	struct ethhdr *eth = data;
	struct iphdr *ip = (struct iphdr *)(eth + 1);
	struct tcphdr *tcp = (struct tcphdr *)((void *)ip + (ip->ihl * 4));

	// Only process IPv4 TCP packets
	if (eth->h_proto != bpf_htons(ETH_P_IP) || ip->protocol != IPPROTO_TCP) {
		return 0; // Pass packet
	}

	// Check for TLS payload (requires looking past TCP header)
	void *payload = (void *)tcp + (tcp->doff * 4);
	if (payload + 5 > data_end) {
		return 0; // Not enough data for TLS header
	}

	__u8 *tls_data = (__u8 *)payload;
	__u8 content_type = tls_data[0];
	__u16 version = (tls_data[1] << 8) | tls_data[2];
	__u16 length = (tls_data[3] << 8) | tls_data[4];

	if (content_type != TLS_RECORD_HANDSHAKE) {
		return 0; // Not a handshake record
	}

	if (payload + 5 + 4 > data_end) {
		return 0; // Not enough data for handshake header
	}

	__u8 handshake_type = tls_data[5];
	if (handshake_type != TLS_HANDSHAKE_CLIENT_HELLO) {
		return 0; // Only interested in ClientHello for now
	}

	// Create TLS key
	struct tls_key key = {};
	key.src_ip = ip->saddr;
	key.dst_ip = ip->daddr;
	key.src_port = tcp->source;
	key.dst_port = tcp->dest;

	// Process ClientHello
	struct tls_handshake_info info = {};
	info.timestamp = bpf_ktime_get_ns();
	info.version = version;

	// Parse SNI using helper from common.h
	int payload_len = (void *)data_end - payload;
	parse_tls_sni(payload, payload_len, info.sni, MAX_SNI_LEN);

	// Produce JA3-like hash
	produce_ja3_hash(&info);
	
	// Store handshake info and send event
	if (bpf_map_update_elem(&tls_handshakes, &key, &info, BPF_ANY) == 0) {
		increment_stat(STAT_HANDSHAKES_OBSERVED);
		send_tls_event(skb, &key, &info);
	}

	return 0;
}

char _license[] SEC("license") = "GPL";
