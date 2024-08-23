#!/bin/bash

set -e
trap "exit" TERM

if [[ -z "${FRONTEND_ADDR}" ]]; then
    echo >&2 "FRONTEND_ADDR not specified"
    exit 1
fi

set -x

# Keep trying until the frontend is reachable
while true; do
    STATUSCODE=$(curl --silent --output /dev/stderr --write-out "%{http_code}" "${FRONTEND_ADDR}")
    if test $STATUSCODE -eq 200; then
        break
    else
        echo "Error: Could not reach frontend - Status code: ${STATUSCODE}"
        sleep 1
    fi
done

# else, run loadgen
locust --host="${FRONTEND_ADDR}" --headless -u "${USERS:-10}" 2>&1
