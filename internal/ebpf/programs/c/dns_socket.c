#include "vmlinux.h"
#include "bpf_helpers.h"
#include "common.h"

// Endian conversion helpers if not available
#ifndef __bpf_ntohs
#define __bpf_ntohs(x) __builtin_bswap16(x)
#endif

#ifndef __bpf_htons
#define __bpf_htons(x) __builtin_bswap16(x)
#endif

// BPF flags
#ifndef BPF_ANY
#define BPF_ANY 0
#endif

#define DNS_PORT 53
#define MAX_DOMAIN_LEN 253
#define DNS_QUERY 0
#define DNS_RESPONSE 1

// Maps for tracking DNS queries and responses
struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 65536);
	__type(key, struct dns_key);
	__type(value, struct dns_query_info);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} dns_queries SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 65536);
	__type(key, __u16); // Query ID
	__type(value, struct dns_response_info);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} dns_responses SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(max_entries, 10);
	__type(key, __u32);
	__type(value, __u64);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} dns_stats SEC(".maps");

// Ring buffer for events
struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 1024 * 256);
} dns_events SEC(".maps");

// Statistics indices
enum {
	STAT_QUERIES_PROCESSED = 0,
	STAT_RESPONSES_PROCESSED = 1,
	STAT_QUERIES_BLOCKED = 2,
	STAT_RESPONSES_BLOCKED = 3,
	STAT_PACKETS_DROPPED = 4,
	STAT_ERRORS = 5,
	STAT_MAX,
};

// Helper function to increment statistics
static __always_inline void increment_stat(__u32 stat_idx) {
	__u64 *count = bpf_map_lookup_elem(&dns_stats, &stat_idx);
	if (count) {
		__sync_fetch_and_add(count, 1);
	}
}

// Helper function to extract domain from DNS packet
// Send DNS event to userspace
static __always_inline void send_dns_event(struct __sk_buff *skb,
                                          const struct dns_key *key,
                                          const struct dns_query_info *query_info,
                                          const struct dns_response_info *response_info,
                                          __u8 is_response) {
	struct dns_event *event = bpf_ringbuf_reserve(&dns_events, sizeof(*event), 0);
	if (!event) {
		increment_stat(STAT_ERRORS);
		return;
	}

	event->timestamp = bpf_ktime_get_ns();
	event->pid = bpf_get_current_pid_tgid() >> 32;
	event->tid = bpf_get_current_pid_tgid() & 0xFFFFFFFF;
	event->src_ip = key->src_ip;
	event->dst_ip = key->dst_ip;
	event->src_port = key->src_port;
	event->dst_port = key->dst_port;
	event->query_id = key->query_id;
	event->is_response = is_response;

	if (is_response && response_info) {
		event->response_code = response_info->response_code;
		event->answer_count = response_info->answer_count;
		event->packet_size = response_info->packet_size;
		__builtin_memcpy(event->domain, response_info->domain, MAX_DOMAIN_LEN);

		// Calculate response time
		if (response_info->query_timestamp > 0) {
			event->response_time_ns = response_info->response_timestamp - response_info->query_timestamp;
		}
	} else if (!is_response && query_info) {
		event->query_type = query_info->query_type;
		event->query_class = query_info->query_class;
		event->packet_size = query_info->packet_size;
		__builtin_memcpy(event->domain, query_info->domain, MAX_DOMAIN_LEN);
	}

	bpf_ringbuf_submit(event, 0);
}

