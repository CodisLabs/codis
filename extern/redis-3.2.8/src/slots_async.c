#include "server.h"

/* ============================ Worker Thread for Lazy Release ============================= */

typedef struct {
    pthread_t thread;
    pthread_mutex_t mutex;
    pthread_cond_t cond;
    list *objs;
} lazyReleaseWorker;

static void *
lazyReleaseWorkerMain(void *args) {
    lazyReleaseWorker *p = args;
    while (1) {
        pthread_mutex_lock(&p->mutex);
        while (listLength(p->objs) == 0) {
            pthread_cond_wait(&p->cond, &p->mutex);
        }
        listNode *head = listFirst(p->objs);
        robj *o = listNodeValue(head);
        listDelNode(p->objs, head);
        pthread_mutex_unlock(&p->mutex);

        decrRefCount(o);
    }
    return NULL;
}

static void
lazyReleaseObject(robj *o) {
    serverAssert(o->refcount == 1);
    lazyReleaseWorker *p = server.slotsmgrt_lazy_release;
    pthread_mutex_lock(&p->mutex);
    if (listLength(p->objs) == 0) {
        pthread_cond_broadcast(&p->cond);
    }
    listAddNodeTail(p->objs, o);
    pthread_mutex_unlock(&p->mutex);
}

static lazyReleaseWorker *
createLazyReleaseWorkerThread() {
    lazyReleaseWorker *p = zmalloc(sizeof(lazyReleaseWorker));
    pthread_mutex_init(&p->mutex, NULL);
    pthread_cond_init(&p->cond, NULL);
    p->objs = listCreate();
    if (pthread_create(&p->thread, NULL, lazyReleaseWorkerMain, p) != 0) {
        serverLog(LL_WARNING,"Fatal: Can't initialize Worker Thread for Lazy Release Jobs.");
        exit(1);
    }
    return p;
}

void
slotsmgrtInitLazyReleaseWorkerThread() {
    server.slotsmgrt_lazy_release = createLazyReleaseWorkerThread();
}

/* ============================ Iterator for Data Migration ================================ */

#define STAGE_PREPARE 0
#define STAGE_PAYLOAD 1
#define STAGE_CHUNKED 2
#define STAGE_FILLTTL 3
#define STAGE_DONE    4

typedef struct {
    int stage;
    robj *key;
    robj *val;
    long long expire;
    unsigned long cursor;
    unsigned long lindex;
    unsigned long zindex;
    unsigned long chunked_msgs;
} singleObjectIterator;

static singleObjectIterator *
createSingleObjectIterator(robj *key) {
    singleObjectIterator *it = zmalloc(sizeof(singleObjectIterator));
    it->stage = STAGE_PREPARE;
    it->key = key;
    incrRefCount(it->key);
    it->val = NULL;
    it->expire = 0;
    it->cursor = 0;
    it->lindex = 0;
    it->zindex = 0;
    it->chunked_msgs = 0;
    return it;
}

static void
freeSingleObjectIterator(singleObjectIterator *it) {
    if (it->val != NULL) {
        decrRefCount(it->val);
    }
    decrRefCount(it->key);
    zfree(it);
}

static void
freeSingleObjectIteratorVoid(void *it) {
    freeSingleObjectIterator(it);
}

static int
singleObjectIteratorHasNext(singleObjectIterator *it) {
    return it->stage != STAGE_DONE;
}

static size_t
sdslenOrElse(robj *o, size_t len) {
    return sdsEncodedObject(o) ? sdslen(o->ptr) : len;
}

static void
singleObjectIteratorScanCallback(void *data, const dictEntry *de) {
    void **pd = (void **)data;
    list *l = pd[0];
    robj *o = pd[1];
    long long *n = pd[2];

    robj *objs[2] = {NULL, NULL};
    switch (o->type) {
    case OBJ_HASH:
        objs[0] = dictGetKey(de);
        objs[1] = dictGetVal(de);
        break;
    case OBJ_SET:
        objs[0] = dictGetKey(de);
        break;
    }
    for (int i = 0; i < 2; i ++) {
        if (objs[i] != NULL) {
            incrRefCount(objs[i]);
            *n += sdslenOrElse(objs[i], 8);
            listAddNodeTail(l, objs[i]);
        }
    }
}

static uint64_t
convertDoubleToRawBits(double value) {
    union {
        double d;
        uint64_t u;
    } fp;
    fp.d = value;
    return fp.u;
}

static double
convertRawBitsToDouble(uint64_t value) {
    union {
        double d;
        uint64_t u;
    } fp;
    fp.u = value;
    return fp.d;
}

static robj *
createRawStringObjectFromUint64(uint64_t v) {
    uint64_t p = intrev64ifbe(v);
    return createRawStringObject((char *)&p, sizeof(p));
}

static int
getUint64FromRawStringObject(robj *o, uint64_t *p) {
    if (sdsEncodedObject(o) && sdslen(o->ptr) == sizeof(uint64_t)) {
        *p = intrev64ifbe(*(uint64_t *)(o->ptr));
        return C_OK;
    }
    return C_ERR;
}

static long
numberOfRestoreCommandsFromObject(robj *val, long long maxbulks) {
    long long numbulks = 0;
    switch (val->type) {
    case OBJ_LIST:
        if (val->encoding == OBJ_ENCODING_QUICKLIST) {
            numbulks = listTypeLength(val);
        }
        break;
    case OBJ_HASH:
        if (val->encoding == OBJ_ENCODING_HT) {
            numbulks = hashTypeLength(val) * 2;
        }
        break;
    case OBJ_SET:
        if (val->encoding == OBJ_ENCODING_HT) {
            numbulks = setTypeSize(val);
        }
        break;
    case OBJ_ZSET:
        if (val->encoding == OBJ_ENCODING_SKIPLIST) {
            numbulks = zsetLength(val) * 2;
        }
        break;
    }
    if (numbulks <= maxbulks) {
        return 1;
    }
    return (numbulks + maxbulks - 1) / maxbulks;
}

static long
estimateNumberOfRestoreCommands(redisDb *db, robj *key, long long maxbulks) {
    robj *val = lookupKeyWrite(db, key);
    if (val != NULL) {
        return numberOfRestoreCommandsFromObject(val, maxbulks);
    }
    return 0;
}

extern void createDumpPayload(rio *payload, robj *o);
extern zskiplistNode* zslGetElementByRank(zskiplist *zsl, unsigned long rank);

static slotsmgrtAsyncClient *getSlotsmgrtAsyncClient(int db);

