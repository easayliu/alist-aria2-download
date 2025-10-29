package alist

import "time"

// FileListRequest 文件列表请求参数
type FileListRequest struct {
	Path     string `json:"path"`
	Password string `json:"password,omitempty"`
	Page     int    `json:"page,omitempty"`
	PerPage  int    `json:"per_page,omitempty"`
	Refresh  bool   `json:"refresh,omitempty"`
}

// FileListResponse 文件列表响应
type FileListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Content  []FileItem `json:"content"`
		Total    int        `json:"total"`
		Readme   string     `json:"readme"`
		Header   string     `json:"header"`
		Write    bool       `json:"write"`
		Provider string     `json:"provider"`
	} `json:"data"`
}

// FileItem 文件项
type FileItem struct {
	ID        string      `json:"id"`
	Path      string      `json:"path"`
	Name      string      `json:"name"`
	Size      int64       `json:"size"`
	IsDir     bool        `json:"is_dir"`
	Modified  string      `json:"modified"`
	Created   string      `json:"created"`
	Sign      string      `json:"sign"`
	Thumb     string      `json:"thumb"`
	Type      int         `json:"type"`
	HashInfo  *HashInfo   `json:"hash_info,omitempty"`
	LabelList []FileLabel `json:"label_list,omitempty"`
}

// HashInfo 文件哈希信息
type HashInfo struct {
	MD5 string `json:"md5"`
}

// FileLabel 文件标签
type FileLabel struct {
	ID         int    `json:"id"`
	Type       int    `json:"type"`
	Name       string `json:"name"`
	CreateTime string `json:"create_time"`
}

// SimplifiedFileItem 简化的文件项（供前端使用）
type SimplifiedFileItem struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"is_dir"`
	Modified time.Time `json:"modified"`
	Sign     string    `json:"sign,omitempty"`
}

// FileGetRequest 获取文件信息请求
type FileGetRequest struct {
	Path     string `json:"path"`
	Password string `json:"password,omitempty"`
}

// FileGetResponse 获取文件信息响应
type FileGetResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Name     string      `json:"name"`
		Size     int64       `json:"size"`
		IsDir    bool        `json:"is_dir"`
		Modified string      `json:"modified"`
		Created  string      `json:"created"`
		Sign     string      `json:"sign"`
		Thumb    string      `json:"thumb"`
		Type     int         `json:"type"`
		HashInfo interface{} `json:"hash_info"`
		RawURL   string      `json:"raw_url"`
		Readme   string      `json:"readme"`
		Header   string      `json:"header"`
		Provider string      `json:"provider"`
		Related  interface{} `json:"related"`
	} `json:"data"`
}

type RenameRequest struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

type RenameResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data"`
}

type MoveRequest struct {
	SrcDir string   `json:"src_dir"`
	DstDir string   `json:"dst_dir"`
	Names  []string `json:"names"`
	Overwrite bool  `json:"overwrite"`
}

type MoveResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type MkdirRequest struct {
	Path string `json:"path"`
}

type MkdirResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type RemoveRequest struct {
	Names []string `json:"names"`
	Dir   string   `json:"dir"`
}

type RemoveResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
