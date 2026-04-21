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
	Outbox    OutboxConfig
	Payment   PaymentConfig
	Nacos     NacosConfig
	Gateway   GatewayConfig
}

type NacosConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	Host        string `mapstructure:"host"`
	Port        uint64 `mapstructure:"port"`
	Namespace   string `mapstructure:"namespace"`
	Group       string `mapstructure:"group"`
	ServiceName string `mapstructure:"service_name"`
	InstanceIP  string `mapstructure:"instance_ip"`
	DataID      string `mapstructure:"data_id"`
}

type GatewayConfig struct {
	Routes        []RouteConfig       `mapstructure:"routes"`
	Sentinel      SentinelConfig      `mapstructure:"sentinel"`
	RefreshSecMs  int                 `mapstructure:"refresh_ms"`
}

type RouteConfig struct {
	PathPrefix  string `mapstructure:"path_prefix"`
	ServiceName string `mapstructure:"service_name"`
	Resource    string `mapstructure:"resource"`
}

type SentinelConfig struct {
	SeckillQPS              float64 `mapstructure:"seckill_qps"`
	SeckillBreakerErrorRate float64 `mapstructure:"seckill_breaker_error_rate"`
	BreakerMinRequests      uint64  `mapstructure:"breaker_min_requests"`
	BreakerStatIntervalMs   uint32  `mapstructure:"breaker_stat_interval_ms"`
	BreakerRetryTimeoutMs   uint32  `mapstructure:"breaker_retry_timeout_ms"`
	DefaultQPS              float64 `mapstructure:"default_qps"`
}

type SnowflakeConfig struct {
	NodeID int64 `mapstructure:"node_id"`
}

type KafkaConfig struct {
	Brokers        []string              `mapstructure:"brokers"`
	Topic          string                `mapstructure:"topic"`
	ProduceTopic   string                `mapstructure:"produce_topic"`
	ConsumerGroups []ConsumerGroupConfig `mapstructure:"consumer_groups"`
}

type ConsumerGroupConfig struct {
	GroupID string `mapstructure:"group_id"`
	Topic   string `mapstructure:"topic"`
}

type OutboxConfig struct {
	IntervalMs int `mapstructure:"interval_ms"`
}

type PaymentConfig struct {
	SuccessRate float64 `mapstructure:"success_rate"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type DatabaseConfig struct {
	Driver   string          `mapstructure:"driver"`
	Host     string          `mapstructure:"host"`
	Port     int             `mapstructure:"port"`
	User     string          `mapstructure:"user"`
	Password string          `mapstructure:"password"`
	DBName   string          `mapstructure:"dbname"`
	SSLMode  string          `mapstructure:"sslmode"`
	Timezone string          `mapstructure:"timezone"`
	Replicas []ReplicaConfig `mapstructure:"replicas"`
}

type ReplicaConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
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

func (d *DatabaseConfig) ReplicaDSNs() []string {
	dsns := make([]string, 0, len(d.Replicas))
	for _, r := range d.Replicas {
		user := r.User
		if user == "" {
			user = d.User
		}
		pw := r.Password
		if pw == "" {
			pw = d.Password
		}
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", user, pw, r.Host, r.Port, d.DBName, d.SSLMode)
		dsns = append(dsns, dsn)
	}
	return dsns
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
