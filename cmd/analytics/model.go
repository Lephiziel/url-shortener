package analytics

import "time"

type Visit struct {
	Code           string
	OccurredAt     time.Time
	KafkaTopic     string
	KafkaPartition int32
	KafkaOffset    int64
}
