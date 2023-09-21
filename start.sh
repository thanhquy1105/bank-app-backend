#!/bin/sh

set -e

echo "run db migration"

# awk -F "=" -v var=$DB_SOURCE '{if (match($0,/DB_SOURCE/)) {print $1"="var} else {print $0}}' app.env > app.env1
if [[ -z "${DB_SOURCE}" ]]; then
    echo "none"
    source /app/app.env
else
    echo "${DB_SOURCE}"
fi
export DB_SOURCE="${DB_SOURCE}"
/app/soda migrate -p /app/migrations -c /app/database.yml -e production

echo "start the app"
exec "$@"