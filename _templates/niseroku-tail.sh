#!/bin/bash

ONLY_APP=
if [ -n "${1}" ]
then
    case "${1}" in
        "-h"|"--help")
            echo "usage: $(basename $0) [app-name]"
            exit 1
            ;;
        *)
            ONLY_APP="${1}"
            if [ ! -f "/etc/niseroku/apps.d/${ONLY_APP}.toml" ]
            then
                echo "error: \"${ONLY_APP}\" not found"
                exit 1
            fi
            ;;
    esac
fi

APP_ARGS=
TITLE="niseroku-tail (%h)"

if [ -n "${ONLY_APP}" ]
then

    TITLE="niseroku-tail [${ONLY_APP}] (%h)"

    INFO_LOG="/var/lib/niseroku/logs.d/${ONLY_APP}.info.log"
    ERROR_LOG="/var/lib/niseroku/logs.d/${ONLY_APP}.error.log"
    ACCESS_LOG="/var/lib/niseroku/logs.d/${ONLY_APP}.access.log"
    [ -f "${ERROR_LOG}" ]  && APP_ARGS="${APP_ARGS} -t ${ONLY_APP}-[FAIL] -iw ${ERROR_LOG} 1s"
    [ -f "${INFO_LOG}" ]   && APP_ARGS="${APP_ARGS} -t ${ONLY_APP}-[INFO] -iw ${INFO_LOG} 1s"
    [ -f "${ACCESS_LOG}" ] && APP_ARGS="${APP_ARGS} -t ${ONLY_APP}-[HTTP] -iw ${ACCESS_LOG} 1s"

else

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

fi


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
