package gorocks

// #cgo LDFLAGS: -lrocksdb
// #include "rocksdb/c.h"
import "C"

type TableOptions struct {
	Opt *C.rocksdb_block_based_table_options_t
}

// NewTableOptions allocates a new Options object.
func NewTableOptions() *TableOptions {
	opt := C.rocksdb_block_based_options_create()
	return &TableOptions{opt}
}

// Close deallocates the TableOptions, freeing its underlying C struct.
func (o *TableOptions) Close() {
	C.rocksdb_block_based_options_destroy(o.Opt)
}

// SetCache places a cache object in the database when a database is opened.
//
// This is usually wise to use. See also ReadOptions.SetFillCache.
func (o *TableOptions) SetCache(cache *Cache) {
	C.rocksdb_block_based_options_set_block_cache(o.Opt, cache.Cache)
}

// SetBlockRestartInterval is the number of keys between restarts points for
// delta encoding keys.
//
// Most clients should leave this parameter alone. See the rocksdb
// documentation for details.
func (o *TableOptions) SetBlockRestartInterval(n int) {
	C.rocksdb_block_based_options_set_block_restart_interval(o.Opt, C.int(n))
}

// SetBlockSize sets the approximate size of user data packed per block.
//
// The default is roughly 4096 uncompressed bytes. A better setting depends on
// your use case. See the rocksdb documentation for details.
func (o *TableOptions) SetBlockSize(s int) {
	C.rocksdb_block_based_options_set_block_size(o.Opt, C.size_t(s))
}

// SetFilterPolicy causes Open to create a new database that will uses filter
// created from the filter policy passed in.
func (o *TableOptions) SetFilterPolicy(fp *FilterPolicy) {
	var policy *C.rocksdb_filterpolicy_t
	if fp != nil {
		policy = fp.Policy
	}
	C.rocksdb_block_based_options_set_filter_policy(o.Opt, policy)
}
