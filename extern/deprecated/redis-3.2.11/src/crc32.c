#include <stdint.h>

static const uint32_t IEEE_POLY = 0xedb88320;

static uint32_t crc32tab[256];

static void
crc32_tabinit(uint32_t poly) {
    int i, j;
    for (i = 0; i < 256; i ++) {
        uint32_t crc = i;
        for (j = 0; j < 8; j ++) {
            if (crc & 1) {
                crc = (crc >> 1) ^ poly;
            } else {
                crc = (crc >> 1);
            }
        }
        crc32tab[i] = crc;
    }
}

void
crc32_init() {
    crc32_tabinit(IEEE_POLY);
}

static uint32_t
crc32_update(uint32_t crc, const char *buf, int len) {
    int i;
    crc = ~crc;
    for (i = 0; i < len; i ++) {
        crc = crc32tab[(uint8_t)((char)crc ^ buf[i])] ^ (crc >> 8);
    }
    return ~crc;
}

uint32_t
crc32_checksum(const char *buf, int len) {
    return crc32_update(0, buf, len);
}
