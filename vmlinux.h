/* SPDX-License-Identifier: (LGPL-2.1 OR BSD-2-Clause) */
/* This is a minimal vmlinux.h for development purposes */
#ifndef __VMLINUX_H__
#define __VMLINUX_H__

/* Basic type definitions */
typedef __u8  u8;
typedef __u16 u16;
typedef __u32 u32;
typedef __u64 u64;

typedef __u8 __u8;
typedef __u16 __u16;
typedef __u32 __u32;
typedef __u64 __u64;

typedef signed char s8;
typedef short s16;
typedef int s32;
typedef long long s64;

typedef signed char __s8;
typedef short __s16;
typedef int __s32;
typedef long long __s64;

/* Basic constants */
#define ETH_P_IP   0x0800
#define ETH_P_IPV6 0x86DD
#define ETH_P_ARP  0x0806

#define IPPROTO_TCP 6
#define IPPROTO_UDP 17
#define IPPROTO_ICMP 1

/* Structure definitions */
struct ethhdr {
	unsigned char h_dest[6];
	unsigned char h_source[6];
	unsigned short h_proto;
};

struct iphdr {
	__u8 ihl:4;
	__u8 version:4;
	__u8 tos;
	__u16 tot_len;
	__u16 id;
	__u16 frag_off;
	__u8 ttl;
	__u8 protocol;
	__u16 check;
	__u32 saddr;
	__u32 daddr;
};

struct tcphdr {
	__u16 source;
	__u16 dest;
	__u32 seq;
	__u32 ack_seq;
	__u16 res1:4;
	__u16 doff:4;
	__u16 fin:1;
	__u16 syn:1;
	__u16 rst:1;
	__u16 psh:1;
	__u16 ack:1;
	__u16 urg:1;
	__u16 ece:1;
	__u16 cwr:1;
	__u16 window;
	__u16 check;
	__u16 urg_ptr;
};

struct udphdr {
	__u16 source;
	__u16 dest;
	__u16 len;
	__u16 check;
};

typedef struct {
	__u32 ip;
	__u32 ifindex;
} bpf_sock_tuple;

struct bpf_sock {
	__u32 bound_dev_if;
	__u32 family;
	__u32 type;
	__u32 protocol;
	__u32 mark;
	__u32 priority;
	__u32 src_ip4;
	__u32 src_ip6[4];
	__u32 src_port;
	__u32 dst_ip4;
	__u32 dst_ip6[4];
	__u32 dst_port;
	__u32 state;
	__u32 rtt_min;
	__u32 rtt_avg;
	__u32 mss_cache;
	__u32 cn_probe;
	__u32 syn_retries;
	__u32 is_fullsock;
	__u32 len;
	__u32 snd_cwnd;
	__u32 sndbuf;
	__u32 txmsg_flags;
	__u32 rx_queue_len;
};

struct __sk_buff {
	__u32 len;
	__u32 pkt_type;
	__u32 hash;
	__u32 queue_mapping;
	__u32 protocol;
	__u32 vlan_present;
	__u32 vlan_tci;
	__u32 vlan_proto;
	__u32 priority;
	__u32 ingress_ifindex;
	__u32 ifindex;
	__u32 tc_index;
	__u32 cb[5];
	__u32 hash;
	__u32 tc_classid;
	__u32 data;
	__u32 data_end;
	__u32 napi_id;
	__u32 family;
	__u32 remote_ip4;
	__u32 local_ip4;
	__u32 remote_ip6[4];
	__u32 local_ip6[4];
	__u32 remote_port;
	__u32 local_port;
	__u32 data_meta;
	union {
		struct bpf_flow_keys *flow_keys;
	};
	__u64 tstamp;
	__u32 wire_len;
	__u32 gso_segs;
	__u32 gso_size;
	__u32 tstamp_type;
	__u32 hwtstamp;
};

struct bpf_tcp_sock {
	__u32 snd_cwnd;
	__u32 srtt_us;
	__u32 rtt_min;
	__u32 snd_ssthresh;
	__u32 rcv_nxt;
	__u32 snd_nxt;
	__u32 snd_una;
	__u32 mss_cache;
	__u32 ecn_flags;
	__u32 rate_delivered;
	__u32 rate_interval_us;
	__u32 packets_out;
	__u32 retrans_out;
	__u32 total_retrans;
	__u32 segs_in;
	__u32 data_segs_in;
	__u32 segs_out;
	__u32 data_segs_out;
	__u32 lost_out;
	__u32 sacked_out;
	__u32 bytes_received;
	__u32 bytes_acked;
	__u32 dsack_dups;
	__u32 delivered;
	__u32 delivered_ce;
	__u32 icsk_retransmits;
};