static int
singleObjectIteratorNext(client *c, singleObjectIterator *it,
        long long timeout, unsigned int maxbulks, unsigned int maxbytes) {
    /* *
     * STAGE_PREPARE ---> STAGE_PAYLOAD ---> STAGE_DONE
     *     |                                      A
     *     V                                      |
     *     +------------> STAGE_CHUNKED ---> STAGE_FILLTTL
     *                      A       |
     *                      |       V
     *                      +-------+
     * */

    robj *key = it->key;

    if (it->stage == STAGE_PREPARE) {
        robj *val = lookupKeyWrite(c->db, key);
        if (val == NULL) {
            it->stage = STAGE_DONE;
            return 0;
        }
        it->val = val;
        incrRefCount(it->val);
        it->expire = getExpire(c->db, key);

        int leading_msgs = 0;

        slotsmgrtAsyncClient *ac = getSlotsmgrtAsyncClient(c->db->id);
        if (ac->c == c) {
            if (ac->used == 0) {
                ac->used = 1;
                if (server.requirepass != NULL) {
                    /* SLOTSRESTORE-ASYNC-AUTH $password */
                    addReplyMultiBulkLen(c, 2);
                    addReplyBulkCString(c, "SLOTSRESTORE-ASYNC-AUTH");
                    addReplyBulkCString(c, server.requirepass);
                    leading_msgs += 1;
                }
                do {
                    /* SLOTSRESTORE-ASYNC-SELECT $db */
                    addReplyMultiBulkLen(c, 2);
                    addReplyBulkCString(c, "SLOTSRESTORE-ASYNC-SELECT");
                    addReplyBulkLongLong(c, c->db->id);
                    leading_msgs += 1;
                } while (0);
            }
        }

        /* SLOTSRESTORE-ASYNC delete $key */
        addReplyMultiBulkLen(c, 3);
        addReplyBulkCString(c, "SLOTSRESTORE-ASYNC");
        addReplyBulkCString(c, "delete");
        addReplyBulk(c, key);

        long n = numberOfRestoreCommandsFromObject(val, maxbulks);
        if (n >= 2) {
            it->stage = STAGE_CHUNKED;
            it->chunked_msgs = n;
        } else {
            it->stage = STAGE_PAYLOAD;
            it->chunked_msgs = 0;
        }
        return 1 + leading_msgs;
    }

    robj *val = it->val;
    long long ttl = 0;
    if (it->stage == STAGE_CHUNKED) {
        ttl = timeout * 3;
    } else if (it->expire != -1) {
        ttl = it->expire - mstime();
        if (ttl < 1) {
            ttl = 1;
        }
    }

    if (it->stage == STAGE_FILLTTL) {
        /* SLOTSRESTORE-ASYNC expire $key $ttl */
        addReplyMultiBulkLen(c, 4);
        addReplyBulkCString(c, "SLOTSRESTORE-ASYNC");
        addReplyBulkCString(c, "expire");
        addReplyBulk(c, key);
        addReplyBulkLongLong(c, ttl);

        it->stage = STAGE_DONE;
        return 1;
    }

    if (it->stage == STAGE_PAYLOAD && val->type != OBJ_STRING) {
        rio payload;
        createDumpPayload(&payload, val);

        /* SLOTSRESTORE-ASYNC object $key $ttl $payload */
        addReplyMultiBulkLen(c, 5);
        addReplyBulkCString(c, "SLOTSRESTORE-ASYNC");
        addReplyBulkCString(c, "object");
        addReplyBulk(c, key);
        addReplyBulkLongLong(c, ttl);
        addReplyBulkSds(c, payload.io.buffer.ptr);

        it->stage = STAGE_DONE;
        return 1;
    }

    if (it->stage == STAGE_PAYLOAD && val->type == OBJ_STRING) {
        /* SLOTSRESTORE-ASYNC string $key $ttl $payload */
        addReplyMultiBulkLen(c, 5);
        addReplyBulkCString(c, "SLOTSRESTORE-ASYNC");
        addReplyBulkCString(c, "string");
        addReplyBulk(c, key);
        addReplyBulkLongLong(c, ttl);
        addReplyBulk(c, val);

        it->stage = STAGE_DONE;
        return 1;
    }

    if (it->stage == STAGE_CHUNKED) {
        const char *cmd = NULL;
        switch (val->type) {
        case OBJ_LIST:
            cmd = "list";
            break;
        case OBJ_HASH:
            cmd = "hash";
            break;
        case OBJ_SET:
            cmd = "dict";
            break;
        case OBJ_ZSET:
            cmd = "zset";
            break;
        default:
            serverPanic("unknown object type");
        }

        int more = 1;

        list *ll = listCreate();
        listSetFreeMethod(ll, decrRefCountVoid);
        long long hint = 0, len = 0;

        if (val->type == OBJ_LIST) {
            listTypeIterator *li = listTypeInitIterator(val, it->lindex, LIST_TAIL);
            do {
                listTypeEntry entry;
                if (listTypeNext(li, &entry)) {
                    quicklistEntry *e = &(entry.entry);
                    robj *obj;
                    if (e->value) {
                        obj = createStringObject((const char *)e->value, e->sz);
                    } else {
                        obj = createStringObjectFromLongLong(e->longval);
                    }
                    len += sdslenOrElse(obj, 8);
                    listAddNodeTail(ll, obj);
                    it->lindex ++;
                } else {
                    more = 0;
                }
            } while (more && listLength(ll) < maxbulks && len < maxbytes);
            listTypeReleaseIterator(li);
            hint = listTypeLength(val);
        }

        if (val->type == OBJ_HASH || val->type == OBJ_SET) {
            int loop = maxbulks * 10;
            if (loop < 100) {
                loop = 100;
            }
            dict *ht = val->ptr;
            void *pd[] = {ll, val, &len};
            do {
                it->cursor = dictScan(ht, it->cursor, singleObjectIteratorScanCallback, pd);
                if (it->cursor == 0) {
                    more = 0;
                }
            } while (more && listLength(ll) < maxbulks && len < maxbytes && (-- loop) >= 0);
            hint = dictSize(ht);
        }

        if (val->type == OBJ_ZSET) {
            zset *zs = val->ptr;
            dict *ht = zs->dict;
            long long rank = (long long)zsetLength(val) - it->zindex;
            zskiplistNode *node = (rank >= 1) ? zslGetElementByRank(zs->zsl, rank) : NULL;
            do {
                if (node != NULL) {
                    robj *field = node->obj;
                    incrRefCount(field);
                    len += sdslenOrElse(field, 8);
                    listAddNodeTail(ll, field);
                    uint64_t bits = convertDoubleToRawBits(node->score);
                    robj *score = createRawStringObjectFromUint64(bits);
                    len += sdslenOrElse(score, 8);
                    listAddNodeTail(ll, score);
                    node = node->backward;
                    it->zindex ++;
                } else {
                    more = 0;
                }
            } while (more && listLength(ll) < maxbulks && len < maxbytes);
            hint = dictSize(ht);
        }

        /* SLOTSRESTORE-ASYNC list/hash/zset/dict $key $ttl $hint [$arg1 ...] */
        addReplyMultiBulkLen(c, 5 + listLength(ll));
        addReplyBulkCString(c, "SLOTSRESTORE-ASYNC");
        addReplyBulkCString(c, cmd);
        addReplyBulk(c, key);
        addReplyBulkLongLong(c, ttl);
        addReplyBulkLongLong(c, hint);

        while (listLength(ll) != 0) {
            listNode *head = listFirst(ll);
            robj *obj = listNodeValue(head);
            addReplyBulk(c, obj);
            listDelNode(ll, head);
        }
        listRelease(ll);

        if (!more) {
            it->stage = STAGE_FILLTTL;
        }
        return 1;
    }

    if (it->stage != STAGE_DONE) {
        serverPanic("invalid iterator stage");
    }

    serverPanic("use of empty iterator");
}

/* ============================ Iterator for Data Migration (batched) ====================== */

typedef struct {
    struct zskiplist *tags;
    dict *keys;
    list *list;
    dict *hash_slot;
    struct zskiplist *hash_tags;
    long long timeout;
    unsigned int maxbulks;
    unsigned int maxbytes;
    list *removed_keys;
    list *chunked_vals;
    long estimate_msgs;
} batchedObjectIterator;

static batchedObjectIterator *
createBatchedObjectIterator(dict *hash_slot, struct zskiplist *hash_tags,
        long long timeout, unsigned int maxbulks, unsigned int maxbytes) {
    batchedObjectIterator *it = zmalloc(sizeof(batchedObjectIterator));
    it->tags = zslCreate();
    it->keys = dictCreate(&setDictType, NULL);
    it->list = listCreate();
    listSetFreeMethod(it->list, freeSingleObjectIteratorVoid);
    it->hash_slot = hash_slot;
    it->hash_tags = hash_tags;
    it->timeout = timeout;
    it->maxbulks = maxbulks;
    it->maxbytes = maxbytes;
    it->removed_keys = listCreate();
    listSetFreeMethod(it->removed_keys, decrRefCountVoid);
    it->chunked_vals = listCreate();
    listSetFreeMethod(it->chunked_vals, decrRefCountVoid);
    it->estimate_msgs = 0;
    return it;
}

static void
freeBatchedObjectIterator(batchedObjectIterator *it) {
    zslFree(it->tags);
    dictRelease(it->keys);
    listRelease(it->list);
    listRelease(it->removed_keys);
    listRelease(it->chunked_vals);
    zfree(it);
}

