package valueobjects

// MediaType 媒体类型值对象
// 不可变的值对象,表示文件的媒体分类
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"   // 电影
	MediaTypeTV      MediaType = "tv"      // 电视剧
	MediaTypeVariety MediaType = "variety" // 综艺
	MediaTypeOther   MediaType = "other"   // 其他
	MediaTypeUnknown MediaType = "unknown" // 未知
)

// String 返回媒体类型的字符串表示
func (m MediaType) String() string {
	return string(m)
}

// IsValid 检查媒体类型是否有效
func (m MediaType) IsValid() bool {
	switch m {
	case MediaTypeMovie, MediaTypeTV, MediaTypeVariety, MediaTypeOther, MediaTypeUnknown:
		return true
	default:
		return false
	}
}

// IsVideo 判断是否为视频类型(电影/电视剧/综艺)
func (m MediaType) IsVideo() bool {
	return m == MediaTypeMovie || m == MediaTypeTV || m == MediaTypeVariety
}

// NewMediaType 创建媒体类型值对象,自动验证
func NewMediaType(value string) MediaType {
	mt := MediaType(value)
	if mt.IsValid() {
		return mt
	}
	return MediaTypeUnknown
}
