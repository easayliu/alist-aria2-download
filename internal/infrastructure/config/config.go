package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Aria2  Aria2Config  `mapstructure:"aria2"`
	Alist  AlistConfig  `mapstructure:"alist"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type Aria2Config struct {
	RpcURL    string `mapstructure:"rpc_url"`
	Token     string `mapstructure:"token"`
	DownloadDir string `mapstructure:"download_dir"`
}

type AlistConfig struct {
	BaseURL     string `mapstructure:"base_url"`
	Token       string `mapstructure:"token"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	DefaultPath string `mapstructure:"default_path"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置默认值
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("aria2.rpc_url", "http://localhost:6800/jsonrpc")
	viper.SetDefault("aria2.download_dir", "/downloads")
	viper.SetDefault("alist.base_url", "http://localhost:5244")
	viper.SetDefault("alist.default_path", "/")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}