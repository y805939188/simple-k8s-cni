#include <linux/bpf.h>

#ifndef __maybe_unused
# define __maybe_unused		__attribute__((__unused__))
#endif
#define __always_inline		inline __attribute__((always_inline))

static __always_inline __maybe_unused void
ctx_store_meta(struct __sk_buff *ctx, const __u32 off, __u32 data) {
	ctx->cb[off] = data;
}

static __always_inline __maybe_unused __u32
ctx_load_meta(const struct __sk_buff *ctx, const __u32 off) {
	return ctx->cb[off];
}
