package result

type RedisResultSubscriberConfig struct {
	Addrs    []string `yaml:"addrs" validate:"required"`
	Password string   `yaml:"password"`
}

type ResultSubscriberConfig struct {
	Type  string                       `yaml:"type" validate:"required"`
	Redis *RedisResultSubscriberConfig `yaml:"redis"`
}
