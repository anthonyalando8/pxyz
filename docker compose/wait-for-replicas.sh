#!/bin/bash
set -e

# Wait for master first
until PGPASSWORD=Kenya_2025! psql -h postgres-master -U postgres -d postgres -c '\q'; do
  echo "Waiting for postgres-master to be ready..."
  sleep 2
done

# Wait for all replicas
for REPLICA in postgres-replica1 postgres-replica2 postgres-replica3; do
  until PGPASSWORD=Kenya_2025! psql -h $REPLICA -U postgres -d postgres -c '\q'; do
    echo "Waiting for $REPLICA to be ready..."
    sleep 2
  done
done

echo "All replicas are ready."
