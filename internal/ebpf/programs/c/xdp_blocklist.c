#include "vmlinux.h"
#include "bpf_helpers.h"
#include "common.h"

// XDP return values
#ifndef XDP_ABORTED
#define XDP_ABORTED 0
#endif
#ifndef XDP_DROP
#define XDP_DROP 1
#endif
#ifndef XDP_PASS
#define XDP_PASS 2
#endif
#ifndef XDP_TX
#define XDP_TX 3
#endif
#ifndef XDP_REDIRECT
#define XDP_REDIRECT 4
#endif

// BPF flags
#ifndef BPF_ANY
#define BPF_ANY 0
#endif
#ifndef BPF_NOEXIST
#define BPF_NOEXIST 1
#endif
#ifndef BPF_EXIST
#define BPF_EXIST 2
#endif

// Map definitions
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 1000000);
} ip_blocklist SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __type(key, struct flow_key);
    __type(value, struct flow_state);
    __uint(max_entries, 1000000);
} flow_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, __u32);
    __type(value, struct statistics);
    __uint(max_entries, 1);
} statistics SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, __u32);
    __type(value, __u64);
    __uint(max_entries, 1);
} config SEC(".maps");

// DNS bloom filter for fast domain blocking
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, __u32);
    __type(value, __u8);
    __uint(max_entries, 131072); // 1MB / 8 bytes = 131K entries
} dns_bloom SEC(".maps");

// Event ring buffer for userspace events
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 20);
} events SEC(".maps");

// Constants
#define XDP_PASS 2
#define XDP_DROP 1
#define XDP_TX 3
#define XDP_REDIRECT 4
#define FLOW_TIMEOUT_NS 300000000000ULL // 5 minutes
#define DNS_PORT 53
#define TRUSTED_FLOW_FLAG 0x01
#define RATE_LIMIT_WINDOW_NS 1000000000ULL // 1 second

// Helper functions
static __always_inline __u32 hash_ip(__u32 ip) {
    return ip;
}

static __always_inline int is_ip_blocked(__u32 ip) {
    __u64 *blocked = bpf_map_lookup_elem(&ip_blocklist, &ip);
    return blocked != NULL;
}

static __always_inline int is_domain_blocked(const char *domain, int len) {
    // Simple hash for domain
    __u32 hash = 0;
    for (int i = 0; i < len && i < 64; i++) {
        hash = hash * 31 + domain[i];
    }

    // Check bloom filter
    __u32 index = (hash % (131072 * 8)) / 8;
    __u32 bit = hash % 8;

    __u8 *filter = bpf_map_lookup_elem(&dns_bloom, &index);
    if (!filter) {
        return 0;
    }

    return (*filter & (1 << bit)) != 0;
}

static __always_inline void update_statistics(__u32 packets, __u32 bytes, __u32 dropped) {
    struct statistics *stats = bpf_map_lookup_elem(&statistics, &(u32){0});
    if (!stats) {
        return;
    }

    __sync_fetch_and_add(&stats->packets_processed, packets);
    __sync_fetch_and_add(&stats->bytes_processed, bytes);

    if (dropped) {
        __sync_fetch_and_add(&stats->packets_dropped, dropped);
    } else {
        __sync_fetch_and_add(&stats->packets_passed, packets);
    }
}

static __always_inline void send_event(__u32 type, struct xdp_md *ctx,
                                     __u32 src_ip, __u32 dst_ip,
                                     __u16 src_port, __u16 dst_port,
                                     __u8 protocol, const void *data, __u8 data_len) {
    struct event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) {
        return;
    }

    e->type = type;
    e->src_ip = src_ip;
    e->dst_ip = dst_ip;
    e->src_port = src_port;
    e->dst_port = dst_port;
    e->protocol = protocol;
    e->data_len = data_len;
    e->timestamp = bpf_ktime_get_ns();

    if (data && data_len > 0) {
        __builtin_memcpy(e->data, data, data_len > 64 ? 64 : data_len);
    }

    bpf_ringbuf_submit(e, 0);
}

static __always_inline struct flow_state *get_or_create_flow(struct flow_key *key) {
    struct flow_state *state = bpf_map_lookup_elem(&flow_map, key);
    if (state) {
        return state;
    }

    // Create new flow
    struct flow_state new_state = {
        .verdict = XDP_PASS,
        .qos_profile = 0,
        .flags = 0,
        .created_at = bpf_ktime_get_ns(),
        .expires_at = bpf_ktime_get_ns() + FLOW_TIMEOUT_NS,
    };

    // Insert new flow
    if (bpf_map_update_elem(&flow_map, key, &new_state, BPF_NOEXIST) == 0) {
        return bpf_map_lookup_elem(&flow_map, key);
    }

    return NULL;
}

