package services

import (
	"github.com/easayliu/alist-aria2-download/internal/infrastructure/alist"
	"testing"
)

func TestDetermineMediaTypeAndPath(t *testing.T) {
	fs := NewFileService(&alist.Client{})

	tests := []struct {
		name        string
		fullPath    string
		fileName    string
		wantType    MediaType
		wantPath    string
		description string
	}{
		{
			name:        "纯数字文件名应识别为TV剧集",
			fullPath:    "/data/来自：分享/不眠日/08.mp4",
			fileName:    "08.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/不眠日/S1",
			description: "08.mp4这样的纯数字文件名是剧集常见格式",
		},
		{
			name:        "第一集纯数字",
			fullPath:    "/data/来自：分享/不眠日/01.mp4",
			fileName:    "01.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/不眠日/S1",
			description: "01.mp4表示第一集",
		},
		{
			name:        "两位数集数",
			fullPath:    "/data/来自：分享/某剧集/12.mp4",
			fileName:    "12.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某剧集/S1",
			description: "12.mp4表示第12集",
		},
		{
			name:        "三位数集数",
			fullPath:    "/data/tvs/长篇动画/156.mp4",
			fileName:    "156.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/长篇动画/S1",
			description: "156.mp4表示第156集",
		},
		{
			name:        "带前导零的集数",
			fullPath:    "/data/series/某剧/001.mp4",
			fileName:    "001.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某剧/S1",
			description: "001.mp4带前导零的格式",
		},
		{
			name:        "电影文件",
			fullPath:    "/data/movies/Avatar.2022.4K.BluRay.mp4",
			fileName:    "Avatar.2022.4K.BluRay.mp4",
			wantType:    MediaTypeMovie,
			wantPath:    "/downloads/movies/Avatar",
			description: "带年份和质量标记的电影文件",
		},
		{
			name:        "标准剧集格式",
			fullPath:    "/data/tvs/Breaking.Bad/S01E01.mp4",
			fileName:    "S01E01.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/Breaking.Bad/S01",
			description: "S##E##标准格式",
		},
		{
			name:        "中文季度格式",
			fullPath:    "/data/tvs/某剧集/第1季/第3集.mp4",
			fileName:    "第3集.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某剧集/S01",
			description: "中文季度和集数格式",
		},
		{
			name:        "电影系列",
			fullPath:    "/data/movies/速度与激情系列/速度与激情8.mp4",
			fileName:    "速度与激情8.mp4",
			wantType:    MediaTypeMovie,
			wantPath:    "/downloads/movies/速度与激情",
			description: "电影系列应识别为电影",
		},
		{
			name:        "甄嬛传E03应识别为TV剧",
			fullPath:    "/data/来自：分享/甄嬛传4K收藏版/后宫·甄嬛传.Empresses.in.the.Palace.2011.E03.WEB-DL.4k.H265.10bit.AAC.mp4",
			fileName:    "后宫·甄嬛传.Empresses.in.the.Palace.2011.E03.WEB-DL.4k.H265.10bit.AAC.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/甄嬛传4K收藏版/S1",
			description: "包含E03的文件应识别为TV剧集",
		},
		{
			name:        "大写EP格式应识别为TV剧",
			fullPath:    "/data/tvs/某剧/EP05.mp4",
			fileName:    "EP05.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某剧/S1",
			description: "大写EP格式应被识别",
		},
		{
			name:        "大写E格式应识别为TV剧",
			fullPath:    "/data/series/测试剧/E12.mp4",
			fileName:    "E12.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/测试剧/S1",
			description: "大写E格式应被识别",
		},
		{
			name:        "甄嬛传E74高集数应识别为TV剧",
			fullPath:    "/data/来自：分享/甄嬛传4K收藏版/后宫·甄嬛传.Empresses.in.the.Palace.2011.E74.WEB-DL.4k.H265.10bit.AAC.mp4",
			fileName:    "后宫·甄嬛传.Empresses.in.the.Palace.2011.E74.WEB-DL.4k.H265.10bit.AAC.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/甄嬛传4K收藏版/S1",
			description: "E74等高集数应被正确识别为TV剧",
		},
		{
			name:        "三位数集数E100应识别为TV剧",
			fullPath:    "/data/长篇剧/某剧/E100.mp4",
			fileName:    "E100.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某剧/S1",
			description: "三位数集数应被识别",
		},
		{
			name:        "小写e格式也应识别",
			fullPath:    "/data/anime/某动画/e156.mp4",
			fileName:    "e156.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某动画/S1",
			description: "小写e格式也应被识别",
		},
		{
			name:        "甄嬛传S01EP76带版本目录",
			fullPath:    "/data/来自：分享/甄嬛传4K收藏版/4K[DV][60帧][高码率]/甄嬛传.S01EP76.2011.2160p.WEB-DL.HQ.DV.H265.60fps.10bit.AAC.mp4",
			fileName:    "甄嬛传.S01EP76.2011.2160p.WEB-DL.HQ.DV.H265.60fps.10bit.AAC.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/甄嬛传/4K[DV][60帧][高码率]",
			description: "S##EP##格式应识别版本目录结构",
		},
		{
			name:        "带版本目录的剧集",
			fullPath:    "/data/tvs/某剧集高清版/1080P[完整版]/某剧.S01EP01.mp4",
			fileName:    "某剧.S01EP01.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某剧集/1080P[完整版]",
			description: "应正确提取剧名并保留版本目录",
		},
		{
			name:        "喜人奇妙夜s1目录",
			fullPath:    "/data/来自：分享/喜人奇妙夜/s1/20240628.纯享版.mp4",
			fileName:    "20240628.纯享版.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/喜人奇妙夜/S01",
			description: "s1目录应被识别为第一季",
		},
		{
			name:        "s2季度目录",
			fullPath:    "/data/综艺/某节目/s2/EP01.mp4",
			fileName:    "EP01.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某节目/S02",
			description: "s2应被识别为第二季",
		},
		{
			name:        "大写S1季度目录",
			fullPath:    "/data/shows/某剧/S1/episode01.mp4",
			fileName:    "episode01.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某剧/S01",
			description: "大写S1也应被识别",
		},
		{
			name:        "喜人奇妙夜综艺节目先导片",
			fullPath:    "/data/来自：分享/喜人奇妙夜/20250919先导1：团长集结！马东召集喜剧半壁江山[4K60FPS].mp4",
			fileName:    "20250919先导1：团长集结！马东召集喜剧半壁江山[4K60FPS].mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/喜人奇妙夜/S1",
			description: "包含先导和知名综艺名称应识别为TV",
		},
		{
			name:        "喜人奇妙夜纯享版",
			fullPath:    "/data/来自：分享/喜人奇妙夜/20250926第1期上纯享版：最强喜剧“新”人爆改三国.mp4",
			fileName:    "20250926第1期上纯享版：最强喜剧“新”人爆改三国.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/喜人奇妙夜/S1",
			description: "包含综艺特征词如纯享版应识别为TV",
		},
		{
			name:        "tvs分类目录不应进入剧名",
			fullPath:    "/data/来自：分享/tvs/向往的生活 第八季/S08.2025.2160p.WEB-DL.H265.AAC/向往的生活 第八季.EP01.mp4",
			fileName:    "向往的生活 第八季.EP01.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/向往的生活/S08",
			description: "分类目录名tvs不应作为剧名，并应正确提取季度信息",
		},
		{
			name:        "日期格式综艺节目",
			fullPath:    "/data/variety/某节目/20240101.本期嘉宾.mp4",
			fileName:    "20240101.本期嘉宾.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/某节目/S1",
			description: "8位日期格式文件应识别为综艺",
		},
		{
			name:        "知名综艺向往的生活",
			fullPath:    "/data/shows/向往的生活/第六季/第10期.mp4",
			fileName:    "第10期.mp4",
			wantType:    MediaTypeTV,
			wantPath:    "/downloads/tvs/向往的生活/S06",
			description: "知名综艺应被识别",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotPath := fs.determineMediaTypeAndPath(tt.fullPath, tt.fileName)

			if gotType != tt.wantType {
				t.Errorf("determineMediaTypeAndPath() 类型判断错误\n"+
					"路径: %s\n"+
					"期望类型: %v\n"+
					"实际类型: %v\n"+
					"说明: %s",
					tt.fullPath, tt.wantType, gotType, tt.description)
			}

			if gotPath != tt.wantPath {
				t.Errorf("determineMediaTypeAndPath() 下载路径错误\n"+
					"路径: %s\n"+
					"期望路径: %s\n"+
					"实际路径: %s\n"+
					"说明: %s",
					tt.fullPath, tt.wantPath, gotPath, tt.description)
			}
		})
	}
}

