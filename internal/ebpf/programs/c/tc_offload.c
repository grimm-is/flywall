#include "vmlinux.h"
#include "bpf_helpers.h"
#include "bpf_endian.h"
#include "common.h"

// Flow verdicts
#define VERDICT_UNKNOWN  0
#define VERDICT_TRUSTED  1
#define VERDICT_DROP     2

// nftables marks for flow bypass
#define NFQUEUE_BYPASS_MARK 0x200000  // Skip NFQUEUE, go to acceptance

// QoS profile structure
struct qos_profile {
    __u32 rate_limit;
    __u32 burst_limit;
    __u8 priority;
    __u8 app_class;
    __u8 padding[2];
};

// Maps
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct flow_key);
    __type(value, struct flow_state);
    __uint(max_entries, 100000);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} flow_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, __u32);
    __type(value, struct qos_profile);
    __uint(max_entries, 16);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} qos_profiles SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, __u32);
    __type(value, struct tc_stats);
    __uint(max_entries, 1);
} tc_stats_map SEC(".maps");

// TC statistics
struct tc_stats {
	__u64 packets_processed;
	__u64 packets_fast_path;
	__u64 packets_slow_path;
	__u64 packets_dropped;
	__u64 bytes_processed;
};

// Helper functions
static __always_inline int extract_flow_key(struct __sk_buff *skb, struct flow_key *key)
{
	void *data_end = (void *)(long)skb->data_end;
	void *data = (void *)(long)skb->data;

	// Minimum packet size for Ethernet + IP
	if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) > data_end)
		return -1;

	struct ethhdr *eth = data;
	struct iphdr *ip = (struct iphdr *)(eth + 1);

	// Only handle IPv4 for now
	if (eth->h_proto != __bpf_constant_htons(ETH_P_IP))
		return -1;

	// Extract basic 5-tuple
	key->src_ip = ip->saddr;
	key->dst_ip = ip->daddr;
	key->ip_proto = ip->protocol;

	// Handle transport layer ports
	if (ip->protocol == IPPROTO_TCP) {
		struct tcphdr *tcp = (struct tcphdr *)((void *)ip + (ip->ihl * 4));
		if ((void *)tcp + sizeof(struct tcphdr) > data_end)
			return -1;

		key->src_port = __bpf_ntohs(tcp->source);
		key->dst_port = __bpf_ntohs(tcp->dest);
	} else if (ip->protocol == IPPROTO_UDP) {
		struct udphdr *udp = (struct udphdr *)((void *)ip + (ip->ihl * 4));
		if ((void *)udp + sizeof(struct udphdr) > data_end)
			return -1;

		key->src_port = __bpf_ntohs(udp->source);
		key->dst_port = __bpf_ntohs(udp->dest);
	} else {
		key->src_port = 0;
		key->dst_port = 0;
	}

	// Use ingress interface as part of key
	key->ifindex = skb->ifindex;

	return 0;
}

// Apply QoS marking based on flow state
static inline int apply_qos(struct __sk_buff *skb, struct flow_state *state)
{
    if (state->qos_profile == QOS_PROFILE_DEFAULT)
        return TC_ACT_OK;

    struct qos_profile *qos = bpf_map_lookup_elem(&qos_profiles, &state->qos_profile);
    if (!qos)
        return TC_ACT_OK;

    // Set priority for hardware scheduling
    skb->priority = qos->priority;

    // Set queue mapping if needed
    if (qos->app_class == QOS_PROFILE_VIDEO || qos->app_class == QOS_PROFILE_VOICE) {
        skb->queue_mapping = qos->app_class;
    }

    // Mark packet for QoS handling
    skb->mark |= 0x100000;  // QoS mark bit

    return TC_ACT_OK;
}

static __always_inline void update_stats(__u64 packets, __u64 bytes, __u64 fast_path, __u64 slow_path, __u64 dropped)
{
	__u32 key = 0;
	struct tc_stats *stats = bpf_map_lookup_elem(&tc_stats_map, &key);
	if (!stats)
		return;

	__sync_fetch_and_add(&stats->packets_processed, packets);
	__sync_fetch_and_add(&stats->bytes_processed, bytes);
	__sync_fetch_and_add(&stats->packets_fast_path, fast_path);
	__sync_fetch_and_add(&stats->packets_slow_path, slow_path);
	__sync_fetch_and_add(&stats->packets_dropped, dropped);
}

// Main TC classifier
SEC("tc")
int tc_fast_path(struct __sk_buff *skb)
{
	struct flow_key key = {};

	// Extract flow key
	if (extract_flow_key(skb, &key) < 0) {
		// Can't extract flow info, let it pass to normal processing
		update_stats(1, skb->len, 0, 1, 0);
		return TC_ACT_OK;
	}

	// Lookup flow state in the map
	struct flow_state *state = bpf_map_lookup_elem(&flow_map, &key);

	if (!state) {
		// Unknown flow -> Pass to Stack -> NFQUEUE
		// This is the slow path for new flows
		update_stats(1, skb->len, 0, 1, 0);
		return TC_ACT_OK;
	}

	// Update flow statistics
	__sync_fetch_and_add(&state->packet_count, 1);
	__sync_fetch_and_add(&state->byte_count, skb->len);
	__sync_fetch_and_add(&state->last_seen, bpf_ktime_get_ns());

	// Check flow verdict
	if (state->verdict == VERDICT_TRUSTED) {
		// Trusted flow - mark for nftables bypass
		skb->mark = NFQUEUE_BYPASS_MARK;

		// Apply QoS marking if configured
		int qos_result = apply_qos(skb, state);
		if (qos_result != TC_ACT_OK) {
			update_stats(1, skb->len, 0, 0, 1);
			return qos_result;
		}

		// Update fast path stats
		update_stats(1, skb->len, 1, 0, 0);

		// Optional: If we have a simple redirect case, we could use:
		// return bpf_redirect(skb->ifindex, 0);

		return TC_ACT_OK;
	} else if (state->verdict == VERDICT_DROP) {
		// Blocked flow - drop early
		update_stats(1, skb->len, 0, 0, 1);
		return TC_ACT_SHOT;
	}

	// Unknown verdict - pass to normal processing
	update_stats(1, skb->len, 0, 1, 0);
	return TC_ACT_OK;
}

// TC egress program for outbound traffic
SEC("tc")
int tc_egress_fast_path(struct __sk_buff *skb)
{
	struct flow_key key = {};

	// Extract flow key (reverse direction for egress)
	if (extract_flow_key(skb, &key) < 0) {
		return TC_ACT_OK;
	}

	// Swap src/dst for egress lookup
	__u32 tmp_ip = key.src_ip;
	key.src_ip = key.dst_ip;
	key.dst_ip = tmp_ip;

	__u16 tmp_port = key.src_port;
	key.src_port = key.dst_port;
	key.dst_port = tmp_port;

	// Lookup flow state
	struct flow_state *state = bpf_map_lookup_elem(&flow_map, &key);

	if (!state) {
		return TC_ACT_OK;
	}

	// Update flow statistics
	__sync_fetch_and_add(&state->packet_count, 1);
	__sync_fetch_and_add(&state->byte_count, skb->len);

	// Apply verdict
	if (state->verdict == VERDICT_TRUSTED) {
		skb->mark = NFQUEUE_BYPASS_MARK;
		return TC_ACT_OK;
	} else if (state->verdict == VERDICT_DROP) {
		return TC_ACT_SHOT;
	}

	return TC_ACT_OK;
}
