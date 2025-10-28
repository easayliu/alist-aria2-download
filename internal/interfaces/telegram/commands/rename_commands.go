package commands

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/utils"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (bc *BasicCommands) HandleRename(chatID int64, command string) {
	ctx := context.Background()
	formatter := bc.messageUtils.GetFormatter().(*utils.MessageFormatter)

	parts := strings.Fields(command)
	if len(parts) < 2 {
		bc.messageUtils.SendMessageHTML(chatID,
			"<b>用法错误</b>\n\n使用方式：<code>/rename &lt;文件路径&gt;</code>\n\n示例：<code>/rename /movies/movie.mkv</code>")
		return
	}

	path := strings.Join(parts[1:], " ")

	bc.messageUtils.SendMessage(chatID, "正在从 TMDB 搜索重命名建议...")

	suggestions, err := bc.fileService.GetRenameSuggestions(ctx, path)
	if err != nil {
		logger.Error("Failed to get rename suggestions", "path", path, "error", err)

		if strings.Contains(err.Error(), "TMDB not configured") {
			bc.messageUtils.SendMessage(chatID,
				"<b>❌ TMDB 未配置</b>\n\n"+
				"请在 config.yaml 中配置 TMDB API Key：\n\n"+
				"<code>tmdb:\n  api_key: \"your_api_key\"\n  language: \"zh-CN\"</code>\n\n"+
				"获取 API Key: https://www.themoviedb.org/settings/api")
			return
		}

		bc.messageUtils.SendMessage(chatID, formatter.FormatError("获取重命名建议", err))
		return
	}

	if len(suggestions) == 0 {
		logger.Warn("No TMDB suggestions found", "path", path)
		bc.messageUtils.SendMessage(chatID,
			"<b>未找到匹配结果</b>\n\n"+
			"文件：<code>"+bc.messageUtils.EscapeHTML(path)+"</code>\n\n"+
			"可能原因：\n"+
			"• 文件名格式无法识别\n"+
			"• TMDB 数据库中没有该电影/剧集\n"+
			"• 文件名包含错误信息")
		return
	}

	encodedPath := base64.URLEncoding.EncodeToString([]byte(path))

	message := fmt.Sprintf("<b>重命名建议</b>\n\n原文件名：<code>%s</code>\n\n请选择新名称：\n\n", path)

	buttons := make([][]tgbotapi.InlineKeyboardButton, 0)
	for i, s := range suggestions {
		if i >= 5 {
			break
		}

		label := fmt.Sprintf("🎬 %s (%d)", s.Title, s.Year)
		if s.MediaType == "tv" && s.Season > 0 {
			label = fmt.Sprintf("📺 %s S%02dE%02d", s.Title, s.Season, s.Episode)
		}

		confidenceStr := ""
		if s.Confidence >= 0.9 {
			confidenceStr = "⭐⭐⭐"
		} else if s.Confidence >= 0.7 {
			confidenceStr = "⭐⭐"
		} else {
			confidenceStr = "⭐"
		}

		message += fmt.Sprintf("%d. %s %s\n<code>%s</code>\n\n", i+1, label, confidenceStr, s.NewName)

		callbackData := fmt.Sprintf("rename_apply|%d|%s", i, encodedPath)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%d. %s %s", i+1, label, confidenceStr),
				callbackData,
			),
		))
	}

	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ 取消", "rename_cancel"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	bc.messageUtils.SendMessageWithKeyboard(chatID, message, "HTML", &keyboard)
}
