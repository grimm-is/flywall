#ifndef COMMON_H
#define COMMON_H

#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/ptrace.h>
#include <linux/in.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/icmp.h>

// Common constants
#define ETH_P_IP 0x0800
#define ETH_P_IPV6 0x86DD
#define ETH_P_ARP 0x0806

#define IPPROTO_TCP 6
#define IPPROTO_UDP 17
#define IPPROTO_ICMP 1
#define IPPROTO_ICMPV6 58

// DNS constants
#define DNS_PORT 53
#define DNS_MAX_NAME 255
#define MAX_DOMAIN_LEN 253

// TLS constants
#define TLS_HANDSHAKE 0x16
#define TLS_CLIENT_HELLO 0x01

// Flow flags
#define FLOW_FLAG_TRUSTED 0x01
#define FLOW_FLAG_OFFLOADED 0x02
#define FLOW_FLAG_BLOCKED 0x04
#define FLOW_FLAG_RATE_LIMITED 0x08

// QoS profiles
#define QOS_PROFILE_DEFAULT 0
#define QOS_PROFILE_HIGH 1
#define QOS_PROFILE_LOW 2
#define QOS_PROFILE_BLOCKED 3

// Event types
#define EVENT_FLOW_CREATED 1
#define EVENT_FLOW_UPDATED 2
#define EVENT_FLOW_EXPIRED 3
#define EVENT_DNS_QUERY 4
#define EVENT_DNS_RESPONSE 5
#define EVENT_TLS_HANDSHAKE 6
#define EVENT_DHCP_DISCOVERY 7
#define EVENT_DHCP_OFFER 8
#define EVENT_ALERT 9
#define EVENT_STATS 10

// Helper macros
#define likely(x) __builtin_expect(!!(x), 1)
#define unlikely(x) __builtin_expect(!!(x), 0)
#define min(a, b) ((a) < (b) ? (a) : (b))
#define max(a, b) ((a) > (b) ? (a) : (b))

// IPv4 address manipulation
#define IPV4_ADDR(a, b, c, d) ((__u32)(((a) & 0xff) << 24) | (((b) & 0xff) << 16) | (((c) & 0xff) << 8) | ((d) & 0xff))

// Port manipulation
#define htons(x) __builtin_bswap16(x)
#define ntohs(x) __builtin_bswap16(x)

// Data structures
struct flow_key {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u8 protocol;
    __u8 padding[3];
};

struct flow_state {
    __u8 verdict;
    __u8 qos_profile;
    __u16 flags;
    __u64 packet_count;
    __u64 byte_count;
    __u64 last_seen;
    __u64 created_at;
    __u64 expires_at;
    __u32 ja3_hash[4]; // For TLS fingerprinting
    char sni[64];      // Server Name Indication
};

struct dns_info {
    char domain[64];
    __u16 qtype;
    __u8 response_code;
};

// DNS key structure
struct dns_key {
	__be32 src_ip;
	__be32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
	__u16 query_id;
};

// DNS query information
struct dns_query_info {
	char domain[253];
	__u16 qtype;
	__u16 qclass;
	__u64 timestamp;
};

// DNS response information
struct dns_response_info {
	__u8 response_code;
	__u16 answer_count;
	__u16 authority_count;
	__u16 additional_count;
	__u64 query_timestamp;
	__u64 response_timestamp;
	char domain[253];
	__u16 packet_size;
};

// DNS event for userspace
struct dns_event {
	__u64 timestamp;
	__u32 pid;
	__u32 tid;
	__be32 src_ip;
	__be32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
	__u16 query_id;
	__u8 is_response;
	__u16 query_type;
	__u16 query_class;
	__u8 response_code;
	__u16 answer_count;
	char domain[253];
	__u16 packet_size;
	__u64 response_time_ns;
};

struct tls_info {
    __u32 ja3_hash[4];
    char sni[64];
    __u16 version;
    __u16 cipher_suite;
};

// TLS key structure
struct tls_key {
	__be32 src_ip;
	__be32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
};

// TLS handshake information
struct tls_handshake_info {
	__u16 version;
	__u16 cipher_suite;
	char sni[MAX_SNI_LEN];
	__u32 ja3_hash[4];
	__u64 timestamp;
};

// TLS event for userspace
struct tls_event {
	__u64 timestamp;
	__u32 pid;
	__u32 tid;
	__be32 src_ip;
	__be32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
	__u16 version;
	__u16 cipher_suite;
	char sni[MAX_SNI_LEN];
	__u32 ja3_hash[4];
	__u16 packet_size;
	__u8 pad[6]; // Explicit padding for 8-byte alignment
};

