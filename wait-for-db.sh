#!/bin/bash

# Wait for PostgreSQL database to be ready
set -e

host="$1"
port="$2"
shift 2
cmd="$@"

echo "Waiting for PostgreSQL at $host:$port..."

until nc -z "$host" "$port"; do
  echo "PostgreSQL is unavailable - sleeping"
  sleep 1
done

echo "PostgreSQL is up - executing command"
exec $cmd
