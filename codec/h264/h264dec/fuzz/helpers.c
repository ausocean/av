/*
DESCRIPTION
  helpers.c provides C helper functions for interfacing with C code used for
  testing in this package.

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

#include "helpers.h"

typedef struct BitReader BitReader;
typedef struct Reader Reader;

BitReader* new_BitReader(char* d, int l){
  Reader* r = (Reader*)malloc(sizeof(Reader));
  if(r == NULL){
    return NULL;
  }
  r->data = d;
  r->curr = 0;
  r->len = l;
  r->err = 0;

  BitReader* br = (BitReader*)malloc(sizeof(BitReader));
  if(br == NULL){
    return NULL;
  }
  br->r = r;
  br->n = 0;
  br->bits = 0;
  br->nRead = 0;
  br->err = 0;
  return br;
}

char next_byte(Reader *r) {
  if(r->curr >= r->len){
    r->err = 1;
  }
  char next = r->data[r->curr];
  r->curr++;
  return next;
}

uint64_t read_bits(BitReader *br, int n) {
  while( n > br->bits ){
    char b = next_byte(br->r);
    if(br->r->err != 0){
      br->err = 1;
      return 0;
    }
    br->nRead++;
    br->n <<= 8;
    br->n |= (uint64_t)b;
    br->bits += 8;
  }

  uint64_t r = (br->n >> (br->bits-n)) & ((1 << n) - 1);
  br->bits -= n;
  return r;
}