func TestIsEpisodeNumber(t *testing.T) {
	fs := NewFileService(&alist.Client{})

	tests := []struct {
		input    string
		expected bool
		desc     string
	}{
		{"01", true, "两位数字带前导零"},
		{"08", true, "单个数字带前导零"},
		{"12", true, "两位数字"},
		{"001", true, "三位数字带前导零"},
		{"100", true, "三位数字"},
		{"999", true, "最大三位数"},
		{"1000", false, "四位数超出范围"},
		{"0", false, "零不是有效集数"},
		{"abc", false, "非数字"},
		{"1a", false, "包含字母"},
		{"", false, "空字符串"},
		{" 12 ", true, "带空格的数字（会被trim）"},
		{"00", false, "双零"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := fs.isEpisodeNumber(tt.input)
			if got != tt.expected {
				t.Errorf("isEpisodeNumber(%q) = %v, want %v (%s)",
					tt.input, got, tt.expected, tt.desc)
			}
		})
	}
}

func TestHasEpisodePattern(t *testing.T) {
	fs := NewFileService(&alist.Client{})

	tests := []struct {
		input    string
		expected bool
		desc     string
	}{
		{"E01", true, "E01格式"},
		{"E74", true, "E74高集数"},
		{"E100", true, "E100三位数"},
		{"E999", true, "E999最大三位数"},
		{"e01", true, "小写e01"},
		{"e156", true, "小写e156"},
		{"EP01", true, "EP01格式"},
		{"EP74", true, "EP74高集数"},
		{"ep01", true, "小写ep01"},
		{"ep100", true, "小写ep100"},
		{"后宫·甄嬛传.E74.WEB-DL", true, "文件名中的E74"},
		{"Breaking.Bad.S01E05.720p", true, "S01E05格式中的E05"},
		{"movie.2022.BluRay", false, "没有集数格式"},
		{"E1000", false, "四位数超出范围"},
		{"E0", false, "E0无效"},
		{"Episode", false, "只有Episode没有数字"},
		{"test.mp4", false, "普通文件名"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := fs.hasEpisodePattern(tt.input)
			if got != tt.expected {
				t.Errorf("hasEpisodePattern(%q) = %v, want %v (%s)",
					tt.input, got, tt.expected, tt.desc)
			}
		})
	}
}

func TestIsKnownTVShow(t *testing.T) {
	fs := NewFileService(&alist.Client{})

	tests := []struct {
		input    string
		expected bool
		desc     string
	}{
		{"/data/来自：分享/喜人奇妙夜/20250919先导1.mp4", true, "喜人奇妙夜是知名综艺"},
		{"/data/shows/向往的生活/第六季/第10期.mp4", true, "向往的生活是知名综艺"},
		{"/data/variety/某节目/20240101.纯享版.mp4", true, "包含纯享版特征"},
		{"/data/variety/某节目/第5期先导片.mp4", true, "包含先导特征"},
		{"/data/shows/20240628.精华版.mp4", true, "日期格式+精华版"},
		{"/data/movies/Avatar.2022.4K.BluRay.mp4", false, "普通电影文件"},
		{"/data/shows/某剧集/S01E01.mp4", false, "普通剧集文件"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := fs.isKnownTVShow(tt.input)
			if got != tt.expected {
				t.Errorf("isKnownTVShow(%q) = %v, want %v (%s)",
					tt.input, got, tt.expected, tt.desc)
			}
		})
	}
}
