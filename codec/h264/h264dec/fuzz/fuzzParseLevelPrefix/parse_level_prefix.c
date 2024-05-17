/*
DESCRIPTION
  parse_level_prefix.c contains a function that will parse the level_prefix
  when performaing CAVLC decoding; extracted from Emeric Grange's h264 decoder
  contained in MiniVideo (https://github.com/emericg/MiniVideo). This is used
  to generate input and output data for testing purposes.

AUTHORS
  Emeric Grange <emeric.grange@gmail.com>
  Saxon A. Nelson-Milton <saxon@ausocean.org>

LICENSE
  COPYRIGHT (C) 2018 Emeric Grange - All Rights Reserved

  This file is part of MiniVideo.

  MiniVideo is free software: you can redistribute it and/or modify
  it under the terms of the GNU Lesser General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  MiniVideo is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU Lesser General Public License for more details.

  You should have received a copy of the GNU Lesser General Public License
  along with MiniVideo.  If not, see <http://www.gnu.org/licenses/>.
*/

#include "parse_level_prefix.h"
#include "../helpers.h"

/*!
 * \brief Parsing process for level_prefix.
 * \param *dc The current DecodingContext.
 * \return leadingZeroBits.
 *
 * From 'ITU-T H.264' recommendation:
 * 9.2.2.1 Parsing process for level_prefix
 *
 * The parsing process for this syntax element consists in reading the bits
 * starting at the current location in the bitstream up to and including the
 * first non-zero bit, and counting the number of leading bits that are equal to 0.
 *
 * level_prefix and level_suffix specify the value of a non-zero transform coefficient level.
 * The range of level_prefix and level_suffix is specified in subclause 9.2.2.
 */
int read_levelprefix(struct BitReader *br){
    int leadingZeroBits = -1;
    int b = 0;
    for (b = 0; !b; leadingZeroBits++){
        b = read_bits(br,1);
        if( br->err != 0 ){
          return -1;
        }
    }
    return leadingZeroBits;
}
