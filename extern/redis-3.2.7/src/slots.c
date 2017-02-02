#include "server.h"

extern void createDumpPayload(rio *payload, robj *o);
extern int verifyDumpPayload(unsigned char *p, size_t len);

static void *
slots_tag(const sds s, int *plen) {
    int i, j, n = sdslen(s);
    for (i = 0; i < n && s[i] != '{'; i ++) {}
    if (i == n) {
        return NULL;
    }
    i ++;
    for (j = i; j < n && s[j] != '}'; j ++) {}
    if (j == n) {
        return NULL;
    }
    if (plen != NULL) {
        *plen = j - i;
    }
    return s + i;
}

int
slots_num(const sds s, uint32_t *pcrc, int *phastag) {
    int taglen;
    int hastag = 0;
    void *tag = slots_tag(s, &taglen);
    if (tag == NULL) {
        tag = s, taglen = sdslen(s);
    } else {
        hastag = 1;
    }
    uint32_t crc = crc32_checksum(tag, taglen);
    if (pcrc != NULL) {
        *pcrc = crc;
    }
    if (phastag != NULL) {
        *phastag = hastag;
    }
    return crc & HASH_SLOTS_MASK;
}

static int
parse_int(client *c, robj *obj, int *p) {
    long v;
    if (getLongFromObjectOrReply(c, obj, &v, NULL) != C_OK) {
        return -1;
    }
    if (v < INT_MIN || v > INT_MAX) {
        addReplyError(c, "value is out of range");
        return -1;
    } else {
        *p = v;
        return 0;
    }
}

static int
parse_timeout(client *c, robj *obj, int *p) {
    int v;
    if (parse_int(c, obj, &v) != 0) {
        return -1;
    }
    if (v < 0) {
        addReplyErrorFormat(c, "invalid timeout = %d", v);
        return -1;
    }
    *p = (v == 0) ? 100 : v;
    return 0;
}

static int
parse_slot(client *c, robj *obj, int *p) {
    int v;
    if (parse_int(c, obj, &v) != 0) {
        return -1;
    }
    if (v < 0 || v >= HASH_SLOTS_SIZE) {
        addReplyErrorFormat(c, "invalid slot number = %d", v);
        return -1;
    }
    *p = v;
    return 0;
}

/* *
 * slotshashkey [key1 key2...]
 * */
void
slotshashkeyCommand(client *c) {
    int i;
    addReplyMultiBulkLen(c, c->argc - 1);
    for (i = 1; i < c->argc; i ++) {
        robj *key = c->argv[i];
        addReplyLongLong(c, slots_num(key->ptr, NULL, NULL));
    }
}

/* *
 * slotsinfo [start] [count]
 * */
void
slotsinfoCommand(client *c) {
    int slots_slot[HASH_SLOTS_SIZE];
    int slots_size[HASH_SLOTS_SIZE];
    int n = 0, beg = 0, end = HASH_SLOTS_SIZE;
    if (c->argc >= 2) {
        if (parse_slot(c, c->argv[1], &beg) != 0) {
            return;
        }
    }
    if (c->argc >= 3) {
        int v;
        if (parse_int(c, c->argv[2], &v) != 0) {
            return;
        }
        if (v < 0) {
            addReplyErrorFormat(c, "invalid slot count = %d", v);
            return;
        }
        if (beg + v < end) {
            end = beg + v;
        }
    }
    if (c->argc >= 4) {
        addReplyErrorFormat(c, "wrong number of arguments for 'slotsinfo' command");
        return;
    }
    int i;
    for (i = beg; i < end; i ++) {
        int s = dictSize(c->db->hash_slots[i]);
        if (s == 0) {
            continue;
        }
        slots_slot[n] = i;
        slots_size[n] = s;
        n ++;
    }
    addReplyMultiBulkLen(c, n);
    for (i = 0; i < n; i ++) {
        addReplyMultiBulkLen(c, 2);
        addReplyLongLong(c, slots_slot[i]);
        addReplyLongLong(c, slots_size[i]);
    }
}

typedef struct {
    int fd;
    int db;
    int authorized;
    time_t lasttime;
} slotsmgrt_sockfd;

