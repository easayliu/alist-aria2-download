package telegram

import (
	"context"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/callbacks"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/commands"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/application/services"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/telegram"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramController 重构后的 Telegram 主控制器
// 负责路由分发和依赖管理
type TelegramController struct {
	// 核心依赖 - 使用contracts接口实现API First架构
	telegramClient      *telegram.Client
	notificationService *services.NotificationService
	fileService         contracts.FileService      // 使用契约接口
	downloadService     contracts.DownloadService  // 使用契约接口
	schedulerService    *services.SchedulerService
	container           *services.ServiceContainer  // 服务容器
	config              *config.Config

	// 状态管理 - 与旧版本兼容
	lastUpdateID int
	ctx          context.Context
	cancel       context.CancelFunc

	// 重构后的模块化组件
	messageUtils        *utils.MessageUtils
	basicCommands       *commands.BasicCommands
	downloadCommands    types.DownloadCommandHandler
	taskCommands        *commands.TaskCommands
	menuCallbacks       *callbacks.MenuCallbacks
	
	// 各个功能处理器
	messageHandler  *MessageHandler
	callbackHandler *CallbackHandler
	downloadHandler *DownloadHandler
	fileHandler     *FileHandler
	taskHandler     *TaskHandler
	statusHandler   *StatusHandler
	common          *Common
}


// NewTelegramController 创建新的 Telegram 控制器
// 使用API First架构，通过ServiceContainer获取契约接口
func NewTelegramController(cfg *config.Config, notificationService *services.NotificationService, fileService *services.FileService, schedulerService *services.SchedulerService) *TelegramController {
	var telegramClient *telegram.Client
	if cfg.Telegram.Enabled {
		telegramClient = telegram.NewClient(&cfg.Telegram)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 创建服务容器
	container, err := services.NewServiceContainer(cfg)
	if err != nil {
		logger.Error("Failed to create service container:", err)
		panic("Service container initialization failed")
	}

	// 创建主控制器实例
	controller := &TelegramController{
		telegramClient:      telegramClient,
		notificationService: notificationService,
		schedulerService:    schedulerService,
		container:           container,
		config:              cfg,
		ctx:                 ctx,
		cancel:              cancel,
	}

	// 初始化模块化组件
	controller.initializeModules()

	return controller
}

// initializeModules 初始化所有模块化组件
func (c *TelegramController) initializeModules() {
	// 创建消息工具
	c.messageUtils = utils.NewMessageUtils(c.telegramClient)

	// 从服务容器获取契约接口，实现API First架构
	c.fileService = c.container.GetFileService()
	c.downloadService = c.container.GetDownloadService()

	// 使用契约接口初始化基础命令模块
	c.basicCommands = commands.NewBasicCommands(c.downloadService, c.fileService, c.config, c.messageUtils)
	c.downloadCommands = commands.NewDownloadCommands(c.container, c.messageUtils)
	c.taskCommands = commands.NewTaskCommands(c.schedulerService, c.config, c.messageUtils)

	// 创建回调处理器
	c.menuCallbacks = callbacks.NewMenuCallbacks(c.downloadService, c.config, c.messageUtils)

	// 初始化各个功能处理器
	c.messageHandler = NewMessageHandler(c)
	c.callbackHandler = NewCallbackHandler(c)
	c.downloadHandler = NewDownloadHandler(c)
	c.fileHandler = NewFileHandler(c)
	c.taskHandler = NewTaskHandler(c)
	c.statusHandler = NewStatusHandler(c)
	c.common = NewCommon(c)
}

// ================================
// 公共接口实现 - 与旧版本完全兼容
// ================================

// Webhook 处理 Webhook 请求（与旧版本完全兼容）
func (c *TelegramController) Webhook(ctx *gin.Context) {
	if !c.config.Telegram.Enabled {
		ctx.JSON(200, gin.H{"error": "Telegram integration disabled"})
		return
	}

	var update tgbotapi.Update
	if err := ctx.ShouldBindJSON(&update); err != nil {
		logger.Error("Failed to parse telegram update:", err)
		ctx.JSON(400, gin.H{"error": "Invalid update format"})
		return
	}

	if update.Message != nil {
		c.messageHandler.HandleMessage(&update)
	} else if update.CallbackQuery != nil {
		c.callbackHandler.HandleCallbackQuery(&update)
	}

	ctx.JSON(200, gin.H{"ok": true})
}

// StartPolling 开始轮询（与旧版本完全兼容）
func (c *TelegramController) StartPolling() {
	if !c.config.Telegram.Enabled || c.telegramClient == nil {
		logger.Info("Telegram polling disabled")
		return
	}

	logger.Info("Starting Telegram polling...")

	go func() {
		for {
			select {
			case <-c.ctx.Done():
				logger.Info("Telegram polling stopped")
				return
			default:
				c.pollUpdates()
			}
		}
	}()
}

// StopPolling 停止轮询（与旧版本完全兼容）
func (c *TelegramController) StopPolling() {
	if c.cancel != nil {
		c.cancel()
	}
}

// pollUpdates 轮询更新
func (c *TelegramController) pollUpdates() {
	updates, err := c.telegramClient.GetUpdates(int64(c.lastUpdateID+1), 30)
	if err != nil {
		logger.Error("Failed to get telegram updates:", err)
		time.Sleep(5 * time.Second)
		return
	}

	for _, update := range updates {
		if update.UpdateID > c.lastUpdateID {
			c.lastUpdateID = update.UpdateID
		}

		if update.Message != nil {
			c.messageHandler.HandleMessage(&update)
		} else if update.CallbackQuery != nil {
			c.callbackHandler.HandleCallbackQuery(&update)
		}
	}
}

// ================================
// 兼容性接口 - 为了保持向后兼容
// ================================

// 为了保持与旧代码的兼容性，提供一些委托方法
func (c *TelegramController) FormatFileSize(size int64) string {
	return c.common.FormatFileSize(size)
}

// 获取器方法，供其他模块使用
func (c *TelegramController) GetTelegramClient() *telegram.Client {
	return c.telegramClient
}

func (c *TelegramController) GetConfig() *config.Config {
	return c.config
}

func (c *TelegramController) GetFileService() contracts.FileService {
	return c.fileService
}

func (c *TelegramController) GetDownloadService() contracts.DownloadService {
	return c.downloadService
}

func (c *TelegramController) GetSchedulerService() *services.SchedulerService {
	return c.schedulerService
}

func (c *TelegramController) GetMessageUtils() *utils.MessageUtils {
	return c.messageUtils
}

func (c *TelegramController) GetBasicCommands() *commands.BasicCommands {
	return c.basicCommands
}

func (c *TelegramController) GetDownloadCommands() types.DownloadCommandHandler {
	return c.downloadCommands
}

func (c *TelegramController) GetTaskCommands() *commands.TaskCommands {
	return c.taskCommands
}

func (c *TelegramController) GetMenuCallbacks() *callbacks.MenuCallbacks {
	return c.menuCallbacks
}