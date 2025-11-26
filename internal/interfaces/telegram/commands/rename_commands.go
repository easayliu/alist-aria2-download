package commands

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
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
			"<b>用法错误</b>\n\n"+
				"使用方式：<code>/rename &lt;文件路径&gt; [--llm] [--strategy=xxx]</code>\n\n"+
				"示例：\n"+
				"<code>/rename /movies/movie.mkv</code>\n"+
				"<code>/rename /movies/movie.mkv --llm</code>\n"+
				"<code>/rename /movies/movie.mkv --llm --strategy=llm_only</code>")
		return
	}

	// 解析参数：检查是否有--llm标志
	useLLM := false
	strategy := "tmdb_first"
	var pathParts []string

	for i := 1; i < len(parts); i++ {
		if parts[i] == "--llm" {
			useLLM = true
		} else if strategyValue, found := strings.CutPrefix(parts[i], "--strategy="); found {
			strategy = strategyValue
			useLLM = true // 使用strategy暗示使用LLM
		} else {
			pathParts = append(pathParts, parts[i])
		}
	}

	if len(pathParts) == 0 {
		bc.messageUtils.SendMessageHTML(chatID, "<b>错误：</b>缺少文件路径参数")
		return
	}

	path := strings.Join(pathParts, " ")

	// 如果使用LLM模式，调用LLM重命名处理
	if useLLM {
		bc.HandleLLMRename(chatID, path, strategy)
		return
	}

	// 否则使用原有的TMDB模式
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
		if i >= MaxSuggestions {
			break
		}

		label := fmt.Sprintf("🎬 %s (%d)", s.Title, s.Year)
		if s.MediaType == "tv" && s.GetSeasonNumber() > 0 {
			label = fmt.Sprintf("📺 %s S%02dE%02d", s.Title, s.GetSeasonNumber(), s.GetEpisodeNumber())
		}

		confidenceStr := ""
		if s.Confidence >= HighConfidence {
			confidenceStr = "⭐⭐⭐"
		} else if s.Confidence >= MediumConfidence {
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

// HandleLLMRename 处理重命名命令(使用批量模式,即使只有单个文件)
func (bc *BasicCommands) HandleLLMRename(chatID int64, path string, strategy string) {
	ctx := context.Background()
	formatter := bc.messageUtils.GetFormatter().(*utils.MessageFormatter)

	// 发送初始消息
	bc.messageUtils.SendMessage(chatID, "🔍 正在分析文件名...")

	// 使用批量模式处理单个文件(统一使用TMDB批量API)
	suggestionsMap, _, err := bc.fileService.GetBatchRenameSuggestionsWithLLM(ctx, []string{path})
	if err != nil {
		logger.Error("Failed to get rename suggestions", "path", path, "error", err)

		// 检查特定错误
		errorMsg := formatter.FormatError("重命名", err)
		bc.messageUtils.SendMessage(chatID, errorMsg)
		return
	}

	// 获取结果
	suggestions, found := suggestionsMap[path]
	if !found || len(suggestions) == 0 {
		errorMsg := fmt.Sprintf("<b>未找到重命名建议</b>\n\n"+
			"文件：<code>%s</code>\n\n"+
			"可能原因：\n"+
			"• 文件名格式无法识别\n"+
			"• TMDB数据库中未找到匹配的影视作品",
			bc.messageUtils.EscapeHTML(path))
		bc.messageUtils.SendMessage(chatID, errorMsg)
		return
	}

	// 转换为旧格式以兼容后续逻辑
	result := &contracts.FileRenameResponse{
		OriginalName:  filepath.Base(path),
		SuggestedName: suggestions[0].NewName,
		Confidence:    float32(suggestions[0].Confidence),
		Source:        string(suggestions[0].Source),
		MediaInfo: &contracts.MediaInfo{
			Type:    string(suggestions[0].MediaType),
			Title:   suggestions[0].Title,
			TitleCN: suggestions[0].TitleCN,
			Year:    suggestions[0].Year,
			Season:  suggestions[0].Season,
			Episode: suggestions[0].Episode,
		},
	}

	// 如果没有结果,返回错误
	if result == nil {
		errorMsg := fmt.Sprintf("<b>未找到重命名建议</b>\n\n文件：<code>%s</code>", bc.messageUtils.EscapeHTML(path))
		bc.messageUtils.SendMessage(chatID, errorMsg)
		return
	}

	// 构建响应消息
	var message string
	if result == nil || result.SuggestedName == "" {
		message = fmt.Sprintf("<b>未找到重命名建议</b>\n\n"+
			"文件：<code>%s</code>\n\n"+
			"可能原因：\n"+
			"• 文件名格式无法识别\n"+
			"• LLM无法推断出有效的影视作品名称",
			bc.messageUtils.EscapeHTML(path))
	} else {
		// 显示置信度星级
		confidenceStr := ""
		if result.Confidence >= HighConfidence {
			confidenceStr = "⭐⭐⭐"
		} else if result.Confidence >= MediumConfidence {
			confidenceStr = "⭐⭐"
		} else {
			confidenceStr = "⭐"
		}

		// 显示来源图标
		sourceIcon := ""
		switch result.Source {
		case "llm":
			sourceIcon = "🤖"
		case "tmdb":
			sourceIcon = "🎬"
		case "hybrid":
			sourceIcon = "🔀"
		}

		message = fmt.Sprintf("<b>%s LLM重命名建议</b> %s\n\n"+
			"<b>原文件名：</b>\n<code>%s</code>\n\n"+
			"<b>推荐名称：</b>\n<code>%s</code>\n\n"+
			"<b>置信度：</b>%.2f %s\n"+
			"<b>来源：</b>%s",
			sourceIcon, confidenceStr,
			bc.messageUtils.EscapeHTML(path),
			bc.messageUtils.EscapeHTML(result.SuggestedName),
			result.Confidence, confidenceStr,
			result.Source)

		// 添加媒体信息（如果有）
		if result.MediaInfo != nil {
			message += "\n\n<b>媒体信息：</b>\n"
			message += fmt.Sprintf("类型：%s\n", result.MediaInfo.Type)
			if result.MediaInfo.Title != "" {
				message += fmt.Sprintf("标题：%s\n", result.MediaInfo.Title)
			}
			if result.MediaInfo.TitleCN != "" {
				message += fmt.Sprintf("中文标题：%s\n", result.MediaInfo.TitleCN)
			}
			if result.MediaInfo.Year > 0 {
				message += fmt.Sprintf("年份：%d\n", result.MediaInfo.Year)
			}
			if result.MediaInfo.Season != nil {
				message += fmt.Sprintf("季度：S%02d\n", *result.MediaInfo.Season)
			}
			if result.MediaInfo.Episode != nil {
				message += fmt.Sprintf("集数：E%02d\n", *result.MediaInfo.Episode)
			}
		}
	}

	bc.messageUtils.SendMessageHTML(chatID, message)
}