struct bpf_sock_ops {
	__u32 op;
	union {
		__u32 args[4];
		__u32 reply;
		__u32 replylong[4];
	};
	__u32 family;
	__u32 remote_ip4;
	__u32 remote_ip6[4];
	__u32 local_ip4;
	__u32 local_ip6[4];
	__u32 remote_port;
	__u32 local_port;
	__u32 is_fullsock;
	__u32 snd_cwnd;
	__u32 srtt_us;
	__u32 bpf_sock_ops_cb_flags;
	__u32 state;
	__u32 rtt_min;
	__u32 snd_ssthresh;
	__u32 rcv_nxt;
	__u32 snd_nxt;
	__u32 snd_una;
	__u32 mss_cache;
	__u32 ecn_flags;
	__u32 rate_delivered;
	__u32 rate_interval_us;
	__u32 packets_out;
	__u32 retrans_out;
	__u32 total_retrans;
	__u32 segs_in;
	__u32 data_segs_in;
	__u32 segs_out;
	__u32 data_segs_out;
	__u32 lost_out;
	__u32 sacked_out;
	__u32 bytes_received;
	__u32 bytes_acked;
	__u32 dsack_dups;
	__u32 delivered;
	__u32 delivered_ce;
	__u32 icsk_retransmits;
};

struct bpf_perf_event_data {
	__u64 sample_period;
	__u64 sample_type;
	__u64 config;
	__u64 kprobe_func;
	__u64 kprobe_addr;
	__u64 retval;
	__u64 ctx;
	__u64 data_event_size;
	__u64 data_event_offset;
};

struct bpf_perf_event_value {
	__u64 counter;
	__u64 enabled;
	__u64 running;
};

struct bpf_map_def {
	unsigned int type;
	unsigned int key_size;
	unsigned int value_size;
	unsigned int max_entries;
	unsigned int map_flags;
};

#define SEC(NAME) __attribute__((section(NAME), used))
#define __always_inline inline __attribute__((always_inline))
#define __weak __attribute__((weak))

#define __uint(name, val) int (*name)[val]
#define __type(name, val) typeof(val) *name
#define __array(name, val) typeof(val) *name[]
#define __builtin_preserve_access_index(val) val

#define BPF_ANNOTATE_KV_PAIR(name, key_type, val_type) \
	struct ____bpf_map_##name { \
		__uint(key_size, sizeof(key_type)); \
		__uint(value_size, sizeof(val_type)); \
	} __attribute__((section(".maps." #name), used))

#define BPF_SEQ_DECLARE(seq) \
	struct seq; \
	static __always_inline void seq_##seq##_new(struct seq *seq) {} \
	static __always_inline void seq_##seq##_next(struct seq *seq) {} \
	static __always_inline void seq_##seq##_delete(struct seq *seq) {}

#define bpf_ksym_exists(name) 0
#define bpf_core_type_exists(type) 0
#define bpf_core_field_exists(type, field) 0

#define bpf_probe_read_kernel(dest, sz, src) \
	bpf_probe_read((dest), (sz), (src))
#define bpf_probe_read_kernel_str(dest, sz, src) \
	bpf_probe_read_str((dest), (sz), (src))
#define bpf_probe_read_user(dest, sz, src) \
	bpf_probe_read((dest), (sz), (src))
#define bpf_probe_read_user_str(dest, sz, src) \
	bpf_probe_read_str((dest), (sz), (src))

#define bpf_get_current_pid_tgid() bpf_get_prandom_u32()
#define bpf_get_current_uid_gid() bpf_get_prandom_u32()
#define bpf_get_current_comm(dest, sz) ({ \
	__builtin_memset((dest), 0, (sz)); \
	0; \
})

#define bpf_spin_lock(lock) ({ 0; })
#define bpf_spin_unlock(lock) ({ 0; })

#define bpf_for_each_map_elem(key, val, map, ctx) ({ 0; })
#define bpf_for_each_map_key(key, map, ctx) ({ 0; })
#define bpf_for_each_map_elem_and_req(key, val, map, ctx, req) ({ 0; })

#define bpf_csum_diff(from, from_size, to, to_size, seed) ({ (seed); })
#define bpf_csum_diff(from, from_size, to, to_size, seed) ({ (seed); })

#define bpf_ktime_get_boot_ns() bpf_ktime_get_ns()
#define bpf_ktime_get_coarse_ns() bpf_ktime_get_ns()
#define bpf_ktime_get_tai_ns() bpf_ktime_get_ns()
#define bpf_ktime_get_real_ns() bpf_ktime_get_ns()

#define bpf_jiffies64() bpf_ktime_get_ns()
#define bpf_jiffies() (bpf_ktime_get_ns() / 1000000000ULL)

#define bpf_get_numa_node_id() 0
#define bpf_get_socket_cookie(skb) bpf_get_prandom_u32()
#define bpf_get_task_cookie() bpf_get_prandom_u32()

#define bpf_get_current_cgroup_id() ({ \
	__u64 __id = 0; \
	__id; \
})

