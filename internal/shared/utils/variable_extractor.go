package utils

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/easayliu/alist-aria2-download/internal/application/contracts"
	"github.com/easayliu/alist-aria2-download/pkg/logger"
	pathutil "github.com/easayliu/alist-aria2-download/pkg/utils/path"
	strutil "github.com/easayliu/alist-aria2-download/pkg/utils/string"
)

// VariableExtractor 变量提取器 - 从文件信息中提取可用于模板的变量
type VariableExtractor struct {
	timeLocation *time.Location
	fileFilter   *FileFilterService
}

// NewVariableExtractor 创建变量提取器
func NewVariableExtractor() *VariableExtractor {
	loc, err := time.LoadLocation("Local")
	if err != nil {
		loc = time.UTC
	}

	return &VariableExtractor{
		timeLocation: loc,
		fileFilter:   NewFileFilterService(),
	}
}

// ExtractVariables 提取所有可用变量
func (e *VariableExtractor) ExtractVariables(
	file contracts.FileResponse,
	baseDir string,
) map[string]string {
	vars := make(map[string]string)

	// 1. 基础变量
	vars["base"] = baseDir
	vars["filename"] = file.Name
	vars["path"] = file.Path

	// 2. 时间变量（当前时间）
	now := time.Now().In(e.timeLocation)
	vars["year"] = now.Format("2006")
	vars["month"] = now.Format("01")
	vars["day"] = now.Format("02")
	vars["date"] = now.Format("20060102")
	vars["datetime"] = now.Format("20060102_150405")

	// 3. 文件时间变量
	if !file.Modified.IsZero() {
		vars["file_year"] = file.Modified.Format("2006")
		vars["file_month"] = file.Modified.Format("01")
		vars["file_date"] = file.Modified.Format("20060102")
	}

	// 4. 媒体类型相关变量
	if e.fileFilter.IsTVShow(file.Path) {
		vars["category"] = "tv"
		vars["show"] = e.extractShowName(file.Path)
		vars["season"] = e.extractSeason(file.Path)
		vars["episode"] = e.extractEpisode(file.Name)
	} else if e.fileFilter.IsMovie(file.Path) {
		vars["category"] = "movie"
		vars["title"] = e.extractMovieTitle(file.Path)
		vars["movie_year"] = e.extractMovieYear(file.Path)
	} else if e.fileFilter.IsVarietyShow(file.Path) {
		vars["category"] = "variety"
		vars["show"] = e.extractShowName(file.Path)
	} else {
		vars["category"] = "other"
	}

	// 5. 路径相关变量
	vars["original_dir"] = filepath.Dir(file.Path)
	vars["parent_dir"] = filepath.Base(filepath.Dir(file.Path))

	// 6. 文件扩展名
	vars["ext"] = filepath.Ext(file.Name)

	logger.Debug("Variable extraction completed",
		"filename", file.Name,
		"category", vars["category"],
		"show", vars["show"],
		"season", vars["season"])

	return vars
}

// extractShowName 提取节目名称
func (e *VariableExtractor) extractShowName(path string) string {
	// 优先从路径中提取
	pathLower := strings.ToLower(path)

	// 查找 /tvs/ 或 /variety/ 后的第一个有意义的目录作为节目名
	patterns := []string{"/tvs/", "/variety/", "/综艺/"}
	for _, pattern := range patterns {
		if idx := strings.Index(pathLower, pattern); idx != -1 {
			afterPattern := path[idx+len(pattern):]
			parts := strings.Split(afterPattern, "/")

			// 跳过常见分类目录、年份目录和季度目录
			for _, part := range parts {
				if part == "" {
					continue
				}

				// 使用增强的跳过检测（包含年份）
				if pathutil.ShouldSkipDirectoryAdvanced(part) {
					logger.Debug("Skipping category directory", "dir", part)
					continue
				}

				// 跳过季度目录
				if strutil.IsSeasonDirectory(part) {
					logger.Debug("Skipping season directory", "dir", part)
					continue
				}

				// 找到第一个非分类目录，作为节目名
				cleaned := e.cleanShowName(part)
				logger.Debug("Show name extracted",
					"path", path,
					"part", part,
					"cleaned", cleaned)
				return cleaned
			}
		}
	}

	// 回退：使用父目录名
	baseName := filepath.Base(filepath.Dir(path))
	logger.Debug("Using fallback for show name",
		"path", path,
		"parentDir", baseName)
	return baseName
}

