package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Log       LogConfig       `mapstructure:"log"`
	Aria2     Aria2Config     `mapstructure:"aria2"`
	Alist     AlistConfig     `mapstructure:"alist"`
	Telegram  TelegramConfig  `mapstructure:"telegram"`
	Download  DownloadConfig  `mapstructure:"download"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type LogConfig struct {
	Level     string `mapstructure:"level"`
	Output    string `mapstructure:"output"`
	Format    string `mapstructure:"format"`
	FilePath  string `mapstructure:"file_path"`
	Colorize  bool   `mapstructure:"colorize"`
	AddSource bool   `mapstructure:"add_source"`
}

type Aria2Config struct {
	RpcURL      string `mapstructure:"rpc_url"`
	Token       string `mapstructure:"token"`
	DownloadDir string `mapstructure:"download_dir"`
}

type AlistConfig struct {
	BaseURL     string `mapstructure:"base_url"`
	Token       string `mapstructure:"token"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	DefaultPath string `mapstructure:"default_path"`
	QPS         int    `mapstructure:"qps"` // 每秒请求数限制，默认50
}

type TelegramConfig struct {
	BotToken string        `mapstructure:"bot_token"`
	ChatIDs  []int64       `mapstructure:"chat_ids"`
	Enabled  bool          `mapstructure:"enabled"`
	AdminIDs []int64       `mapstructure:"admin_ids"`
	Webhook  WebhookConfig `mapstructure:"webhook"`
}

type WebhookConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
	Port    string `mapstructure:"port"`
}

type DownloadConfig struct {
	VideoOnly   bool           `mapstructure:"video_only"`
	VideoExts   []string       `mapstructure:"video_extensions"`
	ExcludeExts []string       `mapstructure:"exclude_extensions"`
	MinFileSize int64          `mapstructure:"min_file_size_mb"`
	MaxFileSize int64          `mapstructure:"max_file_size_mb"`
	PathConfig  PathConfig     `mapstructure:"path_config"` // 路径配置
}

// PathConfig 路径配置
type PathConfig struct {
	Templates PathTemplates `mapstructure:"templates"` // 路径模板
}

// PathTemplates 路径模板配置
type PathTemplates struct {
	TV      string `mapstructure:"tv"`      // 电视剧路径模板
	Movie   string `mapstructure:"movie"`   // 电影路径模板
	Variety string `mapstructure:"variety"` // 综艺路径模板
	Default string `mapstructure:"default"` // 默认路径模板
}

type SchedulerConfig struct {
	Enabled bool            `mapstructure:"enabled"`
	Tasks   []ScheduledTask `mapstructure:"tasks"`
}

type ScheduledTask struct {
	Name        string `mapstructure:"name"`         // 任务名称
	Enabled     bool   `mapstructure:"enabled"`      // 是否启用
	Cron        string `mapstructure:"cron"`         // cron表达式，如 "0 2 * * *" 每天凌晨2点
	Path        string `mapstructure:"path"`         // 要下载的目录路径
	HoursAgo    int    `mapstructure:"hours_ago"`    // 下载多少小时前的文件（如24表示昨天）
	VideoOnly   bool   `mapstructure:"video_only"`   // 是否只下载视频
	AutoPreview bool   `mapstructure:"auto_preview"` // 是否自动预览模式
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置默认值
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.output", "console")
	viper.SetDefault("log.format", "text")
	viper.SetDefault("log.file_path", "./logs/app.log")
	viper.SetDefault("log.colorize", true)
	viper.SetDefault("log.add_source", false)
	viper.SetDefault("aria2.rpc_url", "http://localhost:6800/jsonrpc")
	viper.SetDefault("aria2.download_dir", "/downloads")
	viper.SetDefault("alist.base_url", "http://localhost:5244")
	viper.SetDefault("alist.default_path", "/")
	viper.SetDefault("alist.qps", 50)
	viper.SetDefault("telegram.enabled", false)
	viper.SetDefault("telegram.webhook.enabled", false)
	viper.SetDefault("telegram.webhook.port", "8082")

	// 下载配置默认值
	viper.SetDefault("download.video_only", true)
	viper.SetDefault("download.video_extensions", []string{
		"mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "3gp",
		"ts", "m2ts", "mts", "vob", "divx", "xvid", "rmvb", "rm", "asf",
	})
	viper.SetDefault("download.exclude_extensions", []string{
		"txt", "nfo", "srt", "ass", "ssa", "sup", "idx", "sub",
		"jpg", "jpeg", "png", "gif", "bmp", "webp", "tiff",
	})
	viper.SetDefault("download.min_file_size_mb", 50)
	viper.SetDefault("download.max_file_size_mb", 0)

	// 路径模板默认值（留空表示使用智能路径生成）
	viper.SetDefault("download.path_config.templates.tv", "")
	viper.SetDefault("download.path_config.templates.movie", "")
	viper.SetDefault("download.path_config.templates.variety", "")
	viper.SetDefault("download.path_config.templates.default", "")

	// 调度器配置默认值
	viper.SetDefault("scheduler.enabled", false)
	viper.SetDefault("scheduler.tasks", []ScheduledTask{})

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