static slotsmgrt_sockfd *
slotsmgrt_get_sockfd(client *c, sds host, sds port, int timeout) {
    sds name = sdsempty();
    name = sdscatlen(name, host, sdslen(host));
    name = sdscatlen(name, ":", 1);
    name = sdscatlen(name, port, sdslen(port));

    slotsmgrt_sockfd *pfd = dictFetchValue(server.slotsmgrt_cached_sockfds, name);
    if (pfd != NULL) {
        sdsfree(name);
        pfd->lasttime = server.unixtime;
        return pfd;
    }

    int fd = anetTcpNonBlockConnect(server.neterr, host, atoi(port));
    if (fd == -1) {
        serverLog(LL_WARNING, "slotsmgrt: connect to target %s:%s, error = '%s'",
                host, port, server.neterr);
        sdsfree(name);
        addReplyErrorFormat(c,"Can't connect to target node: %s", server.neterr);
        return NULL;
    }
    anetEnableTcpNoDelay(server.neterr, fd);
    if ((aeWait(fd, AE_WRITABLE, timeout) & AE_WRITABLE) == 0) {
        serverLog(LL_WARNING, "slotsmgrt: connect to target %s:%s, aewait error = '%s'",
                host, port, server.neterr);
        sdsfree(name);
        close(fd);
        addReplySds(c, sdsnew("-IOERR error or timeout connecting to the client\r\n"));
        return NULL;
    }
    serverLog(LL_WARNING, "slotsmgrt: connect to target %s:%s", host, port);

    pfd = zmalloc(sizeof(*pfd));
    pfd->fd = fd;
    pfd->db = -1;
    pfd->authorized = (server.requirepass == NULL) ? 1 : 0;
    pfd->lasttime = server.unixtime;
    dictAdd(server.slotsmgrt_cached_sockfds, name, pfd);
    return pfd;
}

static void
slotsmgrt_close_socket(sds host, sds port) {
    sds name = sdsempty();
    name = sdscatlen(name, host, sdslen(host));
    name = sdscatlen(name, ":", 1);
    name = sdscatlen(name, port, sdslen(port));

    slotsmgrt_sockfd *pfd = dictFetchValue(server.slotsmgrt_cached_sockfds, name);
    if (pfd == NULL) {
        serverLog(LL_WARNING, "slotsmgrt: close target %s:%s again", host, port);
        sdsfree(name);
        return;
    } else {
        serverLog(LL_WARNING, "slotsmgrt: close target %s:%s", host, port);
    }
    dictDelete(server.slotsmgrt_cached_sockfds, name);
    close(pfd->fd);
    zfree(pfd);
    sdsfree(name);
}

void
slotsmgrt_cleanup() {
    dictIterator *di = dictGetSafeIterator(server.slotsmgrt_cached_sockfds);
    dictEntry *de;
    while((de = dictNext(di)) != NULL) {
        slotsmgrt_sockfd *pfd = dictGetVal(de);
        if ((server.unixtime - pfd->lasttime) > 15) {
            serverLog(LL_WARNING, "slotsmgrt: timeout target %s, lasttime = %ld, now = %ld",
                   (char *)dictGetKey(de), pfd->lasttime, server.unixtime);
            dictDelete(server.slotsmgrt_cached_sockfds, dictGetKey(de));
            close(pfd->fd);
            zfree(pfd);
        }
    }
    dictReleaseIterator(di);
}