static __always_inline int rate_limit_check(__u32 ip) {
    // Simple rate limiting per IP
    // In production, use token bucket or more sophisticated algorithm
    __u64 now = bpf_ktime_get_ns();
    __u64 *last_packet = bpf_map_lookup_elem(&ip_blocklist, &ip);

    if (last_packet && (now - *last_packet) < RATE_LIMIT_WINDOW_NS) {
        return 0; // Rate limited
    }

    // Update last packet time
    if (bpf_map_update_elem(&ip_blocklist, &ip, &now, BPF_EXIST) != 0) {
        bpf_map_update_elem(&ip_blocklist, &ip, &now, BPF_NOEXIST);
    }

    return 1; // Allowed
}

// Main XDP program
SEC("xdp")
int xdp_blocklist_prog(struct xdp_md *ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;

    // Minimum packet size check
    if (data + sizeof(struct ethhdr) > data_end) {
        return XDP_PASS;
    }

    struct ethhdr *eth = data;

    // Only handle IPv4 for now
    if (eth->h_proto != __bpf_constant_htons(ETH_P_IP)) {
        return XDP_PASS;
    }

    if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) > data_end) {
        return XDP_PASS;
    }

    struct iphdr *ip = (struct iphdr *)(data + sizeof(struct ethhdr));

    // Check for fragments - pass them through to avoid reading invalid L4 data
    if (ip->frag_off & __bpf_constant_htons(0x3FFF)) {
        return XDP_PASS;
    }

    // Extract IP addresses
    __u32 src_ip = ip->saddr;
    __u32 dst_ip = ip->daddr;

    // Check if source IP is blocked
    if (is_ip_blocked(src_ip)) {
        update_statistics(1, bpf_ntohs(ip->tot_len), 1);
        send_event(1, ctx, src_ip, dst_ip, 0, 0, ip->protocol, NULL, 0);
        return XDP_DROP;
    }

    // Rate limiting check
    if (!rate_limit_check(src_ip)) {
        update_statistics(1, bpf_ntohs(ip->tot_len), 1);
        return XDP_DROP;
    }

    // Create flow key
    struct flow_key flow_key = {
        .src_ip = src_ip,
        .dst_ip = dst_ip,
        .src_port = 0,
        .dst_port = 0,
        .ip_proto = ip->protocol,
    };

    // Handle TCP/UDP for port information
    if (ip->protocol == IPPROTO_TCP || ip->protocol == IPPROTO_UDP) {
        if (data + sizeof(struct ethhdr) + sizeof(struct iphdr) + sizeof(struct udphdr) > data_end) {
            return XDP_PASS;
        }

        struct udphdr *udp = (struct udphdr *)(data + sizeof(struct ethhdr) + sizeof(struct iphdr));
        flow_key.src_port = udp->source;
        flow_key.dst_port = udp->dest;

        // DNS blocking check
        if (ip->protocol == IPPROTO_UDP && udp->dest == __bpf_constant_htons(DNS_PORT)) {
            // Extract DNS query (simplified)
            void *dns_data = (void *)udp + sizeof(struct udphdr);
            if (dns_data + 12 > data_end) { // DNS header size
                return XDP_PASS;
            }

            // Check for DNS query
            __u8 *dns = dns_data;
            if (dns[2] & 0x80) { // QR bit set - it's a response
                return XDP_PASS;
            }

            // Extract domain name (simplified)
            char domain[64];
            int domain_len = 0;
            int pos = 12; // Skip DNS header

            while (pos < 63 && dns_data + pos < data_end) {
                __u8 len = dns[pos];
                if (len == 0) break;

                if (domain_len + len + 1 > 63) break;

                if (domain_len > 0) {
                    domain[domain_len++] = '.';
                }

                for (int i = 0; i < len && pos + i + 1 < 64 && dns_data + pos + i + 1 < data_end; i++) {
                    domain[domain_len++] = dns[pos + i + 1];
                }

                pos += len + 1;
            }

            // Check if domain is blocked
            if (is_domain_blocked(domain, domain_len)) {
                update_statistics(1, bpf_ntohs(ip->tot_len), 1);
                send_event(2, ctx, src_ip, dst_ip, udp->source, udp->dest, ip->protocol, domain, domain_len);
                return XDP_DROP;
            }
        }
    }

    // Get or create flow state
    struct flow_state *flow_state = get_or_create_flow(&flow_key);
    if (!flow_state) {
        return XDP_PASS;
    }

    // Update flow statistics
    __u64 now = bpf_ktime_get_ns();
    __u32 packet_len = bpf_ntohs(ip->tot_len);

    flow_state->packet_count++;
    flow_state->byte_count += packet_len;
    flow_state->last_seen = now;

    // Check if flow should be offloaded (trusted after many packets)
    if (flow_state->packet_count > 100 && !(flow_state->flags & TRUSTED_FLOW_FLAG)) {
        flow_state->flags |= TRUSTED_FLOW_FLAG;
        __sync_fetch_and_add(&((struct statistics *)bpf_map_lookup_elem(&statistics, &(u32){0}))->flows_offloaded, 1);
    }

    // Apply flow verdict if set
    if (flow_state->verdict == XDP_DROP) {
        update_statistics(1, packet_len, 1);
        return XDP_DROP;
    }

    // Update statistics and pass
    update_statistics(1, packet_len, 0);
    return XDP_PASS;
}

// License
char _license[] SEC("license") = "GPL";