static int
batchedObjectIteratorHasNext(batchedObjectIterator *it) {
    while (listLength(it->list) != 0) {
        listNode *head = listFirst(it->list);
        singleObjectIterator *sp = listNodeValue(head);
        if (singleObjectIteratorHasNext(sp)) {
            return 1;
        }
        if (sp->val != NULL) {
            incrRefCount(sp->key);
            listAddNodeTail(it->removed_keys, sp->key);
            if (sp->chunked_msgs != 0) {
                incrRefCount(sp->val);
                listAddNodeTail(it->chunked_vals, sp->val);
            }
        }
        listDelNode(it->list, head);
    }
    return 0;
}

static int
batchedObjectIteratorNext(client *c, batchedObjectIterator *it) {
    if (listLength(it->list) != 0) {
        listNode *head = listFirst(it->list);
        singleObjectIterator *sp = listNodeValue(head);
        long long maxbytes = (long long)it->maxbytes - getClientOutputBufferMemoryUsage(c);
        return singleObjectIteratorNext(c, sp, it->timeout, it->maxbulks, maxbytes > 0 ? maxbytes : 0);
    }
    serverPanic("use of empty iterator");
}

static int
batchedObjectIteratorContains(batchedObjectIterator *it, robj *key, int usetag) {
    if (dictFind(it->keys, key) != NULL) {
        return 1;
    }
    if (!usetag) {
        return 0;
    }
    uint32_t crc;
    int hastag;
    slots_num(key->ptr, &crc, &hastag);
    if (!hastag) {
        return 0;
    }
    zrangespec range;
    range.min = (double)crc;
    range.minex = 0;
    range.max = (double)crc;
    range.maxex = 0;
    return zslFirstInRange(it->tags, &range) != NULL;
}

static int
batchedObjectIteratorAddKey(redisDb *db, batchedObjectIterator *it, robj *key) {
    if (dictAdd(it->keys, key, NULL) != C_OK) {
        return 0;
    }
    incrRefCount(key);
    listAddNodeTail(it->list, createSingleObjectIterator(key));
    it->estimate_msgs += estimateNumberOfRestoreCommands(db, key, it->maxbulks);

    int size = dictSize(it->keys);

    uint32_t crc;
    int hastag;
    slots_num(key->ptr, &crc, &hastag);
    if (!hastag) {
        goto out;
    }
    zrangespec range;
    range.min = (double)crc;
    range.minex = 0;
    range.max = (double)crc;
    range.maxex = 0;
    if (zslFirstInRange(it->tags, &range) != NULL) {
        goto out;
    }
    incrRefCount(key);
    zslInsert(it->tags, (double)crc, key);

    if (it->hash_tags == NULL) {
        goto out;
    }
    zskiplistNode *node = zslFirstInRange(it->hash_tags, &range);
    while (node != NULL && node->score == (double)crc) {
        robj *key = node->obj;
        node = node->level[0].forward;
        if (dictAdd(it->keys, key, NULL) != C_OK) {
            continue;
        }
        incrRefCount(key);
        listAddNodeTail(it->list, createSingleObjectIterator(key));
        it->estimate_msgs += estimateNumberOfRestoreCommands(db, key, it->maxbulks);
    }

out:
    return 1 + dictSize(it->keys) - size;
}

/* ============================ Clients ==================================================== */

static slotsmgrtAsyncClient *
getSlotsmgrtAsyncClient(int db) {
    return &server.slotsmgrt_cached_clients[db];
}

static void
notifySlotsmgrtAsyncClient(slotsmgrtAsyncClient *ac, const char *errmsg) {
    batchedObjectIterator *it = ac->batched_iter;
    list *ll = ac->blocked_list;
    while (listLength(ll) != 0) {
        listNode *head = listFirst(ll);
        client *c = listNodeValue(head);
        if (errmsg != NULL) {
            addReplyError(c, errmsg);
        } else if (it == NULL) {
            addReplyError(c, "invalid iterator (NULL)");
        } else if (it->hash_slot == NULL) {
            addReplyLongLong(c, listLength(it->removed_keys));
        } else {
            addReplyMultiBulkLen(c, 2);
            addReplyLongLong(c, listLength(it->removed_keys));
            addReplyLongLong(c, dictSize(it->hash_slot));
        }
        c->slotsmgrt_flags &= ~CLIENT_SLOTSMGRT_ASYNC_NORMAL_CLIENT;
        c->slotsmgrt_fenceq = NULL;
        listDelNode(ll, head);
    }
}

static void
unlinkSlotsmgrtAsyncCachedClient(client *c, const char *errmsg) {
    slotsmgrtAsyncClient *ac = getSlotsmgrtAsyncClient(c->db->id);
    serverAssert(c->slotsmgrt_flags & CLIENT_SLOTSMGRT_ASYNC_CACHED_CLIENT);
    serverAssert(ac->c == c);

    notifySlotsmgrtAsyncClient(ac, errmsg);

    batchedObjectIterator *it = ac->batched_iter;

    long long elapsed = mstime() - ac->lastuse;
    serverLog(LL_WARNING, "slotsmgrt_async: unlink client %s:%d (DB=%d): "
            "sending_msgs = %ld, batched_iter = %ld, blocked_list = %ld, "
            "timeout = %lld(ms), elapsed = %lld(ms) (%s)",
            ac->host, ac->port, c->db->id, ac->sending_msgs,
            it != NULL ? (long)listLength(it->list) : -1, (long)listLength(ac->blocked_list),
            ac->timeout, elapsed, errmsg);

    sdsfree(ac->host);
    if (it != NULL) {
        freeBatchedObjectIterator(it);
    }
    listRelease(ac->blocked_list);

    c->slotsmgrt_flags &= ~CLIENT_SLOTSMGRT_ASYNC_CACHED_CLIENT;

    memset(ac, 0, sizeof(*ac));
}

static int
releaseSlotsmgrtAsyncClient(int db, const char *errmsg) {
    slotsmgrtAsyncClient *ac = getSlotsmgrtAsyncClient(db);
    if (ac->c == NULL) {
        return 0;
    }
    client *c = ac->c;
    unlinkSlotsmgrtAsyncCachedClient(c, errmsg);
    freeClient(c);
    return 1;
}

static int
createSlotsmgrtAsyncClient(int db, char *host, int port, long timeout) {
    int fd = anetTcpNonBlockConnect(server.neterr, host, port);
    if (fd == -1) {
        serverLog(LL_WARNING, "slotsmgrt_async: create socket %s:%d (DB=%d) failed, %s",
                host, port, db, server.neterr);
        return C_ERR;
    }
    anetEnableTcpNoDelay(server.neterr, fd);

    int wait = 100;
    if (wait > timeout) {
        wait = timeout;
    }
    if ((aeWait(fd, AE_WRITABLE, wait) & AE_WRITABLE) == 0) {
        serverLog(LL_WARNING, "slotsmgrt_async: create socket %s:%d (DB=%d) failed, io error or timeout (%d)",
                host, port, db, wait);
        close(fd);
        return C_ERR;
    }

    client *c = createClient(fd);
    if (c == NULL) {
        serverLog(LL_WARNING, "slotsmgrt_async: create client %s:%d (DB=%d) failed, %s",
                host, port, db, server.neterr);
        return C_ERR;
    }
    if (selectDb(c, db) != C_OK) {
        serverLog(LL_WARNING, "slotsmgrt_async: invalid DB index (DB=%d)", db);
        freeClient(c);
        return C_ERR;
    }
    c->slotsmgrt_flags |= CLIENT_SLOTSMGRT_ASYNC_CACHED_CLIENT;
    c->authenticated = 1;

    releaseSlotsmgrtAsyncClient(db, "interrupted: build new connection");

    serverLog(LL_WARNING, "slotsmgrt_async: create client %s:%d (DB=%d) OK", host, port, db);

    slotsmgrtAsyncClient *ac = getSlotsmgrtAsyncClient(db);
    ac->c = c;
    ac->used = 0;
    ac->host = sdsnew(host);
    ac->port = port;
    ac->timeout = timeout;
    ac->lastuse = mstime();
    ac->sending_msgs = 0;
    ac->batched_iter = NULL;
    ac->blocked_list = listCreate();
    return C_OK;
}

