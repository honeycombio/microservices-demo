#!/bin/bash

trap "exit" TERM

if [[ -z "${FRONTEND_ADDR}" ]]; then
    echo >&2 "FRONTEND_ADDR not specified"
    exit 1
fi

# set -x

# if one request to the frontend fails, then exit
while true; do
    STATUSCODE=$(curl --silent --output /dev/stderr --write-out "%{http_code}" "${FRONTEND_ADDR}")
    if test $STATUSCODE -ne 200; then
        echo "Error: Could not reach frontend - Status code: ${STATUSCODE}"
        sleep 1
    else
        break
    fi
done

# else, run loadgen
locust --host="${FRONTEND_ADDR}" --headless -u "${USERS:-10}" 2>&1