static int
slotsmgrt(client *c, sds host, sds port, slotsmgrt_sockfd *pfd, int db, int timeout, robj *keys[], robj *vals[], int n) {
    rio cmd;
    rioInitWithBuffer(&cmd, sdsempty());

    int needauth = 0;
    if (pfd->authorized == 0 && server.requirepass != NULL) {
        needauth = 1;
        serverAssertWithInfo(c, NULL, rioWriteBulkCount(&cmd, '*', 2));
        serverAssertWithInfo(c, NULL, rioWriteBulkString(&cmd, "AUTH", 4));
        serverAssertWithInfo(c, NULL, rioWriteBulkString(&cmd, server.requirepass, strlen(server.requirepass)));
    }

    int selectdb = 0;
    if (pfd->db != db) {
        selectdb = 1;
        serverAssertWithInfo(c, NULL, rioWriteBulkCount(&cmd, '*', 2));
        serverAssertWithInfo(c, NULL, rioWriteBulkString(&cmd, "SELECT", 6));
        serverAssertWithInfo(c, NULL, rioWriteBulkLongLong(&cmd, db));
    }

    serverAssertWithInfo(c, NULL, rioWriteBulkCount(&cmd, '*', 1 + 3 * n));
    serverAssertWithInfo(c, NULL, rioWriteBulkString(&cmd, "SLOTSRESTORE", 12));

    sds onekey = NULL;
    for (int i = 0; i < n; i ++) {
        robj *key = keys[i], *val = vals[i];
        long long ttl = 0, expireat = getExpire(c->db, key);
        if (expireat != -1) {
            ttl = expireat - mstime();
            if (ttl < 1) {
                ttl = 1;
            }
        }
        sds skey = key->ptr;
        serverAssertWithInfo(c, NULL, rioWriteBulkString(&cmd, skey, sdslen(skey)));
        serverAssertWithInfo(c, NULL, rioWriteBulkLongLong(&cmd, ttl));
        do {
            rio pld;
            createDumpPayload(&pld, val);
            sds buf = pld.io.buffer.ptr;
            serverAssertWithInfo(c, NULL, rioWriteBulkString(&cmd, buf, sdslen(buf)));
            sdsfree(buf);
        } while (0);
        if (onekey == NULL) {
            onekey = skey;
        }
    }

    do {
        sds buf = cmd.io.buffer.ptr;
        size_t pos = 0, towrite;
        int nwritten = 0;
        while ((towrite = sdslen(buf) - pos) > 0) {
            towrite = (towrite > (64 * 1024) ? (64 * 1024) : towrite);
            nwritten = syncWrite(pfd->fd, buf + pos, towrite, timeout);
            if (nwritten != (signed)towrite) {
                serverLog(LL_WARNING, "slotsmgrt: writing to target %s:%s, error '%s', "
                        "nkeys = %d, onekey = '%s', cmd.len = %ld, pos = %ld, towrite = %ld",
                        host, port, server.neterr, n, onekey, sdslen(buf), pos, towrite);
                addReplySds(c, sdsnew("-IOERR error or timeout writing to target\r\n"));
                sdsfree(buf);
                return -1;
            }
            pos += nwritten;
        }
        sdsfree(buf);
    } while (0);

    do {
        char buf[1024];
        if (needauth) {
            if (syncReadLine(pfd->fd, buf, sizeof(buf), timeout) <= 0) {
                serverLog(LL_WARNING, "slotsmgrt: auth failed, reading from target %s:%s: nkeys = %d, onekey = '%s', error = '%s'",
                        host, port, n, onekey, server.neterr);
                addReplySds(c, sdsnew("-IOERR error or timeout reading from target\r\n"));
                return -1;
            }
            if (buf[0] != '+') {
                serverLog(LL_WARNING, "slotsmgrt: auth failed, reading from target %s:%s: nkeys = %d, onekey = '%s', response = '%s'",
                        host, port, n, onekey, buf);
                addReplyError(c, "error on slotsrestore, auth failed");
                return -1;
            }
            pfd->authorized = 1;
        }

        if (selectdb) {
            if (syncReadLine(pfd->fd, buf, sizeof(buf), timeout) <= 0) {
                serverLog(LL_WARNING, "slotsmgrt: select failed, reading from target %s:%s: nkeys = %d, onekey = '%s', error = '%s'",
                        host, port, n, onekey, server.neterr);
                addReplySds(c, sdsnew("-IOERR error or timeout reading from target\r\n"));
                return -1;
            }
            if (buf[0] != '+') {
                serverLog(LL_WARNING, "slotsmgrt: select failed, reading from target %s:%s: nkeys = %d, onekey = '%s', response = '%s'",
                        host, port, n, onekey, buf);
                addReplyError(c, "error on slotsrestore, select failed");
                return -1;
            }
            pfd->db = db;
        }

        if (syncReadLine(pfd->fd, buf, sizeof(buf), timeout) <= 0) {
            serverLog(LL_WARNING, "slotsmgrt: migration failed, reading from target %s:%s: nkeys = %d, onekey = '%s', error = '%s'",
                    host, port, n, onekey, server.neterr);
            addReplySds(c, sdsnew("-IOERR error or timeout reading from target\r\n"));
            return -1;
        }
        if (buf[0] == '-') {
            serverLog(LL_WARNING, "slotsmgrt: migration failed, reading from target %s:%s: nkeys = %d, onekey = '%s', response = '%s'",
                    host, port, n, onekey, buf);
            addReplyError(c, "error on slotsrestore, migration failed");
            return -1;
        }
    } while (0);

    pfd->lasttime = server.unixtime;

    serverLog(LL_VERBOSE, "slotsmgrt: migrate to %s:%s, nkeys = %d, onekey = '%s'", host, port, n, onekey);
    return 0;
}