struct dhcp_info {
    __u32 client_ip;
    __u8 mac_addr[6];
    __u8 message_type;
    char hostname[64];
};

// DHCP key structure
struct dhcp_key {
	__u32 xid; // Transaction ID
	__u8 mac_addr[6];
	__u16 pad; // Padding for alignment
};

// DHCP discover information
struct dhcp_discover_info {
	__u8 mac_addr[6];
	__u8 hostname_len;
	char hostname[64];
	__u8 vendor_class_len;
	char vendor_class[64];
	__u16 packet_size;
	__u64 timestamp;
};

// DHCP offer information
struct dhcp_offer_info {
	__be32 your_ip;
	__be32 server_ip;
	__be32 subnet_mask;
	__be32 router;
	__be32 dns_servers[4];
	__u32 lease_time;
	__u16 packet_size;
	__u64 timestamp;
};

// DHCP request information
struct dhcp_request_info {
	__u8 mac_addr[6];
	__be32 requested_ip;
	__be32 server_ip;
	__u8 hostname_len;
	char hostname[64];
	__u16 packet_size;
	__u64 timestamp;
};

// DHCP acknowledge information
struct dhcp_ack_info {
	__be32 your_ip;
	__be32 server_ip;
	__be32 subnet_mask;
	__be32 router;
	__be32 dns_servers[4];
	__u32 lease_time;
	__u32 renewal_time;
	__u32 rebinding_time;
	__u16 packet_size;
	__u64 timestamp;
};

// DHCP event for userspace
struct dhcp_event {
	__u64 timestamp;
	__u32 pid;
	__u32 tid;
	__be32 src_ip;
	__be32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
	__u8 event_type; // 1 = discover, 2 = offer, 3 = request, 4 = ack
	__u32 xid;
	__u8 mac_addr[6];
	__be32 your_ip;
	__be32 server_ip;
	__be32 subnet_mask;
	__be32 router;
	__be32 dns_servers[4];
	__u32 lease_time;
	__u32 renewal_time;
	__u32 rebinding_time;
	__be32 requested_ip;
	__u8 hostname_len;
	char hostname[64];
	__u8 vendor_class_len;
	char vendor_class[64];
	__u16 packet_size;
};

struct event {
    __u32 type;
    __u64 timestamp;
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u8 protocol;
    __u8 data_len;
    union {
        struct dns_info dns;
        struct tls_info tls;
        struct dhcp_info dhcp;
        __u8 raw[128];
    } data;
};

// Statistics structure
struct statistics {
    __u64 packets_processed;
    __u64 packets_dropped;
    __u64 packets_passed;
    __u64 bytes_processed;
    __u64 blocked_ips;
    __u64 blocked_dns;
    __u64 flows_offloaded;
    __u64 events_generated;
    __u64 last_cleanup;
};

// Rate limiting structure
struct rate_limit {
    __u64 last_packet;
    __u32 packet_count;
    __u32 burst_limit;
};

// Helper functions
static __always_inline __u32 hash_flow_key(struct flow_key *key) {
    __u32 hash = 0;
    hash ^= key->src_ip;
    hash = __builtin_rollover(hash, 13);
    hash ^= key->dst_ip;
    hash = __builtin_rollover(hash, 13);
    hash ^= key->src_port;
    hash = __builtin_rollover(hash, 13);
    hash ^= key->dst_port;
    hash = __builtin_rollover(hash, 13);
    hash ^= key->protocol;
    return hash;
}

static __always_inline __u64 get_time_ns(void) {
    return bpf_ktime_get_ns();
}

static __always_inline int is_expired(__u64 expires_at) {
    return get_time_ns() > expires_at;
}

static __always_inline void update_flow_timestamp(struct flow_state *state) {
    state->last_seen = get_time_ns();
}

static __always_inline int is_port_valid(__u16 port) {
    return port != 0;
}

static __always_inline int is_ip_valid(__u32 ip) {
    return ip != 0 && ip != 0xffffffff;
}

// Packet parsing helpers
static __always_inline struct iphdr *parse_iphdr(void *data, void *data_end) {
    struct ethhdr *eth = data;

    if ((void *)(eth + 1) > data_end) {
        return NULL;
    }

    if (eth->h_proto != __constant_htons(ETH_P_IP)) {
        return NULL;
    }

    struct iphdr *ip = (void *)(eth + 1);
    if ((void *)(ip + 1) > data_end) {
        return NULL;
    }

    return ip;
}

