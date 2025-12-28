package config

import (
	"flag"
	"log"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"

	"github.com/TheLuckymadman/gophermart/internal/repository"
)

type Config struct {
	RunAddress           string                `env:"RUN_ADDRESS"`
	DatabaseURI          string                `env:"DATABASE_URI"`
	AccrualSystemAddress string                `env:"ACCRUAL_SYSTEM_ADDRESS"`
	DBInitMode           repository.DBInitMode `env:"DB_INIT_MODE"`
	Key                  []byte                `env:"KEY"`
	TokenExpTime         int                   `env:"TOKEN_EXP_TIME"`
	RateLimit            int                   `env:"RATE_LIMIT"`
	DBReqFrequency       int                   `env:"DB_REQ_FREQUENCY"`
}

func Load() *Config {
	cfg := Config{
		RunAddress:           "localhost:8080",
		DBInitMode:           repository.IntManaged,
		AccrualSystemAddress: "localhost:8081",
		Key:                  []byte("top_super_secret"),
		TokenExpTime:         15,
		RateLimit:            3,
		DBReqFrequency:       15,
	}
	err := godotenv.Load()
	if err != nil {
		log.Printf("%v", err)
	}
	err = env.Parse(&cfg)
	if err != nil {
		log.Printf("cannot parse environment varibales: %v", err)
	}

	dbInitMode, err := cfg.DBInitMode.MarshalText()
	if err != nil {
		log.Printf("%v", err)
	}
	key := string(cfg.Key)
	flag.StringVar(&cfg.RunAddress, "a", cfg.RunAddress, "server address and port")
	flag.StringVar(&cfg.DatabaseURI, "d", cfg.DatabaseURI, "database URI ")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", cfg.AccrualSystemAddress, "address of the accrual system")
	flag.StringVar(&dbInitMode, "m", dbInitMode, "database init mode, use external, internal, reset")
	flag.StringVar(&key, "k", key, "key JWT secret")
	cfg.Key = []byte(key)
	flag.IntVar(&cfg.TokenExpTime, "t", cfg.TokenExpTime, "key JWT secret")
	flag.IntVar(&cfg.RateLimit, "l", cfg.RateLimit, "worker count, i.e count of simultaneouse connections to the accrual system")
	flag.IntVar(&cfg.DBReqFrequency, "rf", cfg.DBReqFrequency, "a frequency of fetching orders from the db by the worke")
	flag.Parse()

	switch dbInitMode {
	case "internal":
		cfg.DBInitMode = repository.IntManaged
	case "external":
		cfg.DBInitMode = repository.ExtManaged
	case "reset":
		cfg.DBInitMode = repository.IntManagedForce
	default:
		cfg.DBInitMode = repository.IntManaged
	}

	return &cfg
}