static void
slotsremove(client *c, robj **keys, int n, int rewrite) {
    for (int i = 0; i < n; i ++) {
        dbDelete(c->db, keys[i]);
        signalModifiedKey(c->db, keys[i]);
        server.dirty ++;
    }
    if (!rewrite) {
        return;
    }
    for (int i = 0; i < n; i ++) {
        incrRefCount(keys[i]);
    }
    for (int i = 0; i < c->argc; i ++) {
        decrRefCount(c->argv[i]);
    }
    zfree(c->argv);
    c->argc = n + 1;
    c->argv = zmalloc(sizeof(robj *) * c->argc);
    c->argv[0] = createStringObject("DEL", 3);
    for (int i = 0; i < n; i ++) {
        c->argv[i + 1] = keys[i];
    }
    c->cmd = lookupCommandOrOriginal(c->argv[0]->ptr);
    serverAssertWithInfo(c, NULL, c->cmd != NULL);
}

/* *
 * do migrate a key-value for slotsmgrt/slotsmgrtone commands
 * return value:
 *    -1 - error happens
 *   >=0 - # of success migration (0 or 1)
 * */
static int
slotsmgrtone_command(client *c, sds host, sds port, int timeout, robj *key) {
    slotsmgrt_sockfd *pfd = slotsmgrt_get_sockfd(c, host, port, timeout);
    if (pfd == NULL) {
        return -1;
    }

    robj *val = lookupKeyWrite(c->db, key);
    if (val == NULL) {
        return 0;
    }
    robj *keys[] = {key};
    robj *vals[] = {val};
    if (slotsmgrt(c, host, port, pfd, c->db->id, timeout, keys, vals, 1) != 0) {
        slotsmgrt_close_socket(host, port);
        return -1;
    }
    slotsremove(c, keys, 1, 1);
    return 1;
}

/* *
 * slotsmgrtslot host port timeout slot
 * */
void
slotsmgrtslotCommand(client *c) {
    sds host = c->argv[1]->ptr;
    sds port = c->argv[2]->ptr;
    int timeout, slot;
    if (parse_timeout(c, c->argv[3], &timeout) != 0) {
        return;
    }
    if (parse_slot(c, c->argv[4], &slot) != 0) {
        return;
    }

    dict *d = c->db->hash_slots[slot];
    int succ = 0;
    do {
        const dictEntry *de = dictGetRandomKey(d);
        if (de == NULL) {
            break;
        }
        sds skey = dictGetKey(de);
        robj *key = createStringObject(skey, sdslen(skey));
        succ = slotsmgrtone_command(c, host, port, timeout, key);
        decrRefCount(key);
        if (succ < 0) {
            return;
        }
    } while (0);
    addReplyMultiBulkLen(c, 2);
    addReplyLongLong(c, succ);
    addReplyLongLong(c, dictSize(d));
}

/* *
 * slotsmgrtone host port timeout key
 * */
void
slotsmgrtoneCommand(client *c) {
    sds host = c->argv[1]->ptr;
    sds port = c->argv[2]->ptr;
    int timeout;
    if (parse_timeout(c, c->argv[3], &timeout) != 0) {
        return;
    }

    robj *key = c->argv[4];
    int succ = slotsmgrtone_command(c, host, port, timeout, key);
    if (succ < 0) {
        return;
    }
    addReplyLongLong(c, succ);
}

