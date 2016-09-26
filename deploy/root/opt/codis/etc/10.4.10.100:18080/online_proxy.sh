#!/usr/bin/env bash

/opt/codis/bin/codis-admin --dashboard=10.4.10.100:18080 --online-proxy --addr=10.2.16.200:11080
/opt/codis/bin/codis-admin --dashboard=10.4.10.100:18080 --online-proxy --addr=10.2.16.201:11080
/opt/codis/bin/codis-admin --dashboard=10.4.10.100:18080 --online-proxy --addr=10.2.16.202:11080
/opt/codis/bin/codis-admin --dashboard=10.4.10.100:18080 --online-proxy --addr=10.4.10.200:11080
/opt/codis/bin/codis-admin --dashboard=10.4.10.100:18080 --online-proxy --addr=10.4.10.201:11080
