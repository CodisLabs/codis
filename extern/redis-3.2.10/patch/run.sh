#!/bin/bash

set -e

apply_patch() {
    echo "  >>" $@ "<<"
    file=`realpath $1`; shift
    pushd ../ > /dev/null
    patch $@ < $file
    popd > /dev/null
}

patch_patches() {
    dir=$1; shift
    files=`ls -1 $dir/*.patch | sort`
    for i in $files; do
        apply_patch $i $@
    done
}

revert_patches() {
    dir=$1; shift
    files=`ls -1 $dir/*.patch | sort --reverse`
    for i in $files; do
        apply_patch $i $@ -R
    done
}

case "$1" in
patch)
    patch_patches redis -p1
    patch_patches codis -p3
    ;;

revert)
    revert_patches redis -p1
    revert_patches codis -p3
    ;;
*)
    echo "wrong argument(s)"
    ;;
esac


