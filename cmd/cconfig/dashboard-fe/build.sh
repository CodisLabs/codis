#!/bin/sh

grunt build
rm -rf ../assets/statics/admin
cp -r ./dist ../assets/statics/admin/
