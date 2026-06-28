package main

import (
	"os"

	"github.com/json-iterator/go"
)

type Config struct {
	KeyWords []string `json:"keywords"`
}

func GetConfig(filepath string) (*Config, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	if err = jsoniter.NewDecoder(file).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}
