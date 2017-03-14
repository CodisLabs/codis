#!/bin/bash

files=`ls -1 *.patch | sort`

for i in $files; do
    file=`realpath $i`
    pushd ../
    patch -p1 $@ < $file
    popd
done
