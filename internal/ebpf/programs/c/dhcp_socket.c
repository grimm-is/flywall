#include "vmlinux.h"
#include "bpf_helpers.h"
#include "common.h"

// BPF flags
#ifndef BPF_ANY
#define BPF_ANY 0
#endif

// Endian conversion helpers
#ifndef __bpf_ntohs
#define __bpf_ntohs(x) __builtin_bswap16(x)
#endif

#ifndef __bpf_htons
#define __bpf_htons(x) __builtin_bswap16(x)
#endif

#define DHCP_CLIENT_PORT 68
#define DHCP_SERVER_PORT 67
#define DHCP_MAGIC_COOKIE 0x63825363
#define DHCP_OPTION_MESSAGE_TYPE 53
#define DHCP_OPTION_HOST_NAME 12
#define DHCP_OPTION_VENDOR_CLASS 60
#define DHCP_OPTION_PARAMETER_REQUEST 55
#define DHCP_OPTION_SUBNET_MASK 1
#define DHCP_OPTION_ROUTER 3
#define DHCP_OPTION_DNS_SERVER 6
#define DHCP_OPTION_SERVER_ID 54
#define DHCP_OPTION_LEASE_TIME 51
#define DHCP_OPTION_RENEWAL_TIME 58
#define DHCP_OPTION_REBINDING_TIME 59
#define DHCP_OPTION_REQUESTED_IP 50

// Maps for tracking DHCP transactions
struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 65536);
	__type(key, struct dhcp_key);
	__type(value, struct dhcp_discover_info);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} dhcp_discovers SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 65536);
	__type(key, struct dhcp_key);
	__type(value, struct dhcp_offer_info);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} dhcp_offers SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 65536);
	__type(key, struct dhcp_key);
	__type(value, struct dhcp_request_info);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} dhcp_requests SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 65536);
	__type(key, struct dhcp_key);
	__type(value, struct dhcp_ack_info);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} dhcp_acks SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_ARRAY);
	__uint(max_entries, 10);
	__type(key, __u32);
	__type(value, __u64);
	__uint(pinning, LIBBPF_PIN_BY_NAME);
} dhcp_stats SEC(".maps");

// Ring buffer for events
struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 1024 * 256);
} dhcp_events SEC(".maps");

// Statistics indices
enum {
	STAT_DISCOVERS_SEEN = 0,
	STAT_OFFERS_SEEN = 1,
	STAT_REQUESTS_SEEN = 2,
	STAT_ACKS_SEEN = 3,
	STAT_TRANSACTIONS_TRACKED = 4,
	STAT_DEVICES_DISCOVERED = 5,
	STAT_ROGUE_SERVERS_DETECTED = 6,
	STAT_INVALID_PACKETS_BLOCKED = 7,
	STAT_ERRORS = 8,
	STAT_MAX,
};

// Helper function to increment statistics
static __always_inline void increment_stat(__u32 stat_idx) {
	__u64 *count = bpf_map_lookup_elem(&dhcp_stats, &stat_idx);
	if (count) {
		__sync_fetch_and_add(count, 1);
	}
}

// Helper function to parse DHCP options
static __always_inline int parse_dhcp_options(const __u8 *data, int data_len,
                                             int *pos, __u8 option_type,
                                             __u8 *option_value, int max_len) {
	while (*pos + 2 <= data_len) {
		__u8 opt = data[*pos];
		__u8 opt_len = data[*pos + 1];
		*pos += 2;

		if (opt == 255) { // End of options
			break;
		}

		if (opt == option_type) {
			if (opt_len > max_len) {
				opt_len = max_len;
			}
			if (*pos + opt_len <= data_len) {
				bpf_probe_read_kernel(option_value, opt_len, data + *pos);
				return opt_len;
			}
		}

		*pos += opt_len;
	}

	return -1;
}

