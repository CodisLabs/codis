#!/usr/bin/env bash

/opt/codis/bin/codis-admin --dashboard=127.0.0.1:18080 --reinit-proxy --addr=127.0.0.1:11080
/opt/codis/bin/codis-admin --dashboard=127.0.0.1:18080 --reinit-proxy --addr=127.0.0.1:11081
/opt/codis/bin/codis-admin --dashboard=127.0.0.1:18080 --reinit-proxy --addr=127.0.0.1:11082