static slotsmgrtAsyncClient *
getOrCreateSlotsmgrtAsyncClient(int db, char *host, int port, long timeout) {
    slotsmgrtAsyncClient *ac = getSlotsmgrtAsyncClient(db);
    if (ac->c != NULL) {
        if (ac->port == port && !strcmp(ac->host, host)) {
            return ac;
        }
    }
    return createSlotsmgrtAsyncClient(db, host, port, timeout) != C_OK ? NULL : ac;
}

static void
unlinkSlotsmgrtAsyncNormalClient(client *c) {
    serverAssert(c->slotsmgrt_flags & CLIENT_SLOTSMGRT_ASYNC_NORMAL_CLIENT);
    serverAssert(c->slotsmgrt_fenceq != NULL);

    list *ll = c->slotsmgrt_fenceq;
    listNode *node = listSearchKey(ll, c);
    serverAssert(node != NULL);

    c->slotsmgrt_flags &= ~CLIENT_SLOTSMGRT_ASYNC_NORMAL_CLIENT;
    c->slotsmgrt_fenceq = NULL;
    listDelNode(ll, node);
}

void
slotsmgrtAsyncUnlinkClient(client *c) {
    if (c->slotsmgrt_flags & CLIENT_SLOTSMGRT_ASYNC_CACHED_CLIENT) {
        unlinkSlotsmgrtAsyncCachedClient(c, "interrupted: connection closed");
    }
    if (c->slotsmgrt_flags & CLIENT_SLOTSMGRT_ASYNC_NORMAL_CLIENT) {
        unlinkSlotsmgrtAsyncNormalClient(c);
    }
}

void
slotsmgrtAsyncCleanup() {
    for (int i = 0; i < server.dbnum; i ++) {
        slotsmgrtAsyncClient *ac = getSlotsmgrtAsyncClient(i);
        if (ac->c == NULL) {
            continue;
        }
        long long elapsed = mstime() - ac->lastuse;
        long long timeout = ac->batched_iter != NULL ? ac->timeout : 1000LL * 60;
        if (elapsed <= timeout) {
            continue;
        }
        releaseSlotsmgrtAsyncClient(i, ac->batched_iter != NULL ?
                "interrupted: migration timeout" : "interrupted: idle timeout");
    }
}

static int
getSlotsmgrtAsyncClientMigrationStatusOrBlock(client *c, robj *key, int block) {
    slotsmgrtAsyncClient *ac = getSlotsmgrtAsyncClient(c->db->id);
    if (ac->c == NULL || ac->batched_iter == NULL) {
        return 0;
    }
    batchedObjectIterator *it = ac->batched_iter;
    if (key != NULL && !batchedObjectIteratorContains(it, key, 1)) {
        return 0;
    }
    if (!block) {
        return 1;
    }
    if (c->slotsmgrt_flags & CLIENT_SLOTSMGRT_ASYNC_NORMAL_CLIENT) {
        return -1;
    }
    list *ll = ac->blocked_list;
    c->slotsmgrt_flags |= CLIENT_SLOTSMGRT_ASYNC_NORMAL_CLIENT;
    c->slotsmgrt_fenceq = ll;
    listAddNodeTail(ll, c);
    return 1;
}

/* ============================ Slotsmgrt{One,TagOne}AsyncDumpCommand ====================== */

/* SLOTSMGRTONE-ASYNC-DUMP    $timeout $maxbulks $key1 [$key2 ...] */
/* SLOTSMGRTTAGONE-ASYNC-DUMP $timeout $maxbulks $key1 [$key2 ...] */
static void
slotsmgrtAsyncDumpGenericCommand(client *c, int usetag) {
    long long timeout;
    if (getLongLongFromObject(c->argv[1], &timeout) != C_OK ||
            !(timeout >= 0 && timeout <= INT_MAX)) {
        addReplyErrorFormat(c, "invalid value of timeout (%s)",
                (char *)c->argv[1]->ptr);
        return;
    }
    if (timeout == 0) {
        timeout = 1000 * 30;
    }
    long long maxbulks;
    if (getLongLongFromObject(c->argv[2], &maxbulks) != C_OK ||
            !(maxbulks >= 0 && maxbulks <= INT_MAX)) {
        addReplyErrorFormat(c, "invalid value of maxbulks (%s)",
                (char *)c->argv[2]->ptr);
        return;
    }
    if (maxbulks == 0) {
        maxbulks = 1000;
    }

    batchedObjectIterator *it = createBatchedObjectIterator(NULL,
            usetag ? c->db->tagged_keys : NULL, timeout, maxbulks, INT_MAX);
    for (int i = 3; i < c->argc; i ++) {
        batchedObjectIteratorAddKey(c->db, it, c->argv[i]);
    }

    void *ptr = addDeferredMultiBulkLength(c);
    int total = 0;
    while (batchedObjectIteratorHasNext(it)) {
        total += batchedObjectIteratorNext(c, it);
    }
    setDeferredMultiBulkLength(c, ptr, total);
    freeBatchedObjectIterator(it);
}

/* *
 * SLOTSMGRTONE-ASYNC-DUMP    $timeout $maxbulks $key1 [$key2 ...]
 * */
void slotsmgrtOneAsyncDumpCommand(client *c) {
    if (c->argc <= 3) {
        addReplyError(c, "wrong number of arguments for SLOTSMGRTONE-ASYNC-DUMP");
        return;
    }
    slotsmgrtAsyncDumpGenericCommand(c, 0);
}

/* *
 * SLOTSMGRTTAGONE-ASYNC-DUMP $timeout $maxbulks $key1 [$key2 ...]
 * */
void
slotsmgrtTagOneAsyncDumpCommand(client *c) {
    if (c->argc <= 3) {
        addReplyError(c, "wrong number of arguments for SLOTSMGRTTAGONE-ASYNC-DUMP");
        return;
    }
    slotsmgrtAsyncDumpGenericCommand(c, 1);
}

/* ============================ Slotsmgrt{One,TagOne,Slot,TagSlot}AsyncCommand ============= */

static unsigned int
slotsmgrtAsyncMaxBufferLimit(unsigned int maxbytes) {
    clientBufferLimitsConfig *config = &server.client_obuf_limits[CLIENT_TYPE_NORMAL];
    if (config->soft_limit_bytes != 0 && config->soft_limit_bytes < maxbytes) {
        maxbytes = config->soft_limit_bytes;
    }
    if (config->hard_limit_bytes != 0 && config->hard_limit_bytes < maxbytes) {
        maxbytes = config->hard_limit_bytes;
    }
    return maxbytes;
}

static long
slotsmgrtAsyncNextMessagesMicroseconds(slotsmgrtAsyncClient *ac, long atleast, long long usecs) {
    batchedObjectIterator *it = ac->batched_iter;
    long long deadline = ustime() + usecs;
    long msgs = 0;
    while (batchedObjectIteratorHasNext(it) && getClientOutputBufferMemoryUsage(ac->c) < it->maxbytes) {
        if ((msgs += batchedObjectIteratorNext(ac->c, it)) < atleast) {
            continue;
        }
        if (ustime() >= deadline) {
            return msgs;
        }
    }
    return msgs;
}

static void
slotsScanSdsKeyCallback(void *l, const dictEntry *de) {
    sds skey = dictGetKey(de);
    robj *key = createStringObject(skey, sdslen(skey));
    listAddNodeTail((list *)l, key);
}

