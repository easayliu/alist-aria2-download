package config

import (
	"fmt"

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
	TMDB      TMDBConfig      `mapstructure:"tmdb"`
	LLM       LLMConfig       `mapstructure:"llm"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
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
	VideoOnly   bool       `mapstructure:"video_only"`
	VideoExts   []string   `mapstructure:"video_extensions"`
	ExcludeExts []string   `mapstructure:"exclude_extensions"`
	MinFileSize int64      `mapstructure:"min_file_size_mb"`
	MaxFileSize int64      `mapstructure:"max_file_size_mb"`
	PathConfig  PathConfig `mapstructure:"path_config"` // 路径配置
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

type TMDBConfig struct {
	APIKey             string   `mapstructure:"api_key"`
	Language           string   `mapstructure:"language"`
	QPS                int      `mapstructure:"qps"`
	BatchRenameLimit   int      `mapstructure:"batch_rename_limit"`
	QualityDirPatterns []string `mapstructure:"quality_dir_patterns"`
}

// LLMConfig LLM配置
type LLMConfig struct {
	Enabled   bool            `mapstructure:"enabled"`   // 是否启用LLM功能
	Provider  string          `mapstructure:"provider"`  // 提供商: openai, anthropic, ollama, custom
	OpenAI    OpenAIConfig    `mapstructure:"openai"`    // OpenAI配置
	Anthropic AnthropicConfig `mapstructure:"anthropic"` // Anthropic配置(预留)
	Ollama    OllamaConfig    `mapstructure:"ollama"`    // Ollama配置(预留)
	Features  LLMFeatures     `mapstructure:"features"`  // 功能开关
	Batch     LLMBatchConfig  `mapstructure:"batch"`     // 批处理配置
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	APIKey      string  `mapstructure:"api_key"`     // API密钥
	BaseURL     string  `mapstructure:"base_url"`    // API基础URL,支持第三方API
	Model       string  `mapstructure:"model"`       // 模型名称
	Temperature float32 `mapstructure:"temperature"` // 温度参数
	MaxTokens   int     `mapstructure:"max_tokens"`  // 最大Token数
	Timeout     int     `mapstructure:"timeout"`     // 超时时间(秒)
	QPS         int     `mapstructure:"qps"`         // 每秒请求数限制
}

// AnthropicConfig Anthropic配置(预留)
type AnthropicConfig struct {
	APIKey string `mapstructure:"api_key"` // API密钥
	Model  string `mapstructure:"model"`   // 模型名称
}

// OllamaConfig Ollama配置(预留)
type OllamaConfig struct {
	BaseURL string `mapstructure:"base_url"` // 服务地址
	Model   string `mapstructure:"model"`    // 模型名称
}

// LLMFeatures LLM功能开关
type LLMFeatures struct {
	FileNaming      bool `mapstructure:"file_naming"`      // 文件命名
	ContentAnalysis bool `mapstructure:"content_analysis"` // 内容分析
	AutoTagging     bool `mapstructure:"auto_tagging"`     // 自动标签
}

// LLMBatchConfig LLM批处理配置
type LLMBatchConfig struct {
	BatchSize            int  `mapstructure:"batch_size"`             // 每批文件数量，默认8
	TokenLimit           int  `mapstructure:"token_limit"`            // 单批Token限制，默认4000
	MaxConcurrentBatches int  `mapstructure:"max_concurrent_batches"` // 最大并发批次数，默认3
	BaseTokens           int  `mapstructure:"base_tokens"`            // 基础Prompt Token估算，默认300
	EnableSeasonGrouping bool `mapstructure:"enable_season_grouping"` // 是否启用按季度分组，默认true
}

// Validate 验证LLM配置
func (cfg *LLMConfig) Validate() error {
	if !cfg.Enabled {
		return nil // 未启用不需要验证
	}

	switch cfg.Provider {
	case "openai":
		if cfg.OpenAI.APIKey == "" {
			return fmt.Errorf("OpenAI API Key未配置")
		}
		if cfg.OpenAI.BaseURL == "" {
			return fmt.Errorf("OpenAI BaseURL未配置")
		}
		if cfg.OpenAI.Model == "" {
			return fmt.Errorf("OpenAI Model未配置")
		}
	case "anthropic":
		if cfg.Anthropic.APIKey == "" {
			return fmt.Errorf("Anthropic API Key未配置")
		}
		return fmt.Errorf("Anthropic provider尚未实现")
	case "ollama":
		if cfg.Ollama.BaseURL == "" {
			return fmt.Errorf("Ollama BaseURL未配置")
		}
		return fmt.Errorf("Ollama provider尚未实现")
	default:
		return fmt.Errorf("不支持的Provider: %s", cfg.Provider)
	}

	return nil
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置默认值
	viper.SetDefault("server.host", "127.0.0.1")
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

	// TMDB配置默认值
	viper.SetDefault("tmdb.language", "zh-CN")
	viper.SetDefault("tmdb.qps", 40)
	viper.SetDefault("tmdb.batch_rename_limit", 20)
	viper.SetDefault("tmdb.quality_dir_patterns", []string{
		`(?i)\d{3,4}[pP]`,
		`(?i)\d+K`,
		`(?i)\d+FPS`,
		`(?i)BluRay`,
		`(?i)WEB-?DL`,
		`(?i)WEBRip`,
		`(?i)HDRip`,
		`(?i)BDRip`,
		`(?i)x264`,
		`(?i)x265`,
		`(?i)H\.?264`,
		`(?i)H\.?265`,
		`(?i)HEVC`,
		`(?i)HDR`,
		`(?i)DoVi`,
		`(?i)DTS`,
		`(?i)AAC`,
		`(?i)AC3`,
		`(?i)MAX\+?`,
		`(?i)IMAX`,
		`(?i)Atmos`,
	})

	// LLM配置默认值
	viper.SetDefault("llm.enabled", false)
	viper.SetDefault("llm.provider", "openai")
	viper.SetDefault("llm.openai.base_url", "https://api.openai.com/v1")
	viper.SetDefault("llm.openai.model", "gpt-3.5-turbo")
	viper.SetDefault("llm.openai.temperature", 0.3)
	viper.SetDefault("llm.openai.max_tokens", 1000)
	viper.SetDefault("llm.openai.timeout", 60)
	viper.SetDefault("llm.openai.qps", 10)
	viper.SetDefault("llm.anthropic.model", "claude-3-sonnet-20240229")
	viper.SetDefault("llm.ollama.base_url", "http://localhost:11434")
	viper.SetDefault("llm.ollama.model", "llama2")
	viper.SetDefault("llm.features.file_naming", true)
	viper.SetDefault("llm.features.content_analysis", false)
	viper.SetDefault("llm.features.auto_tagging", false)

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
