#!/usr/bin/env bash

/opt/codis/bin/codis-admin --proxy=127.0.0.1:11080 $@
/opt/codis/bin/codis-admin --proxy=127.0.0.1:11081 $@
/opt/codis/bin/codis-admin --proxy=127.0.0.1:11082 $@