static __always_inline struct tcphdr *parse_tcphdr(struct iphdr *ip, void *data_end) {
    if (ip->protocol != IPPROTO_TCP) {
        return NULL;
    }

    void *transport_start = (void *)ip + (ip->ihl * 4);
    struct tcphdr *tcp = transport_start;

    if ((void *)(tcp + 1) > data_end) {
        return NULL;
    }

    return tcp;
}

static __always_inline struct udphdr *parse_udphdr(struct iphdr *ip, void *data_end) {
    if (ip->protocol != IPPROTO_UDP) {
        return NULL;
    }

    void *transport_start = (void *)ip + (ip->ihl * 4);
    struct udphdr *udp = transport_start;

    if ((void *)(udp + 1) > data_end) {
        return NULL;
    }

    return udp;
}

// DNS parsing helpers
static __always_inline int parse_dns_name(const __u8 *data, int data_len,
                                        int *pos, char *name, int name_max) {
    int name_len = 0;
    int original_pos = *pos;
    int jumped = 0;
    int jumps = 0;

    while (*pos < data_len && name_len < name_max - 1) {
        __u8 len = data[*pos];
        (*pos)++;

        if (len == 0) {
            break;
        }

        // Check for compression pointer
        if ((len & 0xc0) == 0xc0) {
            if (*pos >= data_len) {
                return -1;
            }

            if (!jumped) {
                original_pos = *pos + 1;
                jumped = 1;
            }

            if (jumps++ > 5) {
                return -1; // Too many jumps
            }

            __u16 offset = ((len & 0x3f) << 8) | data[*pos];
            *pos = offset;
            continue;
        }

        if (*pos + len > data_len) {
            return -1;
        }

        if (name_len > 0) {
            name[name_len++] = '.';
        }

        for (int i = 0; i < len && name_len < name_max - 1; i++) {
            char c = data[*pos + i];
            if (c >= ' ' && c <= '~') {
                name[name_len++] = c;
            } else {
                name[name_len++] = '?';
            }
        }

        *pos += len;
    }

    name[name_len] = '\0';

    if (jumped) {
        *pos = original_pos;
    }

    return name_len;
}

// TLS parsing helpers
static __always_inline int parse_tls_sni(const __u8 *data, int data_len,
                                        char *sni, int sni_max) {
    // Skip TLS header (5 bytes) and handshake header (4 bytes)
    if (data_len < 9) {
        return -1;
    }

    int pos = 9; // Skip TLS record + handshake headers

    // Skip session ID
    if (pos + 1 >= data_len) {
        return -1;
    }
    __u8 session_id_len = data[pos++];
    pos += session_id_len;

    // Skip cipher suites
    if (pos + 2 >= data_len) {
        return -1;
    }
    __u16 cipher_suites_len = (data[pos] << 8) | data[pos + 1];
    pos += 2 + cipher_suites_len;

    // Skip compression methods
    if (pos + 1 >= data_len) {
        return -1;
    }
    __u8 compression_methods_len = data[pos++];
    pos += compression_methods_len;

    // Parse extensions
    if (pos + 2 >= data_len) {
        return -1;
    }
    __u16 extensions_len = (data[pos] << 8) | data[pos + 1];
    pos += 2;

    int extensions_end = pos + extensions_len;
    while (pos + 4 <= extensions_end && pos < data_len) {
        __u16 ext_type = (data[pos] << 8) | data[pos + 1];
        __u16 ext_len = (data[pos + 2] << 8) | data[pos + 3];
        pos += 4;

        if (pos + ext_len > data_len) {
            return -1;
        }

        // SNI extension type is 0
        if (ext_type == 0) {
            // Skip SNI list length (2 bytes)
            if (pos + 2 > data_len) {
                return -1;
            }
            pos += 2;

            // Skip name type (1 byte) and name length (2 bytes)
            if (pos + 3 > data_len) {
                return -1;
            }
            pos += 3;

            // Extract SNI
            __u16 sni_len = (data[pos - 2] << 8) | data[pos - 1];
            if (sni_len >= sni_max) {
                sni_len = sni_max - 1;
            }

            for (int i = 0; i < sni_len && pos + i < data_len; i++) {
                sni[i] = data[pos + i];
            }
            sni[sni_len] = '\0';

            return sni_len;
        }

        pos += ext_len;
    }

    return 0;
}

#endif /* COMMON_H */
