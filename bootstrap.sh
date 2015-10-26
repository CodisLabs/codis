#!/bin/sh

make clean

go get -u github.com/tools/godep

echo "downloading dependcies, it may take a few minutes..."

godep path > /dev/null
if [ "$?" = 0 ]; then
    GOPATH=`godep path`:$GOPATH
    godep restore || exit $?
    make || exit $?
    make gotest || exit $?
    exit 0
fi

exit $?