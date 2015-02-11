# gorocks

gorocks is a Go wrapper for rocksdb based on [levigo](http://godoc.org/github.com/jmhodges/levigo).

## Building

You'll need the shared library build of
[rocksdb](http://code.google.com/p/rocksdb/) installed on your machine. The
current rocksdb will build it by default.

The minimum version of rocksdb required is currently 1.7. If you require the
use of an older version of rocksdb, see the [fork of levigo for rocksdb
1.4](https://github.com/jmhodges/levigo_rocksdb_1.4). Prefer putting in the
work to be up to date as rocksdb moves very quickly.

Now, if you build rocksdb and put the shared library and headers in one of the
standard places for your OS, you'll be able to simply run:

    go get github.com/tobyhede/gorocks

But, suppose you put the shared rocksdb library somewhere weird like
/path/to/lib and the headers were installed in /path/to/include. To install
levigo remotely, you'll run:

    CGO_CFLAGS="-I/path/to/rocksdb/include" CGO_LDFLAGS="-L/path/to/rocksdb/lib" go get github.com/jmhodges/levigo

and there you go.

In order to build with snappy, you'll have to explicitly add "-lsnappy" to the
`CGO_LDFLAGS`. Supposing that both snappy and rocksdb are in weird places,
you'll run something like:

    CGO_CFLAGS="-I/path/to/rocksdb/include -I/path/to/snappy/include"
    CGO_LDFLAGS="-L/path/to/rocksdb/lib -L/path/to/snappy/lib -lsnappy" go get github.com/jmhodges/levigo

(and make sure the -lsnappy is after the snappy library path!).

Of course, these same rules apply when doing `go build`, as well.

## Caveats

Comparators and WriteBatch iterators must be written in C in your own
library. This seems like a pain in the ass, but remember that you'll have the
rocksdb C API available to your in your client package when you import levigo.

An example of writing your own Comparator can be found in
<https://github.com/jmhodges/levigo/blob/master/examples>.