static void
slotsScanSdsKeyCallback(void *l, const dictEntry *de) {
    sds skey = dictGetKey(de);
    robj *key = createStringObject(skey, sdslen(skey));
    listAddNodeTail((list *)l, key);
}

/* *
 * slotsdel slot1 [slot2 ...]
 * */
void
slotsdelCommand(client *c) {
    int slots_slot[HASH_SLOTS_SIZE];
    int n = 0;
    if (c->argc <= 1) {
        addReplyErrorFormat(c, "wrong number of arguments for 'slotsdel' command");
        return;
    }
    int i;
    for (i = 1; i < c->argc; i ++) {
        int slot;
        if (parse_slot(c, c->argv[i], &slot) != 0) {
            return;
        }
        slots_slot[n] = slot;
        n ++;
    }
    for (i = 0; i < n; i ++) {
        dict *d = c->db->hash_slots[slots_slot[i]];
        int s = dictSize(d);
        if (s == 0) {
            continue;
        }
        list *l = listCreate();
        listSetFreeMethod(l, decrRefCountVoid);
        unsigned long cursor = 0;
        do {
            cursor = dictScan(d, cursor, slotsScanSdsKeyCallback, l);
            while (1) {
                listNode *head = listFirst(l);
                if (head == NULL) {
                    break;
                }
                robj *key = listNodeValue(head);
                robj *keys[] = {key};
                slotsremove(c, keys, 1, 0);
                listDelNode(l, head);
            }
        } while (cursor != 0);
        listRelease(l);
    }
    addReplyMultiBulkLen(c, n);
    for (i = 0; i < n; i ++) {
        int n = slots_slot[i];
        int s = dictSize(c->db->hash_slots[n]);
        addReplyMultiBulkLen(c, 2);
        addReplyLongLong(c, n);
        addReplyLongLong(c, s);
    }
}

/* *
 * slotscheck
 * */
void
slotscheckCommand(client *c) {
    sds bug = NULL;
    int i;
    for (i = 0; i < HASH_SLOTS_SIZE && bug == NULL; i ++) {
        dict *d = c->db->hash_slots[i];
        if (dictSize(d) == 0) {
            continue;
        }
        list *l = listCreate();
        listSetFreeMethod(l, decrRefCountVoid);
        unsigned long cursor = 0;
        do {
            cursor = dictScan(d, cursor, slotsScanSdsKeyCallback, l);
            while (1) {
                listNode *head = listFirst(l);
                if (head == NULL) {
                    break;
                }
                robj *key = listNodeValue(head);
                if (lookupKeyRead(c->db, key) == NULL) {
                    if (bug == NULL) {
                        bug = sdsdup(key->ptr);
                    }
                }
                listDelNode(l, head);
            }
        } while (cursor != 0 && bug == NULL);
        listRelease(l);
    }
    if (bug != NULL) {
        addReplyErrorFormat(c, "step 1, miss = '%s'", bug);
        sdsfree(bug);
        return;
    }
    do {
        dict *d = c->db->dict;
        if (dictSize(d) == 0) {
            break;
        }
        list *l = listCreate();
        listSetFreeMethod(l, decrRefCountVoid);
        unsigned long cursor = 0;
        do {
            cursor = dictScan(d, cursor, slotsScanSdsKeyCallback, l);
            while (1) {
                listNode *head = listFirst(l);
                if (head == NULL) {
                    break;
                }
                robj *key = listNodeValue(head);
                int slot = slots_num(key->ptr, NULL, NULL);
                if (dictFind(c->db->hash_slots[slot], key->ptr) == NULL) {
                    if (bug == NULL) {
                        bug = sdsdup(key->ptr);
                    }
                }
                listDelNode(l, head);
            }
        } while (cursor != 0 && bug == NULL);
        listRelease(l);
    } while (0);
    if (bug != NULL) {
        addReplyErrorFormat(c, "step 2, miss = '%s'", bug);
        sdsfree(bug);
        return;
    }
    zskiplistNode *node = c->db->tagged_keys->header->level[0].forward;
    while (node != NULL && bug == NULL) {
        if (lookupKeyRead(c->db, node->obj) == NULL) {
            bug = sdsdup(node->obj->ptr);
        }
        node = node->level[0].forward;
    }
    if (bug != NULL) {
        addReplyErrorFormat(c, "step 3, miss = '%s'", bug);
        sdsfree(bug);
        return;
    }
    addReply(c, shared.ok);
}