/* SLOTSMGRTONE-ASYNC     $host $port $timeout $maxbulks $maxbytes $key1 [$key2 ...] */
/* SLOTSMGRTTAGONE-ASYNC  $host $port $timeout $maxbulks $maxbytes $key1 [$key2 ...] */
/* SLOTSMGRTSLOT-ASYNC    $host $port $timeout $maxbulks $maxbytes $slot $numkeys    */
/* SLOTSMGRTTAGSLOT-ASYNC $host $port $timeout $maxbulks $maxbytes $slot $numkeys    */
static void
slotsmgrtAsyncGenericCommand(client *c, int usetag, int usekey) {
    char *host = c->argv[1]->ptr;
    long long port;
    if (getLongLongFromObject(c->argv[2], &port) != C_OK ||
            !(port >= 1 && port < 65536)) {
        addReplyErrorFormat(c, "invalid value of port (%s)",
                (char *)c->argv[2]->ptr);
        return;
    }
    long long timeout;
    if (getLongLongFromObject(c->argv[3], &timeout) != C_OK ||
            !(timeout >= 0 && timeout <= INT_MAX)) {
        addReplyErrorFormat(c, "invalid value of timeout (%s)",
                (char *)c->argv[3]->ptr);
        return;
    }
    if (timeout == 0) {
        timeout = 1000 * 30;
    }
    long long maxbulks;
    if (getLongLongFromObject(c->argv[4], &maxbulks) != C_OK ||
            !(maxbulks >= 0 && maxbulks <= INT_MAX)) {
        addReplyErrorFormat(c, "invalid value of maxbulks (%s)",
                (char *)c->argv[4]->ptr);
        return;
    }
    if (maxbulks == 0) {
        maxbulks = 200;
    }
    if (maxbulks > 512 * 1024) {
        maxbulks = 512 * 1024;
    }
    long long maxbytes;
    if (getLongLongFromObject(c->argv[5], &maxbytes) != C_OK ||
            !(maxbytes >= 0 && maxbytes <= INT_MAX)) {
        addReplyErrorFormat(c, "invalid value of maxbytes (%s)",
                (char *)c->argv[5]->ptr);
        return;
    }
    if (maxbytes == 0) {
        maxbytes = 512 * 1024;
    }
    if (maxbytes > INT_MAX / 2) {
        maxbytes = INT_MAX / 2;
    }
    maxbytes = slotsmgrtAsyncMaxBufferLimit(maxbytes);

    dict *hash_slot = NULL;
    long long numkeys = 0;
    if (!usekey) {
        long long slotnum;
        if (getLongLongFromObject(c->argv[6], &slotnum) != C_OK ||
                !(slotnum >= 0 && slotnum < HASH_SLOTS_SIZE)) {
            addReplyErrorFormat(c, "invalid value of slot (%s)",
                    (char *)c->argv[6]->ptr);
            return;
        }
        hash_slot = c->db->hash_slots[slotnum];
        if (getLongLongFromObject(c->argv[7], &numkeys) != C_OK ||
                !(numkeys >= 0 && numkeys <= INT_MAX)) {
            addReplyErrorFormat(c, "invalid value of numkeys (%s)",
                    (char *)c->argv[7]->ptr);
            return;
        }
        if (numkeys == 0) {
            numkeys = 100;
        }
    }

    if (getSlotsmgrtAsyncClientMigrationStatusOrBlock(c, NULL, 0) != 0) {
        addReplyError(c, "the specified DB is being migrated");
        return;
    }
    if (c->slotsmgrt_flags & CLIENT_SLOTSMGRT_ASYNC_NORMAL_CLIENT) {
        addReplyError(c, "previous operation has not finished");
        return;
    }

    slotsmgrtAsyncClient *ac = getOrCreateSlotsmgrtAsyncClient(c->db->id, host, port, timeout);
    if (ac == NULL) {
        addReplyErrorFormat(c, "create client to %s:%d failed", host, (int)port);
        return;
    }

    batchedObjectIterator *it = createBatchedObjectIterator(hash_slot,
            usetag ? c->db->tagged_keys : NULL, timeout, maxbulks, maxbytes);
    if (!usekey) {
        list *ll = listCreate();
        listSetFreeMethod(ll, decrRefCountVoid);
        for (int i = 2; i >= 0 && it->estimate_msgs < numkeys; i --) {
            unsigned long cursor = 0;
            if (i != 0) {
                cursor = random();
            } else {
                if (htNeedsResize(hash_slot)) {
                    dictResize(hash_slot);
                }
            }
            if (dictIsRehashing(hash_slot)) {
                dictRehash(hash_slot, 50);
            }
            int loop = numkeys * 10;
            if (loop < 100) {
                loop = 100;
            }
            do {
                cursor = dictScan(hash_slot, cursor, slotsScanSdsKeyCallback, ll);
                while (listLength(ll) != 0 && it->estimate_msgs < numkeys) {
                    listNode *head = listFirst(ll);
                    robj *key = listNodeValue(head);
                    long msgs = estimateNumberOfRestoreCommands(c->db, key, it->maxbulks);
                    if (it->estimate_msgs == 0 || it->estimate_msgs + msgs <= numkeys * 2) {
                        batchedObjectIteratorAddKey(c->db, it, key);
                    }
                    listDelNode(ll, head);
                }
            } while (cursor != 0 && it->estimate_msgs < numkeys &&
                    dictSize(it->keys) < (unsigned long)numkeys && (-- loop) >= 0);
        }
        listRelease(ll);
    } else {
        for (int i = 6; i < c->argc; i ++) {
            batchedObjectIteratorAddKey(c->db, it, c->argv[i]);
        }
    }
    serverAssert(ac->sending_msgs == 0);
    serverAssert(ac->batched_iter == NULL && listLength(ac->blocked_list) == 0);

    ac->timeout = timeout;
    ac->lastuse = mstime();
    ac->batched_iter = it;
    ac->sending_msgs = slotsmgrtAsyncNextMessagesMicroseconds(ac, 3, 500);

    getSlotsmgrtAsyncClientMigrationStatusOrBlock(c, NULL, 1);

    if (ac->sending_msgs != 0) {
        return;
    }
    notifySlotsmgrtAsyncClient(ac, NULL);

    ac->batched_iter = NULL;
    freeBatchedObjectIterator(it);
}

/* *
 * SLOTSMGRTONE-ASYNC     $host $port $timeout $maxbulks $maxbytes $key1 [$key2 ...]
 * */
void slotsmgrtOneAsyncCommand(client *c) {
    if (c->argc <= 6) {
        addReplyError(c, "wrong number of arguments for SLOTSMGRTONE-ASYNC");
        return;
    }
    slotsmgrtAsyncGenericCommand(c, 0, 1);
}

/* *
 * SLOTSMGRTTAGONE-ASYNC  $host $port $timeout $maxbulks $maxbytes $key1 [$key2 ...]
 * */
void slotsmgrtTagOneAsyncCommand(client *c) {
    if (c->argc <= 6) {
        addReplyError(c, "wrong number of arguments for SLOTSMGRTTAGONE-ASYNC");
        return;
    }
    slotsmgrtAsyncGenericCommand(c, 1, 1);
}

/* *
 * SLOTSMGRTSLOT-ASYNC    $host $port $timeout $maxbulks $maxbytes $slot $numkeys
 * */
void slotsmgrtSlotAsyncCommand(client *c) {
    if (c->argc != 8) {
        addReplyError(c, "wrong number of arguments for SLOTSMGRTSLOT-ASYNC");
        return;
    }
    slotsmgrtAsyncGenericCommand(c, 0, 0);
}

/* *
 * SLOTSMGRTTAGSLOT-ASYNC $host $port $timeout $maxbulks $maxbytes $slot $numkeys
 * */
void slotsmgrtTagSlotAsyncCommand(client *c) {
    if (c->argc != 8) {
        addReplyError(c, "wrong number of arguments for SLOTSMGRTSLOT-ASYNC");
        return;
    }
    slotsmgrtAsyncGenericCommand(c, 1, 0);
}

/* *
 * SLOTSMGRT-ASYNC-FENCE
 * */
void
slotsmgrtAsyncFenceCommand(client *c) {
    int ret = getSlotsmgrtAsyncClientMigrationStatusOrBlock(c, NULL, 1);
    if (ret == 0) {
        addReply(c, shared.ok);
    } else if (ret != 1) {
        addReplyError(c, "previous operation has not finished (call fence again)");
    }
}

/* *
 * SLOTSMGRT-ASYNC-CANCEL
 * */
void
slotsmgrtAsyncCancelCommand(client *c) {
    addReplyLongLong(c, releaseSlotsmgrtAsyncClient(c->db->id, "interrupted: canceled"));
}

/* ============================ SlotsmgrtAsyncStatus ======================================= */

