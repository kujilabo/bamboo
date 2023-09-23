package request

type RequestProducerConfig struct {
	Type  string                      `yaml:"type" validate:"required"`
	Kafka *KafkaRequestProducerConfig `yaml:"kafka"`
	Redis *RedisRequestProducerConfig `yaml:"redis"`
}

type KafkaRequestProducerConfig struct {
	Addr  string `yaml:"addr" validate:"required"`
	Topic string `yaml:"topic" validate:"required"`
}

type RedisRequestProducerConfig struct {
	Addrs    []string `yaml:"addrs" validate:"required"`
	Password string   `yaml:"password"`
	Channel  string   `yaml:"channel" validate:"required"`
}