// Main socket filter program
SEC("socket")
int dns_socket_filter(struct __sk_buff *skb) {
	void *data_end = (void *)(long)skb->data_end;
	void *data = (void *)(long)skb->data;

	// Check minimum packet size (Ethernet + IP + UDP)
	if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) + sizeof(struct udphdr) > data_end) {
		return 0; // Pass packet
	}

	struct ethhdr *eth = data;
	struct iphdr *ip = (struct iphdr *)(eth + 1);
	struct udphdr *udp = (struct udphdr *)(ip + 1);

	// Only process IPv4 UDP packets
	if (eth->h_proto != __bpf_constant_htons(ETH_P_IP) || ip->protocol != IPPROTO_UDP) {
		return 0; // Pass packet
	}

	// Check for DNS port (53)
	if (udp->dest != __bpf_constant_htons(DNS_PORT) && udp->source != __bpf_constant_htons(DNS_PORT)) {
		return 0; // Pass packet
	}

	// Get DNS data
	void *dns_data = (void *)udp + sizeof(struct udphdr);
	int dns_len = bpf_ntohs(udp->len) - sizeof(struct udphdr);

	if (dns_data + dns_len > data_end || dns_len < 12) { // DNS header is 12 bytes
		return 0; // Pass packet
	}

	// Parse DNS header
	__u8 *dns = (__u8 *)dns_data;
	__u16 transaction_id = (dns[0] << 8) | dns[1];
	__u16 flags = (dns[2] << 8) | dns[3];
	__u16 questions = (dns[4] << 8) | dns[5];
	__u16 answers = (dns[6] << 8) | dns[7];
	__u8 is_response = (flags & 0x8000) != 0;

	// Create DNS key
	struct dns_key key = {};
	key.query_id = transaction_id;
	key.src_ip = ip->saddr;
	key.dst_ip = ip->daddr;
	key.src_port = udp->source;
	key.dst_port = udp->dest;
	key.pad = 0;

	// Parse domain name
	int pos = 12; // Skip DNS header
	char domain[MAX_DOMAIN_LEN] = {};
	int domain_len = extract_domain(dns, dns_len, &pos, domain, sizeof(domain));

	if (domain_len < 0) {
		increment_stat(STAT_ERRORS);
		return 0; // Pass packet
	}

	// Check if this is a query or response
	if (!is_response && questions > 0) {
		// DNS Query
		struct dns_query_info query_info = {};
		query_info.packet_size = skb->len;
		query_info.timestamp = bpf_ktime_get_ns();
		__builtin_memcpy(query_info.domain, domain, sizeof(domain));

		// Extract query type and class
		if (pos + 4 <= dns_len) {
			query_info.query_type = (dns[pos] << 8) | dns[pos + 1];
			query_info.query_class = (dns[pos + 2] << 8) | dns[pos + 3];
		}

		// Store query information
		if (bpf_map_update_elem(&dns_queries, &key, &query_info, BPF_ANY) == 0) {
			increment_stat(STAT_QUERIES_PROCESSED);

			// Send event to userspace
			send_dns_event(skb, &key, &query_info, NULL, DNS_QUERY);
		} else {
			increment_stat(STAT_ERRORS);
		}

	} else if (is_response && answers > 0) {
		// DNS Response
		struct dns_response_info response_info = {};
		response_info.packet_size = skb->len;
		response_info.response_timestamp = bpf_ktime_get_ns();
		response_info.answer_count = answers;
		response_info.authority_count = (dns[8] << 8) | dns[9];
		response_info.additional_count = (dns[10] << 8) | dns[11];
		response_info.response_code = flags & 0x000F;
		__builtin_memcpy(response_info.domain, domain, sizeof(domain));

		// Try to find matching query
		// Need to reverse the key for response lookup
		struct dns_key lookup_key = {};
		lookup_key.src_ip = ip->daddr; // Swap src/dst
		lookup_key.dst_ip = ip->saddr;
		lookup_key.src_port = udp->dest; // Swap ports
		lookup_key.dst_port = udp->source;
		lookup_key.query_id = transaction_id;
		lookup_key.pad = 0;

		struct dns_query_info *query = bpf_map_lookup_elem(&dns_queries, &lookup_key);
		if (query) {
			response_info.query_timestamp = query->timestamp;
		}

		// Store response information
		if (bpf_map_update_elem(&dns_responses, &transaction_id, &response_info, BPF_ANY) == 0) {
			increment_stat(STAT_RESPONSES_PROCESSED);

			// Send event to userspace
			send_dns_event(skb, &key, NULL, &response_info, DNS_RESPONSE);
		} else {
			increment_stat(STAT_ERRORS);
		}

		// Clean up query entry
		bpf_map_delete_elem(&dns_queries, &key);
	}

	// Return 0 to pass packet, non-zero to drop
	// For monitoring, we typically want to pass the packet
	return 0;
}

char _license[] SEC("license") = "GPL";
