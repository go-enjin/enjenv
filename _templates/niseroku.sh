#!/bin/bash
ENJENV_BIN=$(which enjenv)
if [ -z "${ENJENV_BIN}" -o ! -x "${ENJENV_BIN}" ]
then
    echo "enjenv binary not found" 1>&2
    exit 1
fi
exec "${ENJENV_BIN}" niseroku "$@"
