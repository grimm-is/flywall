#ifndef __FLOW_H__
#define __FLOW_H__

// Flow key structure for map lookups
struct flow_key {
	__u32 src_ip;
	__u32 dst_ip;
	__u16 src_port;
	__u16 dst_port;
	__u8 ip_proto;
	__u32 ifindex;
	__u8 padding[3];  // Align to 8 bytes
};

// Flow state structure
struct flow_state {
	__u64 first_seen;
	__u64 last_seen;
	__u64 packet_count;
	__u64 byte_count;
	__u32 verdict;
	__u32 offload_mark;
	__u32 qos_profile;  // QoS profile ID
	__u8 flags;
	__u8 padding[3];    // Align to 8 bytes
};

// Flow flags
#define FLOW_FLAG_ESTABLISHED  0x01
#define FLOW_FLAG_BIDIRECTIONAL 0x02
#define FLOW_FLAG_OFFLOADED    0x04

#endif /* __FLOW_H__ */