// Send DHCP event to userspace
static __always_inline void send_dhcp_event(struct __sk_buff *skb,
                                          const struct dhcp_key *key,
                                          const struct dhcp_discover_info *discover_info,
                                          const struct dhcp_offer_info *offer_info,
                                          const struct dhcp_request_info *request_info,
                                          const struct dhcp_ack_info *ack_info,
                                          __u8 event_type) {
	struct dhcp_event *event = bpf_ringbuf_reserve(&dhcp_events, sizeof(*event), 0);
	if (!event) {
		increment_stat(STAT_ERRORS);
		return;
	}

	event->timestamp = bpf_ktime_get_ns();
	event->pid = bpf_get_current_pid_tgid() >> 32;
	event->tid = (__u32)bpf_get_current_pid_tgid();
	event->src_ip = skb->protocol == __bpf_constant_htons(ETH_P_IP) ?
		((struct iphdr *)(skb->data + sizeof(struct ethhdr)))->saddr : 0;
	event->dst_ip = skb->protocol == __bpf_constant_htons(ETH_P_IP) ?
		((struct iphdr *)(skb->data + sizeof(struct ethhdr)))->daddr : 0;
	event->src_port = skb->protocol == __bpf_constant_htons(ETH_P_IP) ?
		((struct udphdr *)(skb->data + sizeof(struct ethhdr) + sizeof(struct iphdr)))->source : 0;
	event->dst_port = skb->protocol == __bpf_constant_htons(ETH_P_IP) ?
		((struct udphdr *)(skb->data + sizeof(struct ethhdr) + sizeof(struct iphdr)))->dest : 0;
	event->event_type = event_type;
	event->xid = key->xid;
	__builtin_memcpy(event->mac_addr, key->mac_addr, 6);

	if (event_type == 1 && discover_info) {
		// Discover event
		__builtin_memcpy(event->hostname, discover_info->hostname, 64);
		__builtin_memcpy(event->vendor_class, discover_info->vendor_class, 64);
		event->hostname_len = discover_info->hostname_len;
		event->vendor_class_len = discover_info->vendor_class_len;
		event->packet_size = discover_info->packet_size;
	} else if (event_type == 2 && offer_info) {
		// Offer event
		event->your_ip = offer_info->your_ip;
		event->server_ip = offer_info->server_ip;
		event->subnet_mask = offer_info->subnet_mask;
		event->router = offer_info->router;
		__builtin_memcpy(event->dns_servers, offer_info->dns_servers, 16);
		event->lease_time = offer_info->lease_time;
		event->packet_size = offer_info->packet_size;
	} else if (event_type == 3 && request_info) {
		// Request event
		event->requested_ip = request_info->requested_ip;
		event->server_ip = request_info->server_ip;
		__builtin_memcpy(event->hostname, request_info->hostname, 64);
		event->hostname_len = request_info->hostname_len;
		event->packet_size = request_info->packet_size;
	} else if (event_type == 4 && ack_info) {
		// ACK event
		event->your_ip = ack_info->your_ip;
		event->server_ip = ack_info->server_ip;
		event->subnet_mask = ack_info->subnet_mask;
		event->router = ack_info->router;
		__builtin_memcpy(event->dns_servers, ack_info->dns_servers, 16);
		event->lease_time = ack_info->lease_time;
		event->renewal_time = ack_info->renewal_time;
		event->rebinding_time = ack_info->rebinding_time;
		event->packet_size = ack_info->packet_size;
	}

	bpf_ringbuf_submit(event, 0);
}

