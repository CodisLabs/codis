#!/usr/bin/env python

import json

servers = [
    "127.0.0.1:16379",
    "127.0.0.1:16380",
    "127.0.0.1:16381",
    "127.0.0.1:16382",
]

mappings = [x % len(servers) for x in range(0, 1024)]
mappings.sort()

slots = []
for i in range(0, len(mappings)):
    g = mappings[i]
    slots.append({'id': i, 'backend_addr': servers[g]})

print(json.dumps(slots, sort_keys=True, indent=4))

