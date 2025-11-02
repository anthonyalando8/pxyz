#!/bin/bash
# scripts/setup-kafka-topics.sh

KAFKA_BROKER="kafka-service:9092"

echo "Creating Kafka topics for user registration..."

# Create main registration topic
kafka-topics.sh --create \
  --bootstrap-server $KAFKA_BROKER \
  --topic user.registration \
  --partitions 6 \
  --replication-factor 3 \
  --config retention.ms=604800000 \
  --config compression.type=snappy \
  --config min.insync.replicas=2

# Create DLQ topic
kafka-topics.sh --create \
  --bootstrap-server $KAFKA_BROKER \
  --topic user.registration.dlq \
  --partitions 3 \
  --replication-factor 3 \
  --config retention.ms=2592000000 \
  --config compression.type=snappy \
  --config min.insync.replicas=2

echo "Topics created successfully!"

# List topics to verify
kafka-topics.sh --list --bootstrap-server $KAFKA_BROKER

# Describe topics
kafka-topics.sh --describe \
  --bootstrap-server $KAFKA_BROKER \
  --topic user.registration

kafka-topics.sh --describe \
  --bootstrap-server $KAFKA_BROKER \
  --topic user.registration.dlq