// Main socket filter program
SEC("socket")
int dhcp_socket_filter(struct __sk_buff *skb) {
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

	// Check for DHCP ports
	if ((udp->dest != __bpf_constant_htons(DHCP_SERVER_PORT) && udp->source != __bpf_constant_htons(DHCP_SERVER_PORT)) &&
	    (udp->dest != __bpf_constant_htons(DHCP_CLIENT_PORT) && udp->source != __bpf_constant_htons(DHCP_CLIENT_PORT))) {
		return 0; // Pass packet
	}

	// Get DHCP data
	void *dhcp_data = (void *)udp + sizeof(struct udphdr);
	int dhcp_len = bpf_ntohs(udp->len) - sizeof(struct udphdr);

	if (dhcp_data + dhcp_len > data_end || dhcp_len < 240) { // Minimum DHCP packet size
		return 0; // Pass packet
	}

	// Parse DHCP header
	__u8 *dhcp = (__u8 *)dhcp_data;
	__u8 op = dhcp[0]; // Message op code
	__u8 htype = dhcp[1]; // Hardware address type
	__u8 hlen = dhcp[2]; // Hardware address length
	__u32 xid = (dhcp[4] << 24) | (dhcp[5] << 16) | (dhcp[6] << 8) | dhcp[7];

	// Skip to options
	int options_pos = 240; // DHCP header is 240 bytes

	// Check magic cookie
	__u32 magic_cookie = (dhcp[236] << 24) | (dhcp[237] << 16) | (dhcp[238] << 8) | dhcp[239];
	if (magic_cookie != DHCP_MAGIC_COOKIE) {
		increment_stat(STAT_INVALID_PACKETS_BLOCKED);
		return 0;
	}

	// Create DHCP key
	struct dhcp_key key = {};
	key.xid = xid;
	__builtin_memcpy(key.mac_addr, dhcp + 28, 6);
	key.pad = 0;

	// Get message type
	__u8 message_type = 0;
	parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_MESSAGE_TYPE, &message_type, 1);

	// Process based on message type
	if (message_type == 1) { // DHCP Discover
		struct dhcp_discover_info discover_info = {};
		discover_info.packet_size = skb->len;
		discover_info.timestamp = bpf_ktime_get_ns();
		__builtin_memcpy(discover_info.mac_addr, dhcp + 28, 6);

		// Extract hostname
		discover_info.hostname_len = parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_HOST_NAME,
		                                              (__u8 *)discover_info.hostname, 63);
		if (discover_info.hostname_len > 0) {
			discover_info.hostname[discover_info.hostname_len] = '\0';
		}

		// Extract vendor class
		discover_info.vendor_class_len = parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_VENDOR_CLASS,
		                                                (__u8 *)discover_info.vendor_class, 63);
		if (discover_info.vendor_class_len > 0) {
			discover_info.vendor_class[discover_info.vendor_class_len] = '\0';
		}

		// Store discover information
		if (bpf_map_update_elem(&dhcp_discovers, &key, &discover_info, BPF_ANY) == 0) {
			increment_stat(STAT_DISCOVERS_SEEN);

			// Send event to userspace
			send_dhcp_event(skb, &key, &discover_info, NULL, NULL, NULL, 1);
		} else {
			increment_stat(STAT_ERRORS);
		}

	} else if (message_type == 2) { // DHCP Offer
		struct dhcp_offer_info offer_info = {};
		offer_info.packet_size = skb->len;
		offer_info.timestamp = bpf_ktime_get_ns();

		// Extract your IP (yiaddr)
		offer_info.your_ip = *(__be32 *)(dhcp + 16);

		// Extract server IP from options
		__be32 server_ip = 0;
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_SERVER_ID,
		                  (__u8 *)&server_ip, 4);
		offer_info.server_ip = server_ip;

		// Extract subnet mask
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_SUBNET_MASK,
		                  (__u8 *)&offer_info.subnet_mask, 4);

		// Extract router
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_ROUTER,
		                  (__u8 *)&offer_info.router, 4);

		// Extract DNS servers
		__u8 dns_data[16];
		int dns_len = parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_DNS_SERVER,
		                               dns_data, 16);
		if (dns_len > 0) {
			__builtin_memcpy(offer_info.dns_servers, dns_data, dns_len);
		}

		// Extract lease time
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_LEASE_TIME,
		                  (__u8 *)&offer_info.lease_time, 4);

		// Store offer information
		if (bpf_map_update_elem(&dhcp_offers, &key, &offer_info, BPF_ANY) == 0) {
			increment_stat(STAT_OFFERS_SEEN);

			// Send event to userspace
			send_dhcp_event(skb, &key, NULL, &offer_info, NULL, NULL, 2);
		} else {
			increment_stat(STAT_ERRORS);
		}

	} else if (message_type == 3) { // DHCP Request
		struct dhcp_request_info request_info = {};
		request_info.packet_size = skb->len;
		request_info.timestamp = bpf_ktime_get_ns();
		__builtin_memcpy(request_info.mac_addr, dhcp + 28, 6);

		// Extract requested IP
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_REQUESTED_IP,
		                  (__u8 *)&request_info.requested_ip, 4);

		// Extract server IP
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_SERVER_ID,
		                  (__u8 *)&request_info.server_ip, 4);

		// Extract hostname
		request_info.hostname_len = parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_HOST_NAME,
		                                            (__u8 *)request_info.hostname, 63);
		if (request_info.hostname_len > 0) {
			request_info.hostname[request_info.hostname_len] = '\0';
		}

		// Store request information
		if (bpf_map_update_elem(&dhcp_requests, &key, &request_info, BPF_ANY) == 0) {
			increment_stat(STAT_REQUESTS_SEEN);

			// Send event to userspace
			send_dhcp_event(skb, &key, NULL, NULL, &request_info, NULL, 3);
		} else {
			increment_stat(STAT_ERRORS);
		}

	} else if (message_type == 5) { // DHCP ACK
		struct dhcp_ack_info ack_info = {};
		ack_info.packet_size = skb->len;
		ack_info.timestamp = bpf_ktime_get_ns();

		// Extract your IP (yiaddr)
		ack_info.your_ip = *(__be32 *)(dhcp + 16);

		// Extract server IP from options
		__be32 server_ip = 0;
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_SERVER_ID,
		                  (__u8 *)&server_ip, 4);
		ack_info.server_ip = server_ip;

		// Extract subnet mask
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_SUBNET_MASK,
		                  (__u8 *)&ack_info.subnet_mask, 4);

		// Extract router
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_ROUTER,
		                  (__u8 *)&ack_info.router, 4);

		// Extract DNS servers
		__u8 dns_data[16];
		int dns_len = parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_DNS_SERVER,
		                               dns_data, 16);
		if (dns_len > 0) {
			__builtin_memcpy(ack_info.dns_servers, dns_data, dns_len);
		}

		// Extract lease time
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_LEASE_TIME,
		                  (__u8 *)&ack_info.lease_time, 4);

		// Extract renewal time
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_RENEWAL_TIME,
		                  (__u8 *)&ack_info.renewal_time, 4);

		// Extract rebinding time
		parse_dhcp_options(dhcp, dhcp_len, &options_pos, DHCP_OPTION_REBINDING_TIME,
		                  (__u8 *)&ack_info.rebinding_time, 4);

		// Store ACK information
		if (bpf_map_update_elem(&dhcp_acks, &key, &ack_info, BPF_ANY) == 0) {
			increment_stat(STAT_ACKS_SEEN);

			// Send event to userspace
			send_dhcp_event(skb, &key, NULL, NULL, NULL, &ack_info, 4);
		} else {
			increment_stat(STAT_ERRORS);
		}
	}

	// Return 0 to pass packet, non-zero to drop
	// For monitoring, we typically want to pass the packet
	return 0;
}

char _license[] SEC("license") = "GPL";