/* *
 * slotsrestore key ttl val [key ttl val ...]
 * */
void
slotsrestoreCommand(client *c) {
    if (c->argc < 4 || (c->argc - 1) % 3 != 0) {
        addReplyErrorFormat(c, "wrong number of arguments for 'slotsrestore' command");
        return;
    }
    int n = (c->argc - 1) / 3;

    long long *ttls = zmalloc(sizeof(long long) * n);
    robj **vals = zmalloc(sizeof(robj *) * n);
    for (int i = 0; i < n; i ++) {
        vals[i] = NULL;
    }

    for (int i = 0; i < n; i ++) {
        robj *key = c->argv[i * 3 + 1];
        robj *ttl = c->argv[i * 3 + 2];
        robj *val = c->argv[i * 3 + 3];
        if (lookupKeyWrite(c->db, key) != NULL) {
            serverLog(LL_WARNING, "slotsrestore: slot = %d, key = '%s' already exists",
                    slots_num(key->ptr, NULL, NULL), (char *)key->ptr);
        }
        if (getLongLongFromObjectOrReply(c, ttl, &ttls[i], NULL) != C_OK) {
            goto cleanup;
        } else if (ttls[i] < 0) {
            addReplyError(c, "invalid ttl value, must be >= 0");
            goto cleanup;
        }
        rio payload;
        int type;
        if (verifyDumpPayload(val->ptr, sdslen(val->ptr)) != C_OK) {
            addReplyError(c, "dump payload version or checksum are wrong");
            goto cleanup;
        }
        rioInitWithBuffer(&payload, val->ptr);
        if (((type = rdbLoadObjectType(&payload)) == -1) ||
                ((vals[i] = rdbLoadObject(type, &payload)) == NULL)) {
            addReplyError(c, "bad data format");
            goto cleanup;
        }
    }

    for (int i = 0; i < n; i ++) {
        robj *key = c->argv[i * 3 + 1];
        long long ttl = ttls[i];
        robj *val = vals[i];
        dbDelete(c->db, key);
        dbAdd(c->db, key, val);
        incrRefCount(val);
        if (ttl) {
            setExpire(c->db, key, mstime() + ttl);
        }
        signalModifiedKey(c->db, key);
        server.dirty ++;
    }
    addReply(c, shared.ok);

cleanup:
    for (int i = 0; i < n; i ++) {
        if (vals[i] != NULL) {
            decrRefCount(vals[i]);
        }
    }
    zfree(vals);
    zfree(ttls);
}

/* *
 * do migrate mutli key-value(s) for {slotsmgrt/slotsmgrtone}with tag commands
 * return value:
 *    -1 - error happens
 *   >=0 - # of success migration
 * */
static int
slotsmgrttag_command(client *c, sds host, sds port, int timeout, robj *key) {
    uint32_t crc;
    int hastag;
    int slot = slots_num(key->ptr, &crc, &hastag);
    if (!hastag) {
        return slotsmgrtone_command(c, host, port, timeout, key);
    }

    slotsmgrt_sockfd *pfd = slotsmgrt_get_sockfd(c, host, port, timeout);
    if (pfd == NULL) {
        return -1;
    }

    dict *d = c->db->hash_slots[slot];
    if (dictSize(d) == 0) {
        return 0;
    }

    zrangespec range;
    range.min = (double)crc;
    range.minex = 0;
    range.max = (double)crc;
    range.maxex = 0;

    list *l = listCreate();
    listSetFreeMethod(l, decrRefCountVoid);

    zskiplistNode *node = zslFirstInRange(c->db->tagged_keys, &range);
    while (node != NULL && node->score == (double)crc) {
        listAddNodeTail(l, node->obj);
        incrRefCount(node->obj);
        node = node->level[0].forward;
    }

    int max = listLength(l);
    if (max == 0) {
        listRelease(l);
        return 0;
    }

    robj **keys = zmalloc(sizeof(robj *) * max);
    robj **vals = zmalloc(sizeof(robj *) * max);

    int n = 0;
    for (int i = 0; i < max; i ++) {
        listNode *head = listFirst(l);
        robj *key = listNodeValue(head);
        robj *val = lookupKeyWrite(c->db, key);
        if (val != NULL) {
            keys[n] = key;
            vals[n] = val;
            n ++;
            incrRefCount(key);
            incrRefCount(val);
        }
        listDelNode(l, head);
    }

    int ret = 0;
    if (n != 0) {
        if (slotsmgrt(c, host, port, pfd, c->db->id, timeout, keys, vals, n) != 0) {
            slotsmgrt_close_socket(host, port);
            ret = -1;
        } else {
            slotsremove(c, keys, n, 1);
            ret = n;
        }
    }

    listRelease(l);
    for (int i = 0; i < n; i ++) {
        decrRefCount(keys[i]);
        decrRefCount(vals[i]);
    }
    zfree(keys);
    zfree(vals);
    return ret;
}

