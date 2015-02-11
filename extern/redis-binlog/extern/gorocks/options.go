package gorocks

// #cgo LDFLAGS: -lrocksdb
// #include "rocksdb/c.h"
import "C"

// CompressionOpt is a value for Options.SetCompression.
type CompressionOpt int

// Known compression arguments for Options.SetCompression.
const (
	NoCompression     = CompressionOpt(0)
	SnappyCompression = CompressionOpt(1)
	ZlibCompression   = CompressionOpt(2)
	Bz2Compression    = CompressionOpt(3)
	Lz4Compression    = CompressionOpt(4)
	Lz4hcCompression  = CompressionOpt(5)
)

// Options represent all of the available options when opening a database with
// Open. Options should be created with NewOptions.
//
// It is usually with to call SetCache with a cache object. Otherwise, all
// data will be read off disk.
//
// To prevent memory leaks, Close must be called on an Options when the
// program no longer needs it.
type Options struct {
	Opt *C.rocksdb_options_t
}

// ReadOptions represent all of the available options when reading from a
// database.
//
// To prevent memory leaks, Close must called on a ReadOptions when the
// program no longer needs it.
type ReadOptions struct {
	Opt *C.rocksdb_readoptions_t
}

// WriteOptions represent all of the available options when writeing from a
// database.
//
// To prevent memory leaks, Close must called on a WriteOptions when the
// program no longer needs it.
type WriteOptions struct {
	Opt *C.rocksdb_writeoptions_t
}

// NewOptions allocates a new Options object.
func NewOptions() *Options {
	opt := C.rocksdb_options_create()
	return &Options{opt}
}

// NewReadOptions allocates a new ReadOptions object.
func NewReadOptions() *ReadOptions {
	opt := C.rocksdb_readoptions_create()
	return &ReadOptions{opt}
}

// NewWriteOptions allocates a new WriteOptions object.
func NewWriteOptions() *WriteOptions {
	opt := C.rocksdb_writeoptions_create()
	return &WriteOptions{opt}
}

// Close deallocates the Options, freeing its underlying C struct.
func (o *Options) Close() {
	C.rocksdb_options_destroy(o.Opt)
}

// SetComparator sets the comparator to be used for all read and write
// operations.
//
// The comparator that created a database must be the same one (technically,
// one with the same name string) that is used to perform read and write
// operations.
//
// The default comparator is usually sufficient.
func (o *Options) SetComparator(cmp *C.rocksdb_comparator_t) {
	C.rocksdb_options_set_comparator(o.Opt, cmp)
}

// SetErrorIfExists, if passed true, will cause the opening of a database that
// already exists to throw an error.
func (o *Options) SetErrorIfExists(error_if_exists bool) {
	eie := boolToUchar(error_if_exists)
	C.rocksdb_options_set_error_if_exists(o.Opt, eie)
}

// SetEnv sets the Env object for the new database handle.
func (o *Options) SetEnv(env *Env) {
	C.rocksdb_options_set_env(o.Opt, env.Env)
}

// SetInfoLog sets a *C.rocksdb_logger_t object as the informational logger
// for the database.
func (o *Options) SetInfoLog(log *C.rocksdb_logger_t) {
	C.rocksdb_options_set_info_log(o.Opt, log)
}

// SetWriteBufferSize sets the number of bytes the database will build up in
// memory (backed by an unsorted log on disk) before converting to a sorted
// on-disk file.
func (o *Options) SetWriteBufferSize(s int) {
	C.rocksdb_options_set_write_buffer_size(o.Opt, C.size_t(s))
}

// SetParanoidChecks, when called with true, will cause the database to do
// aggressive checking of the data it is processing and will stop early if it
// detects errors.
//
// See the rocksdb documentation docs for details.
func (o *Options) SetParanoidChecks(pc bool) {
	C.rocksdb_options_set_paranoid_checks(o.Opt, boolToUchar(pc))
}

// SetMaxOpenFiles sets the number of files than can be used at once by the
// database.
//
// See the rocksdb documentation for details.
func (o *Options) SetMaxOpenFiles(n int) {
	C.rocksdb_options_set_max_open_files(o.Opt, C.int(n))
}

// SetBlockSize sets the approximate size of user data packed per block.
//
// The default is roughly 4096 uncompressed bytes. A better setting depends on
// your use case. See the rocksdb documentation for details.
func (o *Options) SetBlockSize(s int) {
	C.rocksdb_options_set_arena_block_size(o.Opt, C.size_t(s))
}

// SetBlockSize sets the approximate size of user data packed per block.
//
// The default is roughly 4096 uncompressed bytes. A better setting depends on
// your use case. See the rocksdb documentation for details.
func (o *Options) SetArenaBlockSize(s int) {
	C.rocksdb_options_set_arena_block_size(o.Opt, C.size_t(s))
}

// SetCompression sets whether to compress blocks using the specified
// compresssion algorithm.
//
// The default value is SnappyCompression and it is fast enough that it is
// unlikely you want to turn it off. The other option is NoCompression.
//
// If the rocksdb library was built without Snappy compression enabled, the
// SnappyCompression setting will be ignored.
func (o *Options) SetCompression(t CompressionOpt) {
	C.rocksdb_options_set_compression(o.Opt, C.int(t))
}

