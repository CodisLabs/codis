#include "server.h"

/* ============================ Iterator for Lazy Release ================================== */

typedef struct {
    robj *val;
    unsigned long cursor;
} lazyReleaseIterator;

static lazyReleaseIterator *
createLazyReleaseIterator(robj *val) {
    lazyReleaseIterator *it = zmalloc(sizeof(lazyReleaseIterator));
    it->val = val;
    incrRefCount(it->val);
    it->cursor = 0;
    return it;
}

static void
freeLazyReleaseIterator(lazyReleaseIterator *it) {
    if (it->val != NULL) {
        decrRefCount(it->val);
    }
    zfree(it);
}

static int
lazyReleaseIteratorHasNext(lazyReleaseIterator *it) {
    return it->val != NULL;
}

static void
lazyReleaseIteratorScanCallback(void *data, const dictEntry *de) {
    void **pd = (void **)data;
    list *l = pd[0];

    robj *field = dictGetKey(de);
    incrRefCount(field);
    listAddNodeTail(l, field);
}

static void
lazyReleaseIteratorNext(lazyReleaseIterator *it) {
    robj *val = it->val;
    serverAssert(val != NULL);

    const int step = 4096;

    if (val->type == OBJ_LIST) {
        if (listTypeLength(val) <= step * 2) {
            decrRefCount(val);
            it->val = NULL;
        } else {
            for (int i = 0; i < step; i ++) {
                robj *value = listTypePop(val, LIST_HEAD);
                decrRefCount(value);
            }
        }
        return;
    }

    if (val->type == OBJ_HASH || val->type == OBJ_SET) {
        dict *ht = val->ptr;
        if (dictSize(ht) <= step * 2) {
            decrRefCount(val);
            it->val = NULL;
        } else {
            list *ll = listCreate();
            listSetFreeMethod(ll, decrRefCountVoid);
            int loop = step * 10;
            void *pd[] = {ll};
            do {
                it->cursor = dictScan(ht, it->cursor, lazyReleaseIteratorScanCallback, pd);
            } while (it->cursor != 0 && listLength(ll) < (unsigned long)step && (-- loop) >= 0);

            while (listLength(ll) != 0) {
                listNode *head = listFirst(ll);
                robj *field = listNodeValue(head);
                dictDelete(ht, field);
                listDelNode(ll, head);
            }
            listRelease(ll);
        }
        return;
    }

    if (val->type == OBJ_ZSET) {
        zset *zs = val->ptr;
        dict *ht = zs->dict;
        if (dictSize(ht) <= step * 2) {
            decrRefCount(val);
            it->val = NULL;
        } else {
            zskiplist *zsl = zs->zsl;
            for (int i = 0; i < step; i ++) {
                zskiplistNode *node = zsl->header->level[0].forward;
                robj *field = node->obj;
                incrRefCount(field);
                zslDelete(zsl, node->score, field);
                dictDelete(ht, field);
                decrRefCount(field);
            }
        }
        return;
    }

    serverPanic("unknown object type");
}

static int
lazyReleaseIteratorRemains(lazyReleaseIterator *it) {
    robj *val = it->val;
    if (val == NULL) {
        return 0;
    }
    if (val->type == OBJ_LIST) {
        return listTypeLength(val);
    }
    if (val->type == OBJ_HASH) {
        return hashTypeLength(val);
    }
    if (val->type == OBJ_SET) {
        return setTypeSize(val);
    }
    if (val->type == OBJ_ZSET) {
        return zsetLength(val);
    }
    return -1;
}

int
slotsmgrtLazyReleaseIncrementally() {
    list *ll = server.slotsmgrt_lazy_release;
    if (listLength(ll) != 0) {
        listNode *head = listFirst(ll);
        lazyReleaseIterator *it = listNodeValue(head);
        if (lazyReleaseIteratorHasNext(it)) {
            lazyReleaseIteratorNext(it);
        } else {
            freeLazyReleaseIterator(it);
            listDelNode(ll, head);
        }
        return 1;
    }
    return 0;
}

