package statsmt

import "strconv"

// Kafka tracks the health of a consumer
type Kafka map[int32]*KafkaPartition

func NewKafka(prefix string, partitions []int32) Kafka {
	k := make(map[int32]*KafkaPartition)
	for _, part := range partitions {
		k[part] = NewKafkaPartition(prefix + ".partition." + strconv.Itoa(int(part)))
	}
	return k
}

// KafkaPartition tracks the health of a partition consumer
type KafkaPartition struct {
	Offset  Gauge64
	LogSize Gauge64
	Lag     Gauge64
}

func NewKafkaPartition(prefix string) *KafkaPartition {
	k := KafkaPartition{}
	Register.GetOrAdd(prefix+".offset", &k.Offset)
	Register.GetOrAdd(prefix+".log_size", &k.LogSize)
	Register.GetOrAdd(prefix+".lag", &k.Lag)
	return &k
}
