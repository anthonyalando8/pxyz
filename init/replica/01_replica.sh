#!/bin/bash
set -e

echo "Configuring Postgres replica..."

rm -rf "$PGDATA"/*
chmod 700 "$PGDATA"

PGPASSWORD=2000 pg_basebackup -h postgres-master -D "$PGDATA" -U replicator -Fp -Xs -P -R

# Enable standby mode
echo "hot_standby = on" >> "$PGDATA/postgresql.conf"