#define bpf_get_current_ancestor_cgroup_id(level) ({ \
	__u64 __id = 0; \
	__id; \
})

#define bpf_ringbuf_output(ctx, data, size, flags) ({ 0; })
#define bpf_ringbuf_reserve(ctx, size, flags) ({ NULL; })
#define bpf_ringbuf_submit(data, flags) ({ })
#define bpf_ringbuf_discard(data, flags) ({ })
#define bpf_ringbuf_query(ctx, flags) ({ 0; })

#define bpf_skc_to_tcp_sock(sk) ({ NULL; })
#define bpf_skc_to_tcp_timewait_sock(sk) ({ NULL; })
#define bpf_skc_to_tcp_request_sock(sk) ({ NULL; })
#define bpf_skc_to_udp_sock(sk) ({ NULL; })

#define bpf_timer_init(timer, map, flags) ({ -ENOSYS; })
#define bpf_timer_set_callback(timer, callback_fn) ({ -ENOSYS; })
#define bpf_timer_start(timer, nsecs, flags) ({ -ENOSYS; })
#define bpf_timer_cancel(timer) ({ -ENOSYS; })

#define bpf_get_func_ip(ctx) ({ 0; })
#define bpf_get_attach_cookie(ctx) ({ 0; })
#define bpf_get_task_under_cgroup(ctx) ({ 0; })
#define bpf_find_vma(ctx, address, callback_ctx) ({ -ENOSYS; })

#define bpf_kptr_xchg(ptr, val) ({ NULL; })

#define bpf_dynptr_from_mem(data, size, flags, ptr__dynptr) ({ -ENOSYS; })
#define bpf_dynptr_read(data, size, ptr__dynptr, offset) ({ -ENOSYS; })
#define bpf_dynptr_write(ptr__dynptr, offset, data, size) ({ -ENOSYS; })
#define bpf_dynptr_data(ptr__dynptr, offset) ({ NULL; })

#define bpf_get_netns_cookie(ctx) ({ 0; })
#define bpf_get_current_task_btf() ({ NULL; })
#define bpf_rbtree_add(node, root, flags) ({ -ENOSYS; })
#define bpf_rbtree_first(root) ({ NULL; })
#define bpf_rbtree_remove(root, node) ({ NULL; })
#define bpf_rbtree_release(node) ({ })

#define bpf_list_push_front(node, head, flags) ({ -ENOSYS; })
#define bpf_list_push_back(node, head, flags) ({ -ENOSYS; })
#define bpf_list_pop_front(head, flags) ({ NULL; })
#define bpf_list_pop_back(head, flags) ({ NULL; })

#define bpf_mptcp_sock(sk) ({ NULL; })

#define bpf_ktime_get_tai_ns() bpf_ktime_get_ns()

/* Helper functions for eBPF */
static __always_inline void *bpf_map_lookup_elem(void *map, const void *key) {
	return NULL;
}

static __always_inline long bpf_map_update_elem(void *map, const void *key, const void *value, unsigned long flags) {
	return 0;
}

static __always_inline long bpf_map_delete_elem(void *map, const void *key) {
	return 0;
}

static __always_inline long bpf_probe_read(void *dst, __u32 size, const void *unsafe_ptr) {
	return 0;
}

static __always_inline long bpf_probe_read_str(void *dst, __u32 size, const void *unsafe_ptr) {
	return 0;
}

static __always_inline __u64 bpf_ktime_get_ns(void) {
	return 0;
}

static __always_inline __u32 bpf_get_prandom_u32(void) {
	return 0;
}

static __always_inline void bpf_tail_call(void *ctx, void *prog_array_map, __u32 index) {
}

static __always_inline long bpf_clone_redirect(void *ctx, __u32 ifindex, __u32 flags) {
	return 0;
}

static __always_inline long bpf_redirect(__u32 ifindex, __u32 flags) {
	return 0;
}

static __always_inline long bpf_redirect_map(void *map, __u32 key, __u32 flags) {
	return 0;
}

static __always_inline long bpf_redirect_hash(void *map, void *key, __u32 flags) {
	return 0;
}

static __always_inline long bpf_redirect_neigh(void *map, void *key, __u32 flags) {
	return 0;
}

static __always_inline long bpf_redirect_peer(__u32 ifindex, __u32 flags) {
	return 0;
}

static __always_inline long bpf_perf_event_output(void *ctx, void *map, __u64 flags, void *data, __u64 size) {
	return 0;
}

