package worker

type WorkerConfig struct {
	Consumer   *ConsumerConfig  `yaml:"consumer"`
	Publisher  *PublisherConfig `yaml:"publisher"`
	NumWorkers int              `yaml:"numWorkers"`
}

type ConsumerConfig struct {
	Type  string               `yaml:"type" validate:"required"`
	Kafka *KafkaConsumerConfig `yaml:"kafka"`
	Redis *RedisConsumerConfig `yaml:"redis"`
}

type KafkaConsumerConfig struct {
	Brokers []string `yaml:"brokers" validate:"required"`
	GroupID string   `yaml:"groupId" validate:"required"`
	Topic   string   `yaml:"topic" validate:"required"`
}

type RedisConsumerConfig struct {
	Addrs    []string `yaml:"addrs" validate:"required"`
	Password string   `yaml:"password"`
	Channel  string   `yaml:"channel" validate:"required"`
}

type PublisherConfig struct {
	Type  string                `yaml:"type" validate:"required"`
	Redis *RedisPublisherConfig `yaml:"redis"`
}

type RedisPublisherConfig struct {
	Addrs    []string `yaml:"addrs" validate:"required"`
	Password string   `yaml:"password"`
}
