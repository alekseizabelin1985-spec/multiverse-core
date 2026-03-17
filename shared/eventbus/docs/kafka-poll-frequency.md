# Kafka Poll Frequency Configuration

## Environment Variable

- `KAFKA_POLL_FREQUENCY_MS`: Controls how frequently the Kafka consumer checks for new messages (in milliseconds)

## Default Value

- Default: `1000` (1 second)

## Usage

Set the environment variable when running any service that subscribes to Kafka topics:

```bash
KAFKA_POLL_FREQUENCY_MS=2000 KAFKA_BROKERS=localhost:9092 go run cmd/entity-manager/main.go
```

This would set the polling frequency to 2 seconds (2000 milliseconds).

## Effect

The `KAFKA_POLL_FREQUENCY_MS` value sets the `MaxWait` parameter in the Kafka reader configuration, which determines how long the reader waits for messages before returning. Lower values result in more frequent checks but potentially more network requests, while higher values reduce network traffic but may increase message delivery latency.