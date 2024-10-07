package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
	"log/slog"
	"os"
	"path/filepath"
)

type Config struct {
	WB struct {
		Host   []string `yaml:"hostURL"`
		Cookie string   `env:"COOKIE_SECRET"`
	} `yaml:"wb"`
	Band struct {
		BandURL     string `yaml:"bandURL"`
		BandWebhook string `env:"BAND_WEBHOOK_ENDPOINT"`
	} `yaml:"band"`
}

func NewConfig() *Config {
	return &Config{}
}

func FindEnv() (string, error) {

	dir, _ := os.Getwd()

	for {
		envPath := filepath.Join(dir, ".env")
		_, err := os.Stat(envPath)
		fmt.Println("current dir\n", envPath)
		if err == nil {
			fmt.Println("env file found")
			return envPath, nil
		}

		parent := filepath.Dir(dir)

		if parent == dir {
			fmt.Println("no upper directory")
			break
		}

		dir = parent
	}
	return "", nil

}

func MustLoad(log *slog.Logger) *Config {
	cfg := NewConfig()

	yamlCfg, err := os.Open("E:\\Projects\\errorchecker\\config\\config.yaml")
	if err != nil {
		fmt.Println("error openning yaml file", err)
	}

	err = yaml.NewDecoder(yamlCfg).Decode(cfg)
	if err != nil {
		fmt.Println("error decoding yaml file to struct")
	}

	envPath, err := FindEnv()
	if err != nil {
		log.Error("error finding env file", "err", err.Error())
		return nil
	}

	err = godotenv.Load(envPath)
	if err != nil {
		log.Error("error loading env file", "error", err.Error())
		return nil
	}

	return cfg
}
