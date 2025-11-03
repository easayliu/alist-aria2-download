# 路径策略系统完整指南

## 📚 目录

- [概述](#概述)
- [核心功能](#核心功能)
- [配置指南](#配置指南)
- [使用场景](#使用场景)
- [高级功能](#高级功能)
- [故障排查](#故障排查)

---

## 概述

路径策略系统是一个强大而灵活的下载路径管理解决方案，支持：

- ✅ **智能路径生成** - 自动识别媒体类型并生成合适的路径
- ✅ **模板系统** - 使用变量和模板自定义路径结构
- ✅ **冲突检测** - 防止文件覆盖和重复下载
- ✅ **跨平台支持** - Windows/Linux/macOS路径自动适配
- ✅ **安全验证** - 路径长度、特殊字符、权限检查
- ✅ **目录管理** - 自动创建目录、磁盘空间检查

---

## 核心功能

### 1. 路径验证服务 (PathValidatorService)

**功能：**
- 路径长度验证（最大1024字节）
- 路径遍历攻击防护（`..`检测）
- 特殊字符检查和清理
- Windows保留名称检测
- 零宽字符和控制字符清理

**示例：**
```go
validator := NewPathValidatorService(config)

// 验证路径
err := validator.Validate("/downloads/tvs/节目名/S01")

// 清理路径
cleanPath := validator.CleanPath("/downloads/test:file")
// 结果: "/downloads/test-file"
```

### 2. 目录管理服务 (DirectoryManager)

**功能：**
- 自动创建嵌套目录
- 权限验证（可写性测试）
- 磁盘空间检查
- 缓存机制（避免重复检查）

**示例：**
```go
dirManager := NewDirectoryManager(config)

// 确保目录存在
err := dirManager.EnsureDirectory("/downloads/tvs/新节目/S01")

// 检查磁盘空间
err := dirManager.CheckDiskSpace("/downloads", 10*1024*1024*1024) // 10GB
```

### 3. 变量提取器 (VariableExtractor)

**支持的变量：**

| 变量 | 说明 | 示例 |
|------|------|------|
| `{base}` | 基础目录 | `/downloads` |
| `{category}` | 分类 | `tv`, `movie`, `variety` |
| `{show}` | 节目名称 | `明星大侦探` |
| `{season}` | 季度 | `S01`, `S08` |
| `{episode}` | 集数 | `E01`, `E12` |
| `{title}` | 电影标题 | `阿凡达` |
| `{movie_year}` | 电影年份 | `2009` |
| `{year}` | 当前年份 | `2025` |
| `{month}` | 当前月份 | `10` |
| `{day}` | 当前日期 | `01` |
| `{date}` | 完整日期 | `20251001` |
| `{filename}` | 文件名 | `episode.mp4` |

**示例：**
```go
extractor := NewVariableExtractor()

vars := extractor.ExtractVariables(file, "/downloads")
// vars = {
//   "base": "/downloads",
//   "category": "tv",
//   "show": "明星大侦探",
//   "season": "S08",
//   "episode": "E01",
//   ...
// }
```

### 4. 模板渲染器 (TemplateRenderer)

**功能：**
- 将模板和变量渲染成路径
- 支持不同分类的模板
- 自动清理未使用的占位符

**示例：**
```go
renderer := NewTemplateRenderer(templates)

path := renderer.Render("{base}/tvs/{show}/{season}", vars)
// 结果: "/downloads/tvs/明星大侦探/S08"
```

### 5. 冲突检测器 (ConflictDetector)

**功能：**
- 路径冲突检测
- 重复下载检测
- 三种冲突策略：skip/rename/overwrite

**示例：**
```go
detector := NewConflictDetector(config)

// 检查冲突
conflict, err := detector.CheckPathConflict("/downloads/tvs/节目名", "tv")

// 解决冲突
newPath, err := detector.ResolveConflict("/downloads/file.mp4", ConflictPolicyRename)
// 结果: "/downloads/file_1.mp4"
```

---

## 配置指南

### 基础配置

```yaml
download:
  path_config:
    # 基础设置
    auto_create_dir: true        # 自动创建目录
    max_path_length: 1024        # 最大路径长度
    validate_permissions: true   # 权限验证
    check_disk_space: true       # 磁盘空间检查

    # 冲突管理
    conflict_policy: "rename"    # skip/rename/overwrite
    skip_duplicates: false       # 跳过重复下载
```

### 模板配置

#### 默认模板（推荐）

```yaml
download:
  path_config:
    templates:
      tv: "{base}/tvs/{show}/{season}"
      movie: "{base}/movies/{title}"
      variety: "{base}/variety/{show}"
      default: "{base}/others"
```

**效果：**
- 电视剧：`/downloads/tvs/电视剧名/S08/`
- 电影：`/downloads/movies/电影名/`
- 综艺：`/downloads/variety/综艺节目名/`

#### 按年份分类

```yaml
download:
  path_config:
    templates:
      tv: "{base}/{year}/tvs/{show}/{season}"
      movie: "{base}/{year}/movies/{title}"
```

**效果：**
- `/downloads/2025/tvs/电视剧名/S08/`
- `/downloads/2025/movies/电影名/`

#### 按月份归档

```yaml
download:
  path_config:
    templates:
      tv: "{base}/{year}/{month}/tvs/{show}/{season}"
      movie: "{base}/{year}/{month}/movies/{title}"
```

**效果：**
- `/downloads/2025/10/tvs/明星大侦探/S08/`
- `/downloads/2025/10/movies/阿凡达/`

#### 电影按年份分类

```yaml
download:
  path_config:
    templates:
      movie: "{base}/movies/{movie_year}/{title}"
```

**效果：**
- `/downloads/movies/2009/阿凡达/`
- `/downloads/movies/2014/星际穿越/`

---

## 使用场景

### 场景1：家庭媒体库

**需求：**
- 电视剧按节目和季度组织
- 电影按名称组织
- 综艺单独分类

**配置：**
```yaml
download:
  path_config:
    templates:
      tv: "/media/tvs/{show}/{season}"
      movie: "/media/movies/{title}"
      variety: "/media/variety/{show}"
```

### 场景2：按时间归档

**需求：**
- 所有下载按年月归档
- 便于定期清理

**配置：**
```yaml
download:
  path_config:
    templates:
      tv: "/downloads/{year}/{month}/tvs/{show}/{season}"
      movie: "/downloads/{year}/{month}/movies/{title}"
      variety: "/downloads/{year}/{month}/variety/{show}"
```

### 场景3：多用户环境

**需求：**
- 不同用户下载到不同目录
- 避免冲突

**实现：**
通过代码动态设置baseDir：
```go
baseDir := fmt.Sprintf("/downloads/user_%d", userID)
path, err := pathStrategy.GenerateDownloadPath(file, baseDir)
```

### 场景4：存储优化

**需求：**
- 按文件大小分类
- 大文件和小文件分开存储

**配置：**
```yaml
download:
  path_config:
    templates:
      tv: "{base}/large/tvs/{show}/{season}"
      movie: "{base}/large/movies/{title}"
      default: "{base}/small"
```

---

## 高级功能

### 1. 路径映射规则引擎

**功能：**
- 复杂的路径转换规则
- 基于模式匹配
- 支持优先级

**示例规则：**
```go
rule := &PathMappingRule{
    ID:       "rule_variety_special",
    Name:     "综艺特别节目",
    Enabled:  true,
    Priority: 100,
    SourceMatch: SourceMatchRule{
        PathPattern: "*/tvs/综艺/*",
        MediaType:   "variety",
    },
    Transform: TransformRule{
        TargetTemplate: "{base}/variety/special/{show}",
    },
}

engine.AddRule(rule)
```

### 2. 跨平台路径适配

**功能：**
- 自动处理Windows/Linux/macOS路径差异
- 路径分隔符转换
- 保留名称检测

**示例：**
```go
adapter := NewPathAdapter()

// 规范化路径
path := adapter.NormalizePath("/downloads/tvs/节目")
// Windows: C:\downloads\tvs\节目
// Linux: /downloads/tvs/节目

// 验证路径
err := adapter.ValidatePath(path)

// 跨平台比较
same := adapter.ComparePaths(path1, path2)
```

### 3. 冲突策略详解

#### Skip（跳过）
```yaml
path_config:
  conflict_policy: "skip"
```
- 检测到冲突时跳过下载
- 适合：不希望覆盖现有文件

#### Rename（重命名）
```yaml
path_config:
  conflict_policy: "rename"
```
- 自动生成唯一文件名
- 策略：添加序号（file_1.mp4, file_2.mp4）
- 回退：使用时间戳（file_20251001_143022.mp4）

#### Overwrite（覆盖）
```yaml
path_config:
  conflict_policy: "overwrite"
```
- 直接覆盖现有文件
- ⚠️ 谨慎使用，可能丢失数据

### 4. 重复下载检测

```yaml
path_config:
  skip_duplicates: true
```

**功能：**
- 检测相同文件是否已下载
- 基于文件路径识别
- 避免重复下载

---

## 故障排查

### 问题1：路径过长

**症状：**
```
路径验证失败: 路径长度超过限制 (1500 > 1024)
```

**解决方案：**
```yaml
path_config:
  max_path_length: 2048  # 增加限制
```

或简化模板：
```yaml
templates:
  tv: "{base}/tv/{season}"  # 移除节目名
```

### 问题2：Windows保留名称

**症状：**
```
Windows保留名称: CON
```

**解决方案：**
- 自动处理：系统会自动清理路径
- 手动修改：避免使用保留名称（CON, PRN, AUX等）

### 问题3：目录创建失败

**症状：**
```
目录不可写: permission denied
```

**解决方案：**
1. 检查权限：`chmod 755 /downloads`
2. 检查磁盘空间：`df -h`
3. 禁用权限检查：
```yaml
path_config:
  validate_permissions: false
```

### 问题4：磁盘空间不足

**症状：**
```
磁盘空间不足：需要 10.0 GB，可用 5.0 GB
```

**解决方案：**
1. 清理磁盘空间
2. 禁用空间检查：
```yaml
path_config:
  check_disk_space: false
```

### 问题5：路径冲突

**症状：**
```
路径冲突：/downloads/tvs/节目名 已被 movie 类型占用
```

**解决方案：**
1. 使用rename策略：
```yaml
path_config:
  conflict_policy: "rename"
```

2. 修改模板避免冲突：
```yaml
templates:
  tv: "{base}/television/{show}"
  movie: "{base}/cinema/{title}"
```

---

## 最佳实践

### 1. 路径模板设计

**推荐：**
- ✅ 使用清晰的分类结构
- ✅ 保持路径深度适中（2-4层）
- ✅ 使用有意义的变量名

**避免：**
- ❌ 过深的目录结构（>5层）
- ❌ 过长的路径名称
- ❌ 特殊字符和空格过多

### 2. 冲突管理

**推荐配置：**
```yaml
path_config:
  conflict_policy: "rename"     # 自动重命名
  skip_duplicates: true         # 跳过重复
```

### 3. 性能优化

**建议：**
- ✅ 启用目录缓存（默认启用）
- ✅ 适当的空间检查阈值
- ✅ 合理的路径长度限制

### 4. 安全设置

**推荐：**
```yaml
path_config:
  auto_create_dir: true
  validate_permissions: true
  check_disk_space: true
  max_path_length: 1024
```

---

## 附录

### A. 完整配置示例

```yaml
aria2:
  download_dir: "/downloads"

download:
  video_only: true

  path_config:
    # 基础设置
    auto_create_dir: true
    max_path_length: 1024
    validate_permissions: true
    check_disk_space: true

    # 冲突管理
    conflict_policy: "rename"
    skip_duplicates: false

    # 路径模板
    templates:
      tv: "{base}/tvs/{show}/{season}"
      movie: "{base}/movies/{movie_year}/{title}"
      variety: "{base}/variety/{show}"
      default: "{base}/others"
```

### B. API参考

**PathStrategyService核心方法：**

```go
// 生成下载路径
path, err := pathStrategy.GenerateDownloadPath(file, baseDir)

// 准备下载目录（批量下载前）
err := pathStrategy.PrepareDownloadDirectory(baseDir, totalSize)

// 验证路径
err := pathStrategy.ValidatePath(path)

// 清理路径
cleanPath := pathStrategy.CleanPath(path)

// 规范化路径
normalPath := pathStrategy.NormalizePath(path)
```

### C. 变量完整列表

| 变量 | 类型 | 来源 | 示例 |
|------|------|------|------|
| `{base}` | 字符串 | 配置 | `/downloads` |
| `{category}` | 字符串 | 智能识别 | `tv`, `movie`, `variety`, `other` |
| `{show}` | 字符串 | 路径提取 | `电视剧名`, `综艺节目名` |
| `{season}` | 字符串 | 路径提取 | `S01`, `S08`, `S12` |
| `{episode}` | 字符串 | 文件名提取 | `E01`, `E12` |
| `{title}` | 字符串 | 路径提取 | `电影名A`, `电影名B` |
| `{movie_year}` | 字符串 | 路径提取 | `2009`, `2014` |
| `{year}` | 字符串 | 当前时间 | `2025` |
| `{month}` | 字符串 | 当前时间 | `01`, `10` |
| `{day}` | 字符串 | 当前时间 | `01`, `31` |
| `{date}` | 字符串 | 当前时间 | `20251001` |
| `{datetime}` | 字符串 | 当前时间 | `20251001_143022` |
| `{filename}` | 字符串 | 文件信息 | `episode.mp4` |
| `{ext}` | 字符串 | 文件信息 | `.mp4`, `.mkv` |
| `{file_year}` | 字符串 | 文件时间 | `2024` |
| `{file_month}` | 字符串 | 文件时间 | `12` |

---

## 更新日志

### v2.0 (2025-10-01)
- ✅ 实现完整的路径策略系统
- ✅ 支持模板和变量
- ✅ 冲突检测和处理
- ✅ 跨平台路径适配
- ✅ 规则映射引擎

### v1.0
- ✅ 基础路径验证
- ✅ 目录管理
- ✅ 智能路径生成

---

**文档维护：** 路径策略系统开发团队
**最后更新：** 2025-10-01
