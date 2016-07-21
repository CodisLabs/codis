#!/usr/bin/env python
#encoding:utf-8

import BfdCodis as codis

client = codis.BfdCodis("192.168.161.22:2181", "/zk/codis/db_test23/proxy", "item")

print client.set("key", "value")

value = client.get("key")
