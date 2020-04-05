# Function `zslDelete` add a param
- Src:
```
int zslDelete(zskiplist *zsl, double score, robj *obj);
```
- Target:
```
void zslDeleteNode(zskiplist *zsl, zskiplistNode *x, zskiplistNode **update);
```

- Example:
```
-- zslDelete(db->tagged_keys, (double)crc, key);
++ zslDelete(db->tagged_keys, (double)crc, key->ptr, NULL);
```

# Function `dictScan` add a param
- Src:
```
unsigned long dictScan(dict *d,
                       unsigned long v,
                       dictScanFunction *fn,
                       void *privdata);
```
- Target:
```
unsigned long dictScan(dict *d,
					unsigned long v,
					dictScanFunction *fn,
					dictScanBucketFunction* bucketfn,
					void *privdata);
```
- Example:
```
-- it->cursor = dictScan(ht, it->cursor, singleObjectIteratorScanCallback, pd);
++ it->cursor = dictScan(ht, it->cursor, singleObjectIteratorScanCallback, NULL, pd);
```

# Function `rdbLoadObject` add a param

- Src:
```
robj *rdbLoadObject(int rdbtype, rio *rdb);
```
- Target:
```
robj *rdbLoadObject(int rdbtype, rio *rdb, robj *key);
```
- Evample:
```
-- vals[i] = rdbLoadObject(type, &payload)
++ vals[i] = rdbLoadObject(type, &payload, NULL)
```

# Function `setExpire` add a param
- Src:
```
void setExpire(redisDb *db, robj *key, long long when);
```

- Target:
```
void setExpire(client *c, redisDb *db, robj *key, long long when);
```

- Example:
```
-- setExpire(c->db, key, mstime() + ttl);
++ setExpire(c, c->db, key, mstime() + ttl);
```

# Function `hashTypeSet` add a param

- Src:
```
int hashTypeSet(robj *o, robj *field, robj *value);
```

- Target:
```
int hashTypeSet(robj *o, sds field, sds value, int flags);
```