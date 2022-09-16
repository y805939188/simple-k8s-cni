#ifndef __section
# define __section(x)  __attribute__((section(x), used))
#endif

#define bpf_memcpy __builtin_memcpy

#define trace_printk(fmt, ...) do { \
	char _fmt[] = fmt; \
	bpf_trace_printk(_fmt, sizeof(_fmt), ##__VA_ARGS__); \
	} while (0)

#ifndef __packed
# define __packed		__attribute__((packed))
#endif

#ifndef __maybe_unused
# define __maybe_unused		__attribute__((__unused__))
#endif

#ifndef __section_maps_btf
# define __section_maps_btf		__section(".maps")
#endif

