#!/bin/sh

set -e

echo "run db migration"
source /app/app.env
export DB_SOURCE="$DB_SOURCE"
/app/soda migrate -p /app/migrations -c /app/database.yml -e production

echo "start the app"
exec "$@"