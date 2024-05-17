/*
DESCRIPTION
  helpers.h defines some structs and function signatures that will help to
  interface with C code used in our fuzz testing. 

AUTHORS
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2024 the Australian Ocean Lab (AusOcean). All Rights Reserved. 

  The Software and all intellectual property rights associated
  therewith, including but not limited to copyrights, trademarks,
  patents, and trade secrets, are and will remain the exclusive
  property of the Australian Ocean Lab (AusOcean).
*/

#ifndef HELPERS_H
#define HELPERS_H

#include <stdlib.h>
#include <stdint.h>

struct Reader {
  char* data;
  int   curr;
  int   len;
  int   err;
};

struct BitReader {
  struct Reader*  r;
  uint64_t        n;
  int             bits;
  int             nRead;
  int             err;
};

// new_BitReader returns a new instance of a BitReader given the backing array
// d and the length of the data l.
struct BitReader* new_BitReader(char*, int);

// next_byte provides the next byte from a Reader r and advances it's byte index.
char next_byte(struct Reader*);

// read_bits intends to emulate the BitReader.ReadBits function defined in the
// bits package. This is used when a bit reader is required to obtain bytes from
// a stream in the C test code.
uint64_t read_bits(struct BitReader*, int);

#endif // HELPERS_H
