#!/bin/bash

stop_old_slug () {
    TGT="$1"
    TGT_NAME=$(basename "${TGT}" | perl -pe 's!\.(pid|port|settings)$!!')
    TGT_PATH=$(dirname "${TGT}")
    RUN_PATH="${TGT_PATH}/${TGT_NAME}"
    SET_FILE="${TGT_PATH}/${TGT_NAME}.settings"
    PID_FILE="${TGT_PATH}/${TGT_NAME}.pid"
    PORT_FILE="${TGT_PATH}/${TGT_NAME}.port"

    echo "# checking for running pid"
    if [ -f "${PID_FILE}" ]
    then
        TGT_PID=$(cat "${PID_FILE}")
        FOUND_PGRP=$(ps -o pgrp= -p "${TGT_PID}" | perl -pe 's!^\s+!!')
        if [ -n "${FOUND_PGRP}" ]
        then
            echo "# killing: ${TGT_PID} (-${FOUND_PGRP})"
            kill -TERM -- -${FOUND_PGRP}
            if [ $? -ne 0 ]
            then
                echo "# error killing pid"
                return
            fi
        fi
        echo "# removing: ${PID_FILE}"
        rm -f "${PID_FILE}"
    fi

    echo "# checking [run.d]: ${RUN_PATH}"
    if [ -d "${RUN_PATH}" ]
    then
        echo "# removing: ${RUN_PATH}"
        rm -rf "${RUN_PATH}"
    fi

    echo "# checking [port]: ${PORT_FILE}"
    if [ -f "${PORT_FILE}" ]
    then
        echo "# removing: ${PORT_FILE}"
        rm -f "${PORT_FILE}"
    fi

    echo "# checking [settings]: ${SET_FILE}"
    if [ -f "${SET_FILE}" ]
    then
        echo "# removing: ${SET_FILE}"
        rm -f "${SET_FILE}"
    fi
}

if [ $# -eq 0 ]
then
    echo "usage: $(basename $) <file.pid> [file.pid...]"
    exit 1
fi

while [ $# -gt 0 ]
do
    [ -f "$1" ] && stop_old_slug "${1}"
    shift
done