static void
singleObjectIteratorStatus(client *c, singleObjectIterator *it) {
    if (it == NULL) {
        addReply(c, shared.nullmultibulk);
        return;
    }
    void *ptr = addDeferredMultiBulkLength(c);
    int fields = 0;

    fields ++; addReplyBulkCString(c, "key");
    addReplyBulk(c, it->key);

    fields ++; addReplyBulkCString(c, "val.type");
    addReplyBulkLongLong(c, it->val == NULL ? -1 : it->val->type);

    fields ++; addReplyBulkCString(c, "stage");
    addReplyBulkLongLong(c, it->stage);

    fields ++; addReplyBulkCString(c, "expire");
    addReplyBulkLongLong(c, it->expire);

    fields ++; addReplyBulkCString(c, "cursor");
    addReplyBulkLongLong(c, it->cursor);

    fields ++; addReplyBulkCString(c, "lindex");
    addReplyBulkLongLong(c, it->lindex);

    fields ++; addReplyBulkCString(c, "zindex");
    addReplyBulkLongLong(c, it->zindex);

    fields ++; addReplyBulkCString(c, "chunked_msgs");
    addReplyBulkLongLong(c, it->chunked_msgs);

    setDeferredMultiBulkLength(c, ptr, fields * 2);
}

static void
batchedObjectIteratorStatus(client *c, batchedObjectIterator *it) {
    if (it == NULL) {
        addReply(c, shared.nullmultibulk);
        return;
    }
    void *ptr = addDeferredMultiBulkLength(c);
    int fields = 0;

    fields ++; addReplyBulkCString(c, "keys");
    addReplyMultiBulkLen(c, 2);
    addReplyBulkLongLong(c, dictSize(it->keys));
    addReplyMultiBulkLen(c, dictSize(it->keys));
    dictIterator *di = dictGetIterator(it->keys);
    dictEntry *de;
    while((de = dictNext(di)) != NULL) {
        addReplyBulk(c, dictGetKey(de));
    }
    dictReleaseIterator(di);

    fields ++; addReplyBulkCString(c, "timeout");
    addReplyBulkLongLong(c, it->timeout);

    fields ++; addReplyBulkCString(c, "maxbulks");
    addReplyBulkLongLong(c, it->maxbulks);

    fields ++; addReplyBulkCString(c, "maxbytes");
    addReplyBulkLongLong(c, it->maxbytes);

    fields ++; addReplyBulkCString(c, "estimate_msgs");
    addReplyBulkLongLong(c, it->estimate_msgs);

    fields ++; addReplyBulkCString(c, "removed_keys");
    addReplyBulkLongLong(c, listLength(it->removed_keys));

    fields ++; addReplyBulkCString(c, "chunked_vals");
    addReplyBulkLongLong(c, listLength(it->chunked_vals));

    fields ++; addReplyBulkCString(c, "iterators");
    addReplyMultiBulkLen(c, 2);
    addReplyBulkLongLong(c, listLength(it->list));
    singleObjectIterator *sp = NULL;
    if (listLength(it->list) != 0) {
        sp = listNodeValue(listFirst(it->list));
    }
    singleObjectIteratorStatus(c, sp);

    setDeferredMultiBulkLength(c, ptr, fields * 2);
}

/* *
 * SLOTSMGRT-ASYNC-STATUS
 * */
void
slotsmgrtAsyncStatusCommand(client *c) {
    slotsmgrtAsyncClient *ac = getSlotsmgrtAsyncClient(c->db->id);
    if (ac->c == NULL) {
        addReply(c, shared.nullmultibulk);
        return;
    }
    void *ptr = addDeferredMultiBulkLength(c);
    int fields = 0;

    fields ++; addReplyBulkCString(c, "host");
    addReplyBulkCString(c, ac->host);

    fields ++; addReplyBulkCString(c, "port");
    addReplyBulkLongLong(c, ac->port);

    fields ++; addReplyBulkCString(c, "used");
    addReplyBulkLongLong(c, ac->used);

    fields ++; addReplyBulkCString(c, "timeout");
    addReplyBulkLongLong(c, ac->timeout);

    fields ++; addReplyBulkCString(c, "lastuse");
    addReplyBulkLongLong(c, ac->lastuse);

    fields ++; addReplyBulkCString(c, "since_lastuse");
    addReplyBulkLongLong(c, mstime() - ac->lastuse);

    fields ++; addReplyBulkCString(c, "sending_msgs");
    addReplyBulkLongLong(c, ac->sending_msgs);

    fields ++; addReplyBulkCString(c, "blocked_clients");
    addReplyBulkLongLong(c, listLength(ac->blocked_list));

    fields ++; addReplyBulkCString(c, "batched_iterator");
    batchedObjectIteratorStatus(c, ac->batched_iter);

    setDeferredMultiBulkLength(c, ptr, fields * 2);
}

/* ============================ SlotsmgrtExecWrapper ======================================= */

/* *
 * SLOTSMGRT-EXEC-WRAPPER $hashkey $command [$arg1 ...]
 * */
void
slotsmgrtExecWrapperCommand(client *c) {
    addReplyMultiBulkLen(c, 2);
    if (c->argc < 3) {
        addReplyLongLong(c, -1);
        addReplyError(c, "wrong number of arguments for SLOTSMGRT-EXEC-WRAPPER");
        return;
    }
    struct redisCommand *cmd = lookupCommand(c->argv[2]->ptr);
    if (cmd == NULL) {
        addReplyLongLong(c, -1);
        addReplyErrorFormat(c,"invalid command specified (%s)",
                (char *)c->argv[2]->ptr);
        return;
    }
    if ((cmd->arity > 0 && cmd->arity != c->argc - 2) || (c->argc - 2 < -cmd->arity)) {
        addReplyLongLong(c, -1);
        addReplyErrorFormat(c, "wrong number of arguments for command (%s)",
                (char *)c->argv[2]->ptr);
        return;
    }
    if (lookupKeyWrite(c->db, c->argv[1]) == NULL) {
        addReplyLongLong(c, 0);
        addReplyError(c, "the specified key doesn't exist");
        return;
    }
    if (!(cmd->flags & CMD_READONLY) && getSlotsmgrtAsyncClientMigrationStatusOrBlock(c, c->argv[1], 0) != 0) {
        addReplyLongLong(c, 1);
        addReplyError(c, "the specified key is being migrated");
        return;
    } else {
        addReplyLongLong(c, 2);
        robj **argv = zmalloc(sizeof(robj *) * (c->argc - 2));
        for (int i = 2; i < c->argc; i ++) {
            argv[i - 2] = c->argv[i];
            incrRefCount(c->argv[i]);
        }
        for (int i = 0; i < c->argc; i ++) {
            decrRefCount(c->argv[i]);
        }
        zfree(c->argv);
        c->argc = c->argc - 2;
        c->argv = argv;
        c->cmd = cmd;
        call(c, CMD_CALL_FULL & ~CMD_CALL_PROPAGATE);
    }
}

/* ============================ SlotsrestoreAsync Commands ================================= */

static void
slotsrestoreReplyAck(client *c, int err_code, const char *fmt, ...) {
    va_list ap;
    va_start(ap, fmt);
    sds s = sdscatvprintf(sdsempty(), fmt, ap);
    va_end(ap);

    addReplyMultiBulkLen(c, 3);
    addReplyBulkCString(c, "SLOTSRESTORE-ASYNC-ACK");
    addReplyBulkLongLong(c, err_code);
    addReplyBulkSds(c, s);

    if (err_code != 0) {
        c->flags |= CLIENT_CLOSE_AFTER_REPLY;
    }
}

extern int verifyDumpPayload(unsigned char *p, size_t len);

static int
slotsrestoreAsyncHandle(client *c) {
    if (getSlotsmgrtAsyncClientMigrationStatusOrBlock(c, NULL, 0) != 0) {
        slotsrestoreReplyAck(c, -1, "the specified DB is being migrated");
        return C_ERR;
    }

    const char *cmd = "";
    if (c->argc < 2) {
        goto bad_arguments_number;
    }
    cmd = c->argv[1]->ptr;

    /* ==================================================== */
    /* SLOTSRESTORE-ASYNC $cmd $key [$ttl $arg1, $arg2 ...] */
    /* ==================================================== */

    if (c->argc < 3) {
        goto bad_arguments_number;
    }

    robj *key = c->argv[2];

    /* SLOTSRESTORE-ASYNC delete $key */
    if (!strcasecmp(cmd, "delete")) {
        if (c->argc != 3) {
            goto bad_arguments_number;
        }
        int deleted = dbDelete(c->db, key);
        if (deleted) {
            signalModifiedKey(c->db, key);
            server.dirty ++;
        }
        slotsrestoreReplyAck(c, 0, deleted ? "1" : "0");
        return C_OK;
    }

    /* ==================================================== */
    /* SLOTSRESTORE-ASYNC $cmd $key $ttl [$arg1, $arg2 ...] */
    /* ==================================================== */

    if (c->argc < 4) {
        goto bad_arguments_number;
    }

    long long ttl;
    if (getLongLongFromObject(c->argv[3], &ttl) != C_OK || ttl < 0) {
        slotsrestoreReplyAck(c, -1, "invalid TTL value (TTL=%s)", c->argv[3]->ptr);
        return C_ERR;
    }

    /* SLOTSRESTORE-ASYNC expire $key $ttl */
    if (!strcasecmp(cmd, "expire")) {
        if (c->argc != 4) {
            goto bad_arguments_number;
        }
        if (lookupKeyWrite(c->db, key) == NULL) {
            slotsrestoreReplyAck(c, -1, "the specified key doesn't exist (%s)", key->ptr);
            return C_ERR;
        }
        slotsrestoreReplyAck(c, 0, "1");
        goto success_common;
    }

    /* SLOTSRESTORE-ASYNC string $key $ttl $payload */
    if (!strcasecmp(cmd, "string")) {
        if (c->argc != 5) {
            goto bad_arguments_number;
        }
        if (lookupKeyWrite(c->db, key) != NULL) {
            slotsrestoreReplyAck(c, -1, "the specified key already exists (%s)", key->ptr);
            return C_ERR;
        }
        robj *val = c->argv[4] = tryObjectEncoding(c->argv[4]);
        dbAdd(c->db, key, val);
        incrRefCount(val);
        slotsrestoreReplyAck(c, 0, "1");
        goto success_common;
    }

    /* SLOTSRESTORE-ASYNC object $key $ttl $payload */
    if (!strcasecmp(cmd, "object")) {
        if (c->argc != 5) {
            goto bad_arguments_number;
        }
        if (lookupKeyWrite(c->db, key) != NULL) {
            slotsrestoreReplyAck(c, -1, "the specified key already exists (%s)", key->ptr);
            return C_ERR;
        }
        void *bytes = c->argv[4]->ptr;
        rio payload;
        if (verifyDumpPayload(bytes, sdslen(bytes)) != C_OK) {
            slotsrestoreReplyAck(c, -1, "invalid payload checksum");
            return C_ERR;
        }
        rioInitWithBuffer(&payload, bytes);
        int type = rdbLoadObjectType(&payload);
        if (type == -1) {
            slotsrestoreReplyAck(c, -1, "invalid payload type");
            return C_ERR;
        }
        robj *val = rdbLoadObject(type, &payload);
        if (val == NULL) {
            slotsrestoreReplyAck(c, -1, "invalid payload body");
            return C_ERR;
        }
        dbAdd(c->db, key, val);
        slotsrestoreReplyAck(c, 0, "1");
        goto success_common;
    }

    /* ========================================================== */
    /* SLOTSRESTORE-ASYNC $cmd $key $ttl $hint [$arg1, $arg2 ...] */
    /* ========================================================== */

    if (c->argc < 5) {
        goto bad_arguments_number;
    }

    long long hint;
    if (getLongLongFromObject(c->argv[4], &hint) != C_OK || hint < 0) {
        slotsrestoreReplyAck(c, -1, "invalid Hint value (Hint=%s)", c->argv[4]->ptr);
        return C_ERR;
    }

    int xargc = c->argc - 5;
    robj **xargv = &c->argv[5];

    /* SLOTSRESTORE-ASYNC list $key $ttl $hint [$elem1 ...] */
    if (!strcasecmp(cmd, "list")) {
        robj *val = lookupKeyWrite(c->db, key);
        if (val != NULL) {
            if (val->type != OBJ_LIST || val->encoding != OBJ_ENCODING_QUICKLIST) {
                slotsrestoreReplyAck(c, -1, "wrong type (expect=%d/%d,got=%d/%d)",
                        OBJ_LIST, OBJ_ENCODING_QUICKLIST, val->type, val->encoding);
                return C_ERR;
            }
        } else {
            if (xargc == 0) {
                slotsrestoreReplyAck(c, -1, "the specified key doesn't exist (%s)", key->ptr);
                return C_ERR;
            }
            val = createQuicklistObject();
            quicklistSetOptions(val->ptr, server.list_max_ziplist_size,
                    server.list_compress_depth);
            dbAdd(c->db, key, val);
        }
        for (int i = 0; i < xargc; i ++) {
            xargv[i] = tryObjectEncoding(xargv[i]);
            listTypePush(val, xargv[i], LIST_TAIL);
        }
        slotsrestoreReplyAck(c, 0, "%d", listTypeLength(val));
        goto success_common;
    }

    /* SLOTSRESTORE-ASYNC hash $key $ttl $hint [$hkey1 $hval1 ...] */
    if (!strcasecmp(cmd, "hash")) {
        if (xargc % 2 != 0) {
            goto bad_arguments_number;
        }
        robj *val = lookupKeyWrite(c->db, key);
        if (val != NULL) {
            if (val->type != OBJ_HASH || val->encoding != OBJ_ENCODING_HT) {
                slotsrestoreReplyAck(c, -1, "wrong type (expect=%d/%d,got=%d/%d)",
                        OBJ_HASH, OBJ_ENCODING_HT, val->type, val->encoding);
                return C_ERR;
            }
        } else {
            if (xargc == 0) {
                slotsrestoreReplyAck(c, -1, "the specified key doesn't exist (%s)", key->ptr);
                return C_ERR;
            }
            val = createHashObject();
            if (val->encoding !=  OBJ_ENCODING_HT) {
                hashTypeConvert(val, OBJ_ENCODING_HT);
            }
            dbAdd(c->db, key, val);
        }
        if (hint != 0) {
            dict *ht = val->ptr;
            dictExpand(ht, hint);
        }
        for (int i = 0; i < xargc; i += 2) {
            hashTypeTryObjectEncoding(val, &xargv[i], &xargv[i + 1]);
            hashTypeSet(val, xargv[i], xargv[i + 1]);
        }
        slotsrestoreReplyAck(c, 0, "%d", hashTypeLength(val));
        goto success_common;
    }

    /* SLOTSRESTORE-ASYNC dict $key $ttl $hint [$elem1 ...] */
    if (!strcasecmp(cmd, "dict")) {
        robj *val = lookupKeyWrite(c->db, key);
        if (val != NULL) {
            if (val->type != OBJ_SET || val->encoding != OBJ_ENCODING_HT) {
                slotsrestoreReplyAck(c, -1, "wrong type (expect=%d/%d,got=%d/%d)",
                        OBJ_SET, OBJ_ENCODING_HT, val->type, val->encoding);
                return C_ERR;
            }
        } else {
            if (xargc == 0) {
                slotsrestoreReplyAck(c, -1, "the specified key doesn't exist (%s)", key->ptr);
                return C_ERR;
            }
            val = createSetObject();
            if (val->encoding != OBJ_ENCODING_HT) {
                setTypeConvert(val, OBJ_ENCODING_HT);
            }
            dbAdd(c->db, key, val);
        }
        if (hint != 0) {
            dict *ht = val->ptr;
            dictExpand(ht, hint);
        }
        for (int i = 0; i < xargc; i ++) {
            xargv[i] = tryObjectEncoding(xargv[i]);
            setTypeAdd(val, xargv[i]);
        }
        slotsrestoreReplyAck(c, 0, "%d", setTypeSize(val));
        goto success_common;
    }

    /* SLOTSRESTORE-ASYNC zset $key $ttl $hint [$elem1 $score1 ...] */
    if (!strcasecmp(cmd, "zset")) {
        if (xargc % 2 != 0) {
            goto bad_arguments_number;
        }
        double *scores = zmalloc(sizeof(double) * xargc / 2);
        for (int i = 1, j = 0; i < xargc; i += 2, j ++) {
            uint64_t bits;
            if (getUint64FromRawStringObject(xargv[i], &bits) != C_OK) {
                zfree(scores);
                slotsrestoreReplyAck(c, -1, "invalid zset score ([%d]), bad raw bits", j);
                return C_ERR;
            }
            scores[j] = convertRawBitsToDouble(bits);
        }
        robj *val = lookupKeyWrite(c->db, key);
        if (val != NULL) {
            if (val->type != OBJ_ZSET || val->encoding != OBJ_ENCODING_SKIPLIST) {
                zfree(scores);
                slotsrestoreReplyAck(c, -1, "wrong type (expect=%d/%d,got=%d/%d)",
                        OBJ_ZSET, OBJ_ENCODING_SKIPLIST, val->type, val->encoding);
                return C_ERR;
            }
        } else {
            if (xargc == 0) {
                zfree(scores);
                slotsrestoreReplyAck(c, -1, "the specified key doesn't exist (%s)", key->ptr);
                return C_ERR;
            }
            val = createZsetObject();
            if (val->encoding != OBJ_ENCODING_SKIPLIST) {
                zsetConvert(val, OBJ_ENCODING_SKIPLIST);
            }
            dbAdd(c->db, key, val);
        }
        zset *zset = val->ptr;
        if (hint != 0) {
            dict *ht = zset->dict;
            dictExpand(ht, hint);
        }
        for (int i = 0, j = 0; i < xargc; i += 2, j ++) {
            robj *elem = xargv[i] = tryObjectEncoding(xargv[i]);
            dictEntry *de = dictFind(zset->dict, elem);
            if (de != NULL) {
                double score = *(double *)dictGetVal(de);
                zslDelete(zset->zsl, score, elem);
                dictDelete(zset->dict, elem);
            }
            zskiplistNode *znode = zslInsert(zset->zsl, scores[j], elem);
            incrRefCount(elem);
            dictAdd(zset->dict, elem, &(znode->score));
            incrRefCount(elem);
        }
        zfree(scores);
        slotsrestoreReplyAck(c, 0, "%d", zsetLength(val));
        goto success_common;
    }

    slotsrestoreReplyAck(c, -1, "unknown command (argc=%d,cmd=%s)", c->argc, cmd);
    return C_ERR;

bad_arguments_number:
    slotsrestoreReplyAck(c, -1, "wrong number of arguments (argc=%d,cmd=%s)", c->argc, cmd);
    return C_ERR;

success_common:
    if (ttl != 0) {
        setExpire(c->db, key, mstime() + ttl);
    } else {
        removeExpire(c->db, key);
    }
    signalModifiedKey(c->db, key);
    server.dirty ++;
    return C_OK;
}


/* *
 * SLOTSRESTORE-ASYNC delete $key
 *                    expire $key $ttl
 *                    object $key $ttl $payload
 *                    string $key $ttl $payload
 *                    list   $key $ttl $hint [$elem1 ...]
 *                    hash   $key $ttl $hint [$hkey1 $hval1 ...]
 *                    dict   $key $ttl $hint [$elem1 ...]
 *                    zset   $key $ttl $hint [$elem1 $score1 ...]
 * */
void
slotsrestoreAsyncCommand(client *c) {
    if (slotsrestoreAsyncHandle(c) != C_OK) {
        c->flags |= CLIENT_CLOSE_AFTER_REPLY;
    }
}

static int
slotsrestoreAsyncAckHandle(client *c) {
    slotsmgrtAsyncClient *ac = getSlotsmgrtAsyncClient(c->db->id);
    if (ac->c != c) {
        addReplyErrorFormat(c, "invalid client, permission denied");
        return C_ERR;
    }
    if (c->argc != 3) {
        addReplyError(c, "wrong number of arguments for SLOTSRESTORE-ASYNC-ACK");
        return C_ERR;
    }
    long long errcode;
    if (getLongLongFromObject(c->argv[1], &errcode) != C_OK) {
        addReplyErrorFormat(c, "invalid errcode (%s)",
                (char *)c->argv[1]->ptr);
        return C_ERR;
    }
    const char *errmsg = c->argv[2]->ptr;
    if (errcode != 0) {
        serverLog(LL_WARNING, "slotsmgrt_async: ack[%d] %s",
                (int)errcode, errmsg != NULL ? errmsg : "(null)");
        return C_ERR;
    }
    if (ac->batched_iter == NULL) {
        serverLog(LL_WARNING, "slotsmgrt_async: null batched iterator");
        addReplyError(c, "invalid iterator (NULL)");
        return C_ERR;
    }
    if (ac->sending_msgs == 0) {
        serverLog(LL_WARNING, "slotsmgrt_async: invalid message counter");
        addReplyError(c, "invalid pending messages");
        return C_ERR;
    }

    ac->lastuse = mstime();
    ac->sending_msgs -= 1;
    ac->sending_msgs += slotsmgrtAsyncNextMessagesMicroseconds(ac, 2, 10);

    if (ac->sending_msgs != 0) {
        return C_OK;
    }
    notifySlotsmgrtAsyncClient(ac, NULL);

    batchedObjectIterator *it = ac->batched_iter;
    if (listLength(it->removed_keys) != 0) {
        list *ll = it->removed_keys;
        for (int i = 0; i < c->argc; i ++) {
            decrRefCount(c->argv[i]);
        }
        zfree(c->argv);
        c->argc = 1 + listLength(ll);
        c->argv = zmalloc(sizeof(robj *) * c->argc);
        for (int i = 1; i < c->argc; i ++) {
            listNode *head = listFirst(ll);
            robj *key = listNodeValue(head);
            if (dbDelete(c->db, key)) {
                signalModifiedKey(c->db, key);
                server.dirty ++;
            }
            c->argv[i] = key;
            incrRefCount(key);
            listDelNode(ll, head);
        }
        c->argv[0] = createStringObject("DEL", 3);
    }

    if (listLength(it->chunked_vals) != 0) {
        list *ll = it->chunked_vals;
        while (listLength(ll) != 0) {
            listNode *head = listFirst(ll);
            robj *o = listNodeValue(head);
            incrRefCount(o);
            listDelNode(ll, head);
            if (o->refcount != 1) {
                decrRefCount(o);
            } else {
                lazyReleaseObject(o);
            }
        }
    }

    ac->batched_iter = NULL;
    freeBatchedObjectIterator(it);
    return C_OK;
}

/* *
 * SLOTSRESTORE-ASYNC-ACK $errno $message
 * */
void
slotsrestoreAsyncAckCommand(client *c) {
    if (slotsrestoreAsyncAckHandle(c) != C_OK) {
        c->flags |= CLIENT_CLOSE_AFTER_REPLY;
    }
}

extern int time_independent_strcmp(const char *a, const char *b);

/* *
 * SLOTSRESTORE-ASYNC-AUTH $passwd
 * */
void
slotsrestoreAsyncAuthCommand(client *c) {
    if (!server.requirepass) {
        slotsrestoreReplyAck(c, -1, "Client sent AUTH, but no password is set");
        return;
    }
    if (!time_independent_strcmp(c->argv[1]->ptr, server.requirepass)) {
        c->authenticated = 1;
        slotsrestoreReplyAck(c, 0, "OK");
    } else {
        c->authenticated = 0;
        slotsrestoreReplyAck(c, -1, "invalid password");
    }
}

/* *
 * SLOTSRESTORE-ASYNC-SELECT $db
 * */
void
slotsrestoreAsyncSelectCommand(client *c) {
    long long db;
    if (getLongLongFromObject(c->argv[1], &db) != C_OK ||
            !(db >= 0 && db <= INT_MAX) || selectDb(c, db) != C_OK) {
        slotsrestoreReplyAck(c, -1, "invalid DB index (%s)", c->argv[1]->ptr);
    } else {
        slotsrestoreReplyAck(c, 0, "OK");
    }
}
