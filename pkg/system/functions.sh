#!/bin/bash

# Copyright (c) 2023  The Go-Enjin Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

function _enjenv_has_path () {
    ARG="$1"; while IFS=: read -d: -r present
    do
        [ "${ARG}" == "${present}" -a -d "${present}" ] && return 0
    done <<< ${PATH:+"${PATH}:"}
    return 1
}

function _enjenv_add_path () {
    while [ $# -gt 0 ]
    do
        ARG="$1"
        shift
        if [ -d "${ARG}" ]
        then
            if _enjenv_has_path "${ARG}"
            then
                echo "# already present: ${ARG}"
                continue
            fi
            echo "# exporting: ${ARG}"
            export PATH="${ARG}:${PATH}"
        else
            echo "# not a directory, nothing to do: ${ARG}"
        fi
    done
}

function _enjenv_rem_path () {
    while [ $# -gt 0 ]
    do
        ARG="$1"
        shift
        if ! _enjenv_has_path "${ARG}"
        then
            echo "# not present, nothing to do: ${ARG}"
            continue
        fi
        echo "# removing: ${ARG}"
        export PATH=$(echo $PATH | perl -pe "@parts=split(m/:/,\$_);@pruned=();foreach \$part (@parts) { push(@pruned,\$part) if (\$part ne '${ARG}'); }; \$_=join(':',@pruned);")
    done
}

function _enjenv_unset_functions() {
    unset _enjenv_add_path
    unset _enjenv_rem_path
    unset _enjenv_has_path
    unset _enjenv_unset_functions
}