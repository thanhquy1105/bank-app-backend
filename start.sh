#!/bin/sh

set -e

echo "run db migration"
/app/soda migrate -p ./migrations -c ./database.yml -e production

echo "start the app"
exec "$@"