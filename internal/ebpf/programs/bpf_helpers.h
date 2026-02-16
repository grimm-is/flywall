/* Minimal bpf_helpers.h for compilation */
#ifndef __BPF_HELPERS_H__
#define __BPF_HELPERS_H__

/* Include vmlinux.h first if available */
#ifdef __has_include
#if __has_include("vmlinux.h")
#include "vmlinux.h"
#endif
#endif

/* Map definitions that might not be in vmlinux.h */
#ifndef BPF_MAP_TYPE_HASH
#define BPF_MAP_TYPE_HASH 1
#endif

#ifndef BPF_MAP_TYPE_ARRAY
#define BPF_MAP_TYPE_ARRAY 2
#endif

#ifndef BPF_MAP_TYPE_PERCPU_ARRAY
#define BPF_MAP_TYPE_PERCPU_ARRAY 3
#endif

#ifndef BPF_MAP_TYPE_LRU_HASH
#define BPF_MAP_TYPE_LRU_HASH 9
#endif

#ifndef BPF_MAP_TYPE_RINGBUF
#define BPF_MAP_TYPE_RINGBUF 27
#endif

/* Map attributes */
#ifndef SEC
#define SEC(NAME) __attribute__((section(NAME), used))
#endif

#ifndef __uint
#define __uint(name, val) int (*name)[val]
#endif

#ifndef __type
#define __type(name, val) typeof(val) *name
#endif

#ifndef __array
#define __array(name, val) typeof(val) *name[]
#endif

/* Ringbuf helpers if not defined */
#ifndef bpf_ringbuf_output
static void *(*bpf_ringbuf_output)(void *ringbuf, void *data, __u64 size, __u64 flags) = (void *) 131;
#endif

#ifndef bpf_ringbuf_reserve
static void *(*bpf_ringbuf_reserve)(void *ringbuf, __u64 size, __u64 flags) = (void *) 132;
#endif

#ifndef bpf_ringbuf_submit
static void (*bpf_ringbuf_submit)(void *data, __u64 flags) = (void *) 133;
#endif

/* License */
char LICENSE[] SEC("license") = "GPL";

#endif /* __BPF_HELPERS_H__ */
