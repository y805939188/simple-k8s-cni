#include "stdio.h"
#include <net/if.h>

int main(void) {
  char *name = "veth3";
  int ifni = if_nametoindex(name);
  printf("这里的 index 是: %d\n", ifni);
}