/* *
 * slotsmgrttagslot host port timeout slot
 * */
void
slotsmgrttagslotCommand(client *c) {
    sds host = c->argv[1]->ptr;
    sds port = c->argv[2]->ptr;
    int timeout, slot;
    if (parse_timeout(c, c->argv[3], &timeout) != 0) {
        return;
    }
    if (parse_slot(c, c->argv[4], &slot) != 0) {
        return;
    }

    dict *d = c->db->hash_slots[slot];
    int succ = 0;
    do {
        const dictEntry *de = dictGetRandomKey(d);
        if (de == NULL) {
            break;
        }
        sds skey = dictGetKey(de);
        robj *key = createStringObject(skey, sdslen(skey));
        succ = slotsmgrttag_command(c, host, port, timeout, key);
        decrRefCount(key);
        if (succ < 0) {
            return;
        }
    } while (0);
    addReplyMultiBulkLen(c, 2);
    addReplyLongLong(c, succ);
    addReplyLongLong(c, dictSize(d));
}

/* *
 * slotsmgrttagone host port timeout key
 * */
void
slotsmgrttagoneCommand(client *c) {
    sds host = c->argv[1]->ptr;
    sds port = c->argv[2]->ptr;
    int timeout;
    if (parse_timeout(c, c->argv[3], &timeout) != 0) {
        return;
    }

    robj *key = c->argv[4];
    int succ = slotsmgrttag_command(c, host, port, timeout, key);
    if (succ < 0) {
        return;
    }
    addReplyLongLong(c, succ);
}

/* *
 * slotsscan slotnum cursor [COUNT count]
 * */
void
slotsscanCommand(client *c) {
    int slot;
    if (parse_slot(c, c->argv[1], &slot) != 0) {
        return;
    }
    unsigned long cursor;
    if (parseScanCursorOrReply(c, c->argv[2], &cursor) == C_ERR) {
        return;
    }
    unsigned long count = 10;
    if (c->argc != 3 && c->argc != 5) {
        addReplyErrorFormat(c, "wrong number of arguments for 'slotsscan' command");
        return;
    }
    if (c->argc == 5) {
        if (strcasecmp(c->argv[3]->ptr, "count") != 0) {
            addReply(c, shared.syntaxerr);
            return;
        }
        int v;
        if (parse_int(c, c->argv[4], &v) != 0) {
            return;
        }
        if (v < 1) {
            addReply(c, shared.syntaxerr);
            return;
        }
        count = v;
    }
    dict *d = c->db->hash_slots[slot];
    list *l = listCreate();
    listSetFreeMethod(l, decrRefCountVoid);

    long loops = count * 10;
    do {
        cursor = dictScan(d, cursor, slotsScanSdsKeyCallback, l);
        loops --;
    } while (cursor != 0 && loops > 0 && listLength(l) < count);

    addReplyMultiBulkLen(c, 2);
    addReplyBulkLongLong(c, cursor);

    addReplyMultiBulkLen(c, listLength(l));
    while (1) {
        listNode *head = listFirst(l);
        if (head == NULL) {
            break;
        }
        robj *key = listNodeValue(head);
        addReplyBulk(c, key);
        listDelNode(l, head);
    }

    listRelease(l);
}
