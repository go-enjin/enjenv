#!/bin/bash

APP_ARGS=

for TOML in $(ls -1 /etc/niseroku/apps.d/*.toml)
do
    NAME=$(basename "${TOML}" ".toml")
    if ls /var/lib/niseroku/slugs.d/ | egrep -q "^${NAME}--.*\.zip"
    then
        INFO_LOG="/var/lib/niseroku/logs.d/${NAME}.info.log"
        ERROR_LOG="/var/lib/niseroku/logs.d/${NAME}.error.log"
        ACCESS_LOG="/var/lib/niseroku/logs.d/${NAME}.access.log"
        APP_ARGS="${APP_ARGS} -t ${NAME} --label [FAIL] -iw ${ERROR_LOG} 1s"
        APP_ARGS="${APP_ARGS} -t ${NAME} --label [INFO] -Iw ${INFO_LOG} 1s"
        APP_ARGS="${APP_ARGS} -t ${NAME} --label [HTTP] -Iw ${ACCESS_LOG} 1s"
    fi
done

exec multitail \
     -x 'niseroku-tail (%h)' \
     -H -T -du -N 1 \
     --basename \
     --no-repeat \
     --retry-all \
     --follow-all \
     --mark-interval 300 \
     /var/log/niseroku.log \
     ${APP_ARGS}
