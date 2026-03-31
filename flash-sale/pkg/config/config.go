package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Redis     RedisConfig
	Snowflake SnowflakeConfig
	Kafka     KafkaConfig
}

type SnowflakeConfig struct {
	NodeID int64 `mapstructure:"node_id"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	Timezone string `mapstructure:"timezone"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode)
}

func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)

	viper.SetEnvPrefix("FLASH_SALE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("Read config: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("Unmarshal config: %w", err)
	}

	return &cfg, nil
}