// SetCreateIfMissing causes Open to create a new database on disk if it does
// not already exist.
func (o *Options) SetCreateIfMissing(b bool) {
	C.rocksdb_options_set_create_if_missing(o.Opt, boolToUchar(b))
}

// Close deallocates the ReadOptions, freeing its underlying C struct.
func (ro *ReadOptions) Close() {
	C.rocksdb_readoptions_destroy(ro.Opt)
}

// SetVerifyChecksums controls whether all data read with this ReadOptions
// will be verified against corresponding checksums.
//
// It defaults to false. See the rocksdb documentation for details.
func (ro *ReadOptions) SetVerifyChecksums(b bool) {
	C.rocksdb_readoptions_set_verify_checksums(ro.Opt, boolToUchar(b))
}

// SetFillCache controls whether reads performed with this ReadOptions will
// fill the Cache of the server. It defaults to true.
//
// It is useful to turn this off on ReadOptions for DB.Iterator (and DB.Get)
// calls used in offline threads to prevent bulk scans from flushing out live
// user data in the cache.
//
// See also Options.SetCache
func (ro *ReadOptions) SetFillCache(b bool) {
	C.rocksdb_readoptions_set_fill_cache(ro.Opt, boolToUchar(b))
}

// SetSnapshot causes reads to provided as they were when the passed in
// Snapshot was created by DB.NewSnapshot. This is useful for getting
// consistent reads during a bulk operation.
//
// See the rocksdb documentation for details.
func (ro *ReadOptions) SetSnapshot(snap *Snapshot) {
	var s *C.rocksdb_snapshot_t
	if snap != nil {
		s = snap.snap
	}
	C.rocksdb_readoptions_set_snapshot(ro.Opt, s)
}

// Close deallocates the WriteOptions, freeing its underlying C struct.
func (wo *WriteOptions) Close() {
	C.rocksdb_writeoptions_destroy(wo.Opt)
}

// SetSync controls whether each write performed with this WriteOptions will
// be flushed from the operating system buffer cache before the write is
// considered complete.
//
// If called with true, this will signficantly slow down writes. If called
// with false, and the host machine crashes, some recent writes may be
// lost. The default is false.
//
// See the rocksdb documentation for details.
func (wo *WriteOptions) SetSync(b bool) {
	C.rocksdb_writeoptions_set_sync(wo.Opt, boolToUchar(b))
}

func (o *Options) SetNumLevels(n int) {
	C.rocksdb_options_set_num_levels(o.Opt, C.int(n))
}

func (o *Options) SetMaxWriteBufferNumber(n int) {
	C.rocksdb_options_set_max_write_buffer_number(o.Opt, C.int(n))
}

func (o *Options) SetMinWriteBufferNumberToMerge(n int) {
	C.rocksdb_options_set_min_write_buffer_number_to_merge(o.Opt, C.int(n))
}

func (o *Options) SetLevel0FileNumCompactionTrigger(n int) {
	C.rocksdb_options_set_level0_file_num_compaction_trigger(o.Opt, C.int(n))
}

func (o *Options) SetLevel0SlowdownWritesTrigger(n int) {
	C.rocksdb_options_set_level0_slowdown_writes_trigger(o.Opt, C.int(n))
}

func (o *Options) SetLevel0StopWritesTrigger(n int) {
	C.rocksdb_options_set_level0_stop_writes_trigger(o.Opt, C.int(n))
}

func (o *Options) SetTargetFileSizeBase(n int) {
	C.rocksdb_options_set_target_file_size_base(o.Opt, C.uint64_t(n))
}

func (o *Options) SetTargetFileSizeMultiplier(n int) {
	C.rocksdb_options_set_target_file_size_multiplier(o.Opt, C.int(n))
}

func (o *Options) SetMaxBytesForLevelBase(n int) {
	C.rocksdb_options_set_max_bytes_for_level_base(o.Opt, C.uint64_t(n))
}

func (o *Options) SetMaxBytesForLevelMultiplier(n int) {
	C.rocksdb_options_set_max_bytes_for_level_multiplier(o.Opt, C.int(n))
}

func (o *Options) SetDisableAutoCompactions(b bool) {
	C.rocksdb_options_set_disable_auto_compactions(o.Opt, boolToInt(b))
}

func (o *Options) SetDisableDataSync(b bool) {
	C.rocksdb_options_set_disable_data_sync(o.Opt, boolToInt(b))
}

func (o *Options) SetUseFsync(b bool) {
	C.rocksdb_options_set_use_fsync(o.Opt, boolToInt(b))
}

func (o *Options) SetMaxBackgroundCompactions(n int) {
	C.rocksdb_options_set_max_background_compactions(o.Opt, C.int(n))
}

func (o *Options) SetMaxBackgroundFlushes(n int) {
	C.rocksdb_options_set_max_background_flushes(o.Opt, C.int(n))
}

func (o *Options) SetAllowOSBuffer(b bool) {
	C.rocksdb_options_set_allow_os_buffer(o.Opt, boolToUchar(b))
}

func (o *Options) SetBlockBasedTableFactory(t *TableOptions) {
	C.rocksdb_options_set_block_based_table_factory(o.Opt, t.Opt)
}
