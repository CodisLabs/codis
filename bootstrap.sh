#!/bin/sh

make clean

echo "downloading dependcies, it may take a few minutes..."
# Test godep install, steal it from LedisDB project :P
godep path > /dev/null 2>&1
if [ "$?" = 0 ]; then
    GOPATH=`godep path`:$GOPATH
    godep restore
    make || exit $?
    make gotest
    exit 0
fi

go get ./...

make || exit $?
make gotest
