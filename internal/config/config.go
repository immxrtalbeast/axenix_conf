package config

import (
	"flag"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env    string       `yaml:"env" env-default:"local"`
	HTTP   HTTPConfig   `yaml:"http"`
	WebRTC WebRTCConfig `yaml:"webrtc"`
}

type HTTPConfig struct {
	Address string `yaml:"address" env-default:""`
}

type WebRTCConfig struct {
	STUNServers []string `yaml:"stun_servers" env-default:""`
}

func MustLoad() *Config {
	configPath := fetchConfigPath()
	if configPath == "" {
		panic("config path is empty")
	}

	return MustLoadPath(configPath)
}

func MustLoadPath(configPath string) *Config {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("cannot read config: " + err.Error())
	}

	cfg.setDefaults()

	return &cfg
}

func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	if res == "" {
		res = "config/local.yaml"
	}

	return res
}

func (c *Config) setDefaults() {
	if c.HTTP.Address == "" {
		c.HTTP.Address = ":8080"
	}
	if len(c.WebRTC.STUNServers) == 0 {
		c.WebRTC.STUNServers = []string{"stun:stun.l.google.com:19302"}
	}
}
