package callbacks

import (
	"context"
	"runtime"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/config"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/commands"
	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MenuCallbacks struct {
	downloadService contracts.DownloadService
	config          *config.Config
	messageUtils    types.MessageSender
	basicCommands   *commands.BasicCommands
}

func NewMenuCallbacks(downloadService contracts.DownloadService, config *config.Config, messageUtils types.MessageSender, basicCommands *commands.BasicCommands) *MenuCallbacks {
	return &MenuCallbacks{
		downloadService: downloadService,
		config:          config,
		messageUtils:    messageUtils,
		basicCommands:   basicCommands,
	}
}

func (mc *MenuCallbacks) HandleStartWithEdit(chatID int64, messageID int) {
	mc.basicCommands.HandleStartWithEdit(chatID, messageID)
}

func (mc *MenuCallbacks) HandleHelpWithEdit(chatID int64, messageID int) {
	mc.basicCommands.HandleHelpWithEdit(chatID, messageID)
}

// HandleStatusWithEdit handles status command (supports message editing)
func (mc *MenuCallbacks) HandleStatusWithEdit(chatID int64, messageID int) {
	ctx := context.Background()
	status, err := mc.downloadService.GetSystemStatus(ctx)
	if err != nil {
		message := "<b>系统状态</b>\n\n⚠️ 获取系统状态失败: " + err.Error()
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
			),
		)
		mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
		return
	}

	aria2Info := status["aria2"].(map[string]interface{})
	telegramInfo := status["telegram"].(map[string]interface{})
	serverInfo := status["server"].(map[string]interface{})

	message := "<b>系统状态</b>\n\n" +
		"<b>服务状态:</b>\n" +
		"• Telegram: " + telegramInfo["status"].(string) + "\n" +
		"• Aria2: " + aria2Info["status"].(string) + " (" + aria2Info["version"].(string) + ")\n" +
		"• 服务器: " + serverInfo["mode"].(string) + " 模式\n" +
		"• 端口: " + serverInfo["port"].(string)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 刷新", "cmd_status"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}

// HandleSystemStatusWithEdit handles system status (supports message editing)
func (mc *MenuCallbacks) HandleSystemStatusWithEdit(chatID int64, messageID int) {
	ctx := context.Background()
	status, err := mc.downloadService.GetSystemStatus(ctx)

	var message string
	if err != nil {
		message = "<b>系统状态</b>\n\n" +
			"⚠️ 获取系统状态失败: " + err.Error()
	} else {
		aria2Info := status["aria2"].(map[string]interface{})
		telegramInfo := status["telegram"].(map[string]interface{})
		serverInfo := status["server"].(map[string]interface{})

		message = "<b>系统状态</b>\n\n" +
			"<b>服务状态:</b>\n" +
			"• 服务器: " + serverInfo["mode"].(string) + " 模式\n" +
			"• 端口: " + serverInfo["port"].(string) + "\n" +
			"• Telegram: " + telegramInfo["status"].(string) + "\n" +
			"• Aria2: " + aria2Info["status"].(string) + " (" + aria2Info["version"].(string) + ")\n\n" +
			"<b>配置信息:</b>\n" +
			"• Alist地址: " + mc.config.Alist.BaseURL + "\n" +
			"• 下载目录: " + mc.config.Aria2.DownloadDir + "\n\n" +
			"<b>运行环境:</b>\n" +
			"• Go版本: " + runtime.Version() + "\n" +
			"• 系统: " + runtime.GOOS + "/" + runtime.GOARCH
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 刷新", "system_status"),
			tgbotapi.NewInlineKeyboardButtonData("🔌 Alist登录", "api_alist_login"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏥 健康检查", "api_health_check"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 主菜单", "back_main"),
		),
	)

	mc.messageUtils.EditMessageWithKeyboard(chatID, messageID, message, "HTML", &keyboard)
}