void
slotsmgrtLazyReleaseCommand(client *c) {
    if (c->argc != 1 && c->argc != 2) {
        addReplyError(c, "wrong number of arguments for SLOTSMGRT-LAZY-RELEASE");
        return;
    }
    long long step = 1;
    if (c->argc != 1) {
        if (getLongLongFromObject(c->argv[1], &step) != C_OK ||
                !(step >= 0 && step <= INT_MAX)) {
            addReplyErrorFormat(c, "invalid value of step (%s)",
                    (char *)c->argv[1]->ptr);
            return;
        }
    }
    while (step != 0 && slotsmgrtLazyReleaseIncrementally()) { step --; }

    list *ll = server.slotsmgrt_lazy_release;

    addReplyMultiBulkLen(c, 2);
    addReplyLongLong(c, listLength(ll));

    if (listLength(ll) != 0) {
        lazyReleaseIterator *it = listNodeValue(listFirst(ll));
        addReplyLongLong(c, lazyReleaseIteratorRemains(it));
    } else {
        addReplyLongLong(c, 0);
    }
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
    int chunked;
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
    it->chunked = 0;
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

        int extra_msgs = 0;

        slotsmgrtAsyncClient *client = getSlotsmgrtAsyncClient(c->db->id);
        if (client->c == c) {
            if (client->used == 0) {
                client->used = 1;
                if (server.requirepass != NULL) {
                    /* SLOTSRESTORE-ASYNC-AUTH $password */
                    addReplyMultiBulkLen(c, 2);
                    addReplyBulkCString(c, "SLOTSRESTORE-ASYNC-AUTH");
                    addReplyBulkCString(c, server.requirepass);
                    extra_msgs += 1;
                }
                do {
                    /* SLOTSRESTORE-ASYNC select $db */
                    addReplyMultiBulkLen(c, 3);
                    addReplyBulkCString(c, "SLOTSRESTORE-ASYNC");
                    addReplyBulkCString(c, "select");
                    addReplyBulkLongLong(c, c->db->id);
                    extra_msgs += 1;
                } while (0);
            }
        }

        /* SLOTSRESTORE-ASYNC del $key */
        addReplyMultiBulkLen(c, 3);
        addReplyBulkCString(c, "SLOTSRESTORE-ASYNC");
        addReplyBulkCString(c, "del");
        addReplyBulk(c, key);

        unsigned long limits = maxbulks * 2;

        switch (val->type) {
        case OBJ_LIST:
            it->chunked = (val->encoding == OBJ_ENCODING_QUICKLIST) &&
                (limits < listTypeLength(val));
            break;
        case OBJ_HASH:
            it->chunked = (val->encoding == OBJ_ENCODING_HT) &&
                (limits < hashTypeLength(val));
            break;
        case OBJ_SET:
            it->chunked = (val->encoding == OBJ_ENCODING_HT) &&
                (limits < setTypeSize(val));
            break;
        case OBJ_ZSET:
            it->chunked = (val->encoding == OBJ_ENCODING_SKIPLIST) &&
                (limits < zsetLength(val));
            break;
        }

        it->stage = it->chunked ? STAGE_CHUNKED : STAGE_PAYLOAD;
        return 1 + extra_msgs;
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

    if (it->stage == STAGE_PAYLOAD) {
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
            if (loop < 128) {
                loop = 128;
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
                    robj *score = createStringObjectFromLongDouble(node->score, 0);
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
            if (sp->chunked) {
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
batchedObjectIteratorAddKey(batchedObjectIterator *it, robj *key) {
    if (dictAdd(it->keys, key, NULL) != C_OK) {
        return 0;
    }
    incrRefCount(key);
    listAddNodeTail(it->list, createSingleObjectIterator(key));

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
    }

out:
    return 1 + dictSize(it->keys) - size;
}

static void
batchedObjectIteratorAddKeyCallback(void *data, const dictEntry *de) {
    void **pd = (void **)data;
    batchedObjectIterator *it = pd[0];

    sds skey = dictGetKey(de);
    robj *key = createStringObject(skey, sdslen(skey));
    batchedObjectIteratorAddKey(it, key);
    decrRefCount(key);
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
            "pending_msgs = %ld, batched_iter = %ld, blocked_list = %ld, "
            "timeout = %lld(ms), elapsed = %lld(ms) (%s)",
            ac->host, ac->port, c->db->id, ac->pending_msgs, it != NULL ? (long)listLength(it->list) : -1,
            (long)listLength(ac->blocked_list), ac->timeout, elapsed, errmsg);

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
    ac->pending_msgs = 0;
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
    slotsmgrtLazyReleaseIncrementally();
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
        batchedObjectIteratorAddKey(it, c->argv[i]);
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
        maxbulks = 1000;
    }
    if (maxbulks > 1024 * 512) {
        maxbulks = 1024 * 512;
    }
    long long maxbytes;
    if (getLongLongFromObject(c->argv[5], &maxbytes) != C_OK ||
            !(maxbytes >= 0 && maxbytes <= INT_MAX)) {
        addReplyErrorFormat(c, "invalid value of maxbytes (%s)",
                (char *)c->argv[5]->ptr);
        return;
    }
    if (maxbytes == 0) {
        maxbytes = 1024 * 256;
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
            numkeys = 128;
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
        if (dictIsRehashing(hash_slot)) {
            dictRehash(hash_slot, 1);
        } else if (htNeedsResize(hash_slot)) {
            dictResize(hash_slot);
        }
        const int round = 2;
        void *pd[] = {it};
        for (int i = 0; i < round && dictSize(it->keys) == 0; i ++) {
            unsigned long cursor = (i != round - 1) ? random() : 0;
            int loop = numkeys * 5;
            if (loop < 32) {
                loop = 32;
            }
            do {
                cursor = dictScan(hash_slot, cursor, batchedObjectIteratorAddKeyCallback, pd);
            } while (cursor != 0 && dictSize(it->keys) < (unsigned long)numkeys && (-- loop) >= 0);
        }
    } else {
        for (int i = 6; i < c->argc; i ++) {
            batchedObjectIteratorAddKey(it, c->argv[i]);
        }
    }
    serverAssert(ac->pending_msgs == 0);
    serverAssert(ac->batched_iter == NULL && listLength(ac->blocked_list) == 0);

    ac->timeout = timeout;
    ac->lastuse = mstime();
    ac->batched_iter = it;

    while (batchedObjectIteratorHasNext(it) && getClientOutputBufferMemoryUsage(ac->c) < it->maxbytes) {
        ac->pending_msgs += batchedObjectIteratorNext(ac->c, it);
    }
    getSlotsmgrtAsyncClientMigrationStatusOrBlock(c, NULL, 1);

    if (ac->pending_msgs != 0) {
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
slotsrestoreReplyAck(client *c, int errcode, const char *fmt, ...) {
    va_list ap;
    va_start(ap, fmt);
    sds s = sdscatvprintf(sdsempty(), fmt, ap);
    va_end(ap);

    addReplyMultiBulkLen(c, 3);
    addReplyBulkCString(c, "SLOTSRESTORE-ASYNC-ACK");
    addReplyBulkLongLong(c, errcode);
    addReplyBulkSds(c, s);
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

    /* SLOTSRESTORE-ASYNC select $db */
    if (!strcasecmp(cmd, "select")) {
        long long db;
        if (c->argc != 3) {
            goto bad_arguments_number;
        }
        if (getLongLongFromObject(c->argv[2], &db) != C_OK ||
                !(db >= 0 && db <= INT_MAX) || selectDb(c, db) != C_OK) {
            slotsrestoreReplyAck(c, -1, "invalid DB index (DB=%s)", c->argv[2]->ptr);
            return C_ERR;
        }
        slotsrestoreReplyAck(c, 0, "%d", c->db->id);
        return C_OK;
    }

    /* ==================================================== */
    /* SLOTSRESTORE-ASYNC $cmd $key [$ttl $arg1, $arg2 ...] */
    /* ==================================================== */

    if (c->argc < 3) {
        goto bad_arguments_number;
    }

    robj *key = c->argv[2];

    /* SLOTSRESTORE-ASYNC del $key */
    if (!strcasecmp(cmd, "del")) {
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

    /* SLOTSRESTORE-ASYNC object key ttl payload */
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
            long double score;
            if (getLongDoubleFromObject(xargv[i], &score) != C_OK) {
                zfree(scores);
                slotsrestoreReplyAck(c, -1, "invalid zset score ([%d]=%s)", j, xargv[i]->ptr);
                return C_ERR;
            }
            scores[j] = score;
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
 * SLOTSRESTORE-ASYNC select $db
 *                    del    $key
 *                    expire $key $ttl
 *                    object $key $ttl $payload
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
    if (ac->pending_msgs == 0) {
        serverLog(LL_WARNING, "slotsmgrt_async: invalid message counter");
        addReplyError(c, "invalid pending messages");
        return C_ERR;
    }

    ac->lastuse = mstime();
    ac->pending_msgs -= 1;

    batchedObjectIterator *it = ac->batched_iter;
    while (batchedObjectIteratorHasNext(it) && getClientOutputBufferMemoryUsage(ac->c) < it->maxbytes) {
        ac->pending_msgs += batchedObjectIteratorNext(ac->c, it);
    }

    if (ac->pending_msgs != 0) {
        return C_OK;
    }
    notifySlotsmgrtAsyncClient(ac, NULL);

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
            robj *val = listNodeValue(head);
            listAddNodeTail(server.slotsmgrt_lazy_release, createLazyReleaseIterator(val));
            listDelNode(ll, head);
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

