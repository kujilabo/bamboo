package config

import (
	"embed"
	"os"

	_ "embed"

	"gopkg.in/yaml.v2"

	"github.com/kujilabo/bamboo/bamboo-lib/worker"
	libconfig "github.com/kujilabo/bamboo/lib/config"
	libD "github.com/kujilabo/bamboo/lib/domain"
	liberrors "github.com/kujilabo/bamboo/lib/errors"
)

type AppConfig struct {
	Name        string `yaml:"name" validate:"required"`
	MetricsPort int    `yaml:"metricsPort" validate:"required"`
}

type ShutdownConfig struct {
	TimeSec1 int `yaml:"timeSec1" validate:"gte=1"`
	TimeSec2 int `yaml:"timeSec2" validate:"gte=1"`
}

type DebugConfig struct {
	GinMode bool `yaml:"ginMode"`
	Wait    bool `yaml:"wait"`
}

type Config struct {
	App      *AppConfig             `yaml:"app" validate:"required"`
	Worker   *worker.WorkerConfig   `yaml:"worker" validate:"required"`
	Trace    *libconfig.TraceConfig `yaml:"trace" validate:"required"`
	Shutdown *ShutdownConfig        `yaml:"shutdown" validate:"required"`
	Log      *libconfig.LogConfig   `yaml:"log" validate:"required"`
	Debug    *DebugConfig           `yaml:"debug"`
}

// //go:embed run.yml

//go:embed debug.yml
var config embed.FS

func LoadConfig(mode string) (*Config, error) {
	filename := mode + ".yml"
	confContent, err := config.ReadFile(filename)
	if err != nil {
		return nil, liberrors.Errorf("config.ReadFile. filename: %s, err: %w", filename, err)
	}

	confContent = []byte(os.ExpandEnv(string(confContent)))
	conf := &Config{}
	if err := yaml.Unmarshal(confContent, conf); err != nil {
		return nil, liberrors.Errorf("yaml.Unmarshal. filename: %s, err: %w", filename, err)
	}

	if err := libD.Validator.Struct(conf); err != nil {
		return nil, liberrors.Errorf("Validator.Structl. filename: %s, err: %w", filename, err)
	}

	return conf, nil
}