// cleanShowName 清理节目名称（使用公共工具函数）
func (e *VariableExtractor) cleanShowName(name string) string {
	cleaned := strutil.CleanShowName(name)
	logger.Debug("Show name cleaned", "original", name, "cleaned", cleaned)
	return cleaned
}

// extractSeason 提取季度信息（使用公共工具函数）
func (e *VariableExtractor) extractSeason(path string) string {
	return strutil.ExtractSeason(path)
}

// extractEpisode 提取集数信息
func (e *VariableExtractor) extractEpisode(filename string) string {
	filenameLower := strings.ToLower(filename)

	// 模式1: E01, E02 格式
	if matches := strutil.EpisodePattern.FindStringSubmatch(filenameLower); len(matches) > 1 {
		episodeNum, _ := strconv.Atoi(matches[1])
		return "E" + padZero(episodeNum, 2)
	}

	// 模式2: EP01, EP02 格式
	if matches := strutil.EpisodeEPPattern.FindStringSubmatch(filenameLower); len(matches) > 1 {
		episodeNum, _ := strconv.Atoi(matches[1])
		return "E" + padZero(episodeNum, 2)
	}

	// 模式3: 第X集
	if matches := strutil.ChineseEpisodePattern.FindStringSubmatch(filename); len(matches) > 1 {
		episodeNum := strutil.ChineseToNumber(matches[1])
		if episodeNum > 0 {
			return "E" + padZero(episodeNum, 2)
		}
	}

	return ""
}

// extractMovieTitle 提取电影标题
func (e *VariableExtractor) extractMovieTitle(path string) string {
	// 查找 /movies/ 后的第一个有意义的目录作为电影名
	pathLower := strings.ToLower(path)
	if idx := strings.Index(pathLower, "/movies/"); idx != -1 {
		afterMovies := path[idx+8:] // "/movies/" 长度为8
		parts := strings.Split(afterMovies, "/")

		// 跳过常见分类目录、年份目录和季度目录
		for _, part := range parts {
			if part == "" {
				continue
			}

			// 使用增强的跳过检测（包含年份）
			if pathutil.ShouldSkipDirectoryAdvanced(part) {
				logger.Debug("Skipping movie category directory", "dir", part)
				continue
			}

			// 跳过季度目录
			if strutil.IsSeasonDirectory(part) {
				logger.Debug("Skipping season directory in movie path", "dir", part)
				continue
			}

			// 找到第一个非分类目录，作为电影名
			cleaned := e.cleanMovieTitle(part)
			logger.Debug("Movie title extracted",
				"path", path,
				"part", part,
				"cleaned", cleaned)
			return cleaned
		}
	}

	// 回退：使用文件名（去除扩展名和年份）
	basename := filepath.Base(path)
	basename = strings.TrimSuffix(basename, filepath.Ext(basename))
	cleaned := e.cleanMovieTitle(basename)
	logger.Debug("Using fallback for movie title",
		"path", path,
		"basename", basename,
		"cleaned", cleaned)
	return cleaned
}

// cleanMovieTitle 清理电影标题
func (e *VariableExtractor) cleanMovieTitle(title string) string {
	// 直接使用 CleanShowName，它已经包含了所有清理逻辑：
	// - 移除网站水印（【xxx】[xxx]）
	// - 移除视频质量信息（1080p, WEB-DL等）
	// - 移除编码信息（H265, x264等）
	// - 移除音频信息（DDP5.1等）
	// - 移除发布组名
	// - 移除季度信息
	// - 提取中文部分
	cleaned := strutil.CleanShowName(title)
	logger.Debug("Movie title cleaned", "original", title, "cleaned", cleaned)
	return cleaned
}

// extractMovieYear 提取电影年份
func (e *VariableExtractor) extractMovieYear(path string) string {
	// 查找年份：(2009), [2014], 2020 等格式
	if matches := strutil.YearPattern.FindStringSubmatch(path); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// padZero 数字补零
func padZero(num int, width int) string {
	numStr := strconv.Itoa(num)
	if len(numStr) < width {
		return strings.Repeat("0", width-len(numStr)) + numStr
	}
	return numStr
}
