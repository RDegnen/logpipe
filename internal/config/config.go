package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	KafkaBrokers        string
	KafkaProcessorGroup string
	KafkaSinkGroup      string
	RawLogsTopic        string
	ProcessedLogsTopic  string
	DLQTopic            string
	SeenEventsTopic     string
	Port                string
	CommitEvery         int
	Host                string
	DB                  string
	DBUser              string
	DBPassword          string
	DBPort              string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	commitEvery, _ := strconv.Atoi(os.Getenv("COMMIT_EVERY"))

	return &Config{
		KafkaBrokers:        os.Getenv("KAFKA_BROKERS"),
		KafkaProcessorGroup: os.Getenv("KAFKA_PROCESSOR_GROUP"),
		KafkaSinkGroup:      os.Getenv("KAFKA_SINK_GROUP"),
		RawLogsTopic:        os.Getenv("RAW_LOGS_TOPIC"),
		ProcessedLogsTopic:  os.Getenv("PROCESSED_LOGS_TOPIC"),
		DLQTopic:            os.Getenv("DLQ_TOPIC"),
		SeenEventsTopic:     os.Getenv("SEEN_EVENTS_TOPIC"),
		Port:                os.Getenv("PORT"),
		CommitEvery:         commitEvery,
		Host:                os.Getenv("PG_HOST"),
		DB:                  os.Getenv("PG_DATABASE"),
		DBUser:              os.Getenv("PG_USER"),
		DBPassword:          os.Getenv("PG_PASSWORD"),
		DBPort:              os.Getenv("PG_PORT"),
	}
}
