package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	KafkaBrokers string
	KafkaGroup string
	RawLogsTopic string
	ProcessedLogsTopic string
	DLQTopic string
	Port string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil{
		log.Fatal("Error loading .env file")
	}

	return &Config{
		KafkaBrokers: os.Getenv("KAFKA_BROKERS"),
		KafkaGroup: os.Getenv("KAFKA_GROUP"),
		RawLogsTopic: os.Getenv("RAW_LOGS_TOPIC"),
		ProcessedLogsTopic: os.Getenv("PROCESSED_LOGS_TOPIC"),
		DLQTopic: os.Getenv("DLQ_TOPIC"),
		Port: os.Getenv("PORT"),
	}
}