#include <linux/bpf.h>
#include "ctx.h"
#include "compiler.h"
// #include "helpers_skb.h"

// #ifndef __BPF_HELPERS__
// #define __BPF_HELPERS__
#ifndef BPF_FUNC
# define BPF_FUNC(NAME, ...)						\
	(* NAME)(__VA_ARGS__) __maybe_unused = (void *)BPF_FUNC_##NAME
#endif

/* Map access/manipulation */
static void *BPF_FUNC(
  map_lookup_elem, const void *map, const void *key
);
static int BPF_FUNC(
  map_update_elem,
  const void *map,
  const void *key,
	const void *value,
  __u32 flags
);
static int BPF_FUNC(
  map_delete_elem,
  const void *map,
  const void *key
);
