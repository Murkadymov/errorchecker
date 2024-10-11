package config

import (
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Host        string   `env:"HOST"`
	Cluster     []string `env:"-"`
	Cookie      string   `env:"COOKIE_SECRET"`
	Interval    int      `env:"INTERVAL"`
	BandURL     string   `env:"BAND_URL"`
	BandWebhook string   `env:"BAND_WEBHOOK_ENDPOINT"`
}

const envFileName = ".env"

func MustParse() *Config {
	if err := godotenv.Load(envFileName); err != nil {
		panic(err)
	}

	var cfg Config
	if err := env.Parse(cfg); err != nil {
		panic(err)
	}

	cfg.Cluster = strings.Split(os.Getenv("CLUSTER"), ",")

	return &cfg
}
