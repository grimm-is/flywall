#ifndef __COMMON_H__
#define __COMMON_H__

#include "flow.h"

// Common constants
#define MAX_PACKET_SIZE 1518  // Standard MTU + headers
#define FLOW_TIMEOUT_NS 300000000000ULL  // 5 minutes in nanoseconds

// Ethernet protocols
#define ETH_P_IP 0x0800
#define ETH_P_IPV6 0x86DD
#define ETH_P_ARP 0x0806

// IP protocols
#define IPPROTO_TCP 6
#define IPPROTO_UDP 17
#define IPPROTO_ICMP 1

// TC actions
#define TC_ACT_OK 0
#define TC_ACT_SHOT -2
#define TC_ACT_REDIRECT 7

// XDP actions
#define XDP_ABORTED 0
#define XDP_DROP 1
#define XDP_PASS 2
#define XDP_TX 3
#define XDP_REDIRECT 4

// Flow flags
#define FLOW_FLAG_ESTABLISHED  0x01
#define FLOW_FLAG_BIDIRECTIONAL 0x02
#define FLOW_FLAG_OFFLOADED    0x04

// QoS profile IDs
#define QOS_PROFILE_DEFAULT   0
#define QOS_PROFILE_BULK      1
#define QOS_PROFILE_INTERACTIVE 2
#define QOS_PROFILE_VIDEO     3
#define QOS_PROFILE_VOICE     4
#define QOS_PROFILE_CRITICAL  5

#endif /* __COMMON_H__ */
