package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
}

func LoadR2Config() (R2Config, error) {
	_ = godotenv.Load()

	cfg := R2Config{
		AccountID:       os.Getenv("R2_ACCOUNT_ID"),
		AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		BucketName:      os.Getenv("R2_BUCKET_NAME"),
	}

	if cfg.AccountID == "" {
		return cfg, fmt.Errorf("R2_ACCOUNT_ID is required")
	}
	if cfg.AccessKeyID == "" {
		return cfg, fmt.Errorf("R2_ACCESS_KEY_ID is required")
	}
	if cfg.SecretAccessKey == "" {
		return cfg, fmt.Errorf("R2_SECRET_ACCESS_KEY is required")
	}
	if cfg.BucketName == "" {
		return cfg, fmt.Errorf("R2_BUCKET_NAME is required")
	}

	return cfg, nil
}