static __always_inline long bpf_get_stackid(void *ctx, void *map, __u64 flags) {
	return 0;
}

static __always_inline long bpf_get_stack(void *ctx, void *buf, __u32 size, __u64 flags) {
	return 0;
}

static __always_inline long bpf_csum_diff(__be32 *from, __u32 from_size, __be32 *to, __u32 to_size, __be32 seed) {
	return 0;
}

static __always_inline __u64 bpf_get_current_pid_tgid(void) {
	return 0;
}

static __always_inline __u64 bpf_get_current_uid_gid(void) {
	return 0;
}

static __always_inline long bpf_get_current_comm(void *buf, __u32 size_of_buf) {
	return 0;
}

static __always_inline long bpf_get_socket_cookie(void *ctx) {
	return 0;
}

static __always_inline long bpf_get_socket_uid(void *ctx) {
	return 0;
}

static __always_inline long bpf_get_hash_recalc(void *ctx) {
	return 0;
}

static __always_inline long bpf_set_hash_invalid(void *ctx) {
	return 0;
}

static __always_inline long bpf_setsockopt(void *ctx, __u32 level, __u32 optname, void *optval, __u32 optlen) {
	return 0;
}

static __always_inline long bpf_getsockopt(void *ctx, __u32 level, __u32 optname, void *optval, __u32 optlen) {
	return 0;
}

static __always_inline long bpf_sock_ops_cb_flags_set(void *ctx, __u32 arg, __u64 flags) {
	return 0;
}

static __always_inline long bpf_msg_redirect_map(void *msg, void *map, __u32 key, __u64 flags) {
	return 0;
}

static __always_inline long bpf_msg_redirect_hash(void *msg, void *map, void *key, __u64 flags) {
	return 0;
}

static __always_inline long bpf_msg_apply_bytes(void *msg, __u32 bytes) {
	return 0;
}

static __always_inline long bpf_msg_pull_data(void *msg, __u32 start, __u32 end, __u64 flags) {
	return 0;
}

static __always_inline long bpf_bind(void *ctx, void *addr, int addr_len) {
	return 0;
}

static __always_inline long bpf_xdp_adjust_meta(void *ctx, int delta) {
	return 0;
}

static __always_inline long bpf_xdp_adjust_head(void *ctx, int delta) {
	return 0;
}

static __always_inline long bpf_xdp_adjust_tail(void *ctx, int delta) {
	return 0;
}

static __always_inline long bpf_probe_read_user(void *dst, __u32 size, const void *unsafe_ptr) {
	return 0;
}

static __always_inline long bpf_probe_read_user_str(void *dst, __u32 size, const void *unsafe_ptr) {
	return 0;
}

static __always_inline long bpf_probe_read_kernel(void *dst, __u32 size, const void *unsafe_ptr) {
	return 0;
}

static __always_inline long bpf_probe_read_kernel_str(void *dst, __u32 size, const void *unsafe_ptr) {
	return 0;
}

static __always_inline long bpf_get_socket_cookie(struct bpf_sock_addr *ctx) {
	return 0;
}

static __always_inline long bpf_get_socket_cookie(struct bpf_sock_ops *ctx) {
	return 0;
}

static __always_inline long bpf_getsockopt(struct bpf_sock_ops *ctx, __u32 level, __u32 optname, void *optval, __u32 optlen) {
	return 0;
}

static __always_inline long bpf_setsockopt(struct bpf_sock_ops *ctx, __u32 level, __u32 optname, void *optval, __u32 optlen) {
	return 0;
}

static __always_inline long bpf_sock_ops_cb_flags_set(struct bpf_sock_ops *ctx, __u32 arg, __u64 flags) {
	return 0;
}

static __always_inline long bpf_getsockopt(void *ctx, __u32 level, __u32 optname, void *optval, __u32 optlen) {
	return 0;
}

static __always_inline long bpf_setsockopt(void *ctx, __u32 level, __u32 optname, void *optval, __u32 optlen) {
	return 0;
}

/* Map pinning */
#define LIBBPF_PIN_BY_NAME (1 << 0)

/* Map update flags */
#define BPF_ANY     0 /* create new element or update existing */
#define BPF_NOEXIST 1 /* create new element if it didn't exist */
#define BPF_EXIST   2 /* update existing element */
#define BPF_F_LOCK  4 /* spin_lock-ed mapupdate/map_delete */

/* TC return codes */
#define TC_ACT_OK		0
#define TC_ACT_SHOT		-2
#define TC_ACT_STOLEN		4
#define TC_ACT_REDIRECT		7
#define TC_ACT_UNSPEC		-1
#define TC_ACT_PIPE		3
#define TC_ACT_RECLASSIFY	1
#define TC_ACT_QUEUE		5

#endif /* __VMLINUX_H__ */