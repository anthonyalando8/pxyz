#!/bin/bash
set -e

echo "Configuring Postgres master..."

# Enable replication
echo "wal_level = replica" >> "$PGDATA/postgresql.conf"
echo "max_wal_senders = 10" >> "$PGDATA/postgresql.conf"
echo "wal_keep_size = 64MB" >> "$PGDATA/postgresql.conf"
echo "hot_standby = on" >> "$PGDATA/postgresql.conf"

# Allow replication connections
echo "host replication replicator all 0.0.0.0/0 md5" >> "$PGDATA/pg_hba.conf"

# Create replication user
psql -U postgres -c "CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD 'replica_pass';"