#include <stdint.h>

/* A famous generator polynomial which was selected by IEEE802.3, and has been
 * widely used in Ethernet, V.42, FDDI, Gzip, Zip, Png... */
static const uint32_t IEEE_POLY = 0xedb88320;

/* A table which cached the crc32 result for all the possible byte values. */
static uint32_t crc32tab[256];

/* Init the crc32tab with the given generator polynomial. */
static void crc32_tabinit(uint32_t poly) {
    int i, j;
    for (i = 0; i < 256; i++) {
        uint32_t crc = i;
        for (j = 0; j < 8; j++) {
            if (crc & 1) {
                crc = (crc >> 1) ^ poly;
            } else {
                crc = (crc >> 1);
            }
        }
        crc32tab[i] = crc;
    }
}

/* Update the crc32 result with the new data buffer. */
static uint32_t crc32_update(uint32_t crc, const char *buf, int len) {
    int i;
    crc = ~crc;
    for (i = 0; i < len; i++) {
        crc = crc32tab[(uint8_t)((char)crc ^ buf[i])] ^ (crc >> 8);
    }
    return ~crc;
}


/* Init the crc32tab with the widely used generator polynomial: IEEE_POLY. */
void crc32_init() {
    crc32_tabinit(IEEE_POLY);
}

/* Get the crc32 result of the given data buffer. */
uint32_t crc32_checksum(const char *buf, int len) {
    return crc32_update(0, buf, len);
}
