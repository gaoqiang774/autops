# API 描述自动同步工具

## 🎯 功能

自动从 Swagger 注解中提取 API 描述，并更新到 `middleware/apiDescriptions.go`，实现操作日志自动填充描述。

## 📦 文件

```
sync_api_descriptions.py          # 一键同步脚本 ⭐
middleware/apiDescriptions.go     # 自动生成的映射表
```

## 🚀 使用方法

### 一键同步

```bash
python3 sync_api_descriptions.py
```

就这么简单！脚本会自动：
1. 扫描所有 `api/*/controller/*.go` 文件
2. 提取 `@Description` 注解
3. 更新 `middleware/apiDescriptions.go`
4. 显示统计信息

## 📝 添加新 API

在 controller 中添加 `@Description` 注解：

```go
// @Tags AI模型管理
// @Summary 获取AI模型详情
// @Description 根据ID获取AI模型详情  // ← 添加这一行
// @Param id query int true "模型ID"
// @Success 200 {object} result.Result
// @Router /api/v1/ai/model/info [get]
func GetAiModelInfo(c *gin.Context) {
    // ...
}
```

然后运行：

```bash
python3 sync_api_descriptions.py
```

重启应用即可。

## ✅ 验证效果

访问 API 后查看操作日志：

```bash
curl http://localhost:8080/api/v1/sysOperationLog/list
```

返回结果：

```json
{
    "data": {
        "list": [
            {
                "id": 5,
                "method": "delete",
                "url": "/api/v1/ai/model/delete",
                "description": "根据ID删除AI模型",  // ← 自动填充
                "createTime": "2026-01-19 13:20:00"
            }
        ]
    }
}
```

## 🔧 工作原理

```
Controller 注解 → 扫描提取 → 更新映射表 → 中间件获取 → 数据库记录
```

中间件代码（已集成在 `middleware/logMiddleware.go`）：

```go
func LogMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()

        method := strings.ToLower(c.Request.Method)
        if method != "get" {
            url := c.Request.URL.Path
            log := model.SysOperationLog{
                Method:      method,
                Url:         url,
                Description: GetAPIDescription(url, method), // 自动获取
                // ...
            }
            dao.CreateSysOperationLog(log)
        }
    }
}
```

## 📊 当前统计

运行脚本后会显示：
- 总 API 数
- 有描述的 API 数量和百分比
- 缺少描述的 API 列表（前10个）

## ⚠️ 注意事项

1. 每次添加或修改 API 后运行脚本
2. 运行后需要重启应用
3. 建议为所有 API 添加 `@Description` 注解
4. 描述要简洁明了（10-20字）

## 💡 最佳实践

### ✅ 好的描述

```go
// @Description 创建新的AI模型配置
// @Description 根据ID获取AI模型详情
// @Description 批量删除操作日志
```

### ❌ 不好的描述

```go
// @Description 获取信息  // 太模糊
// @Description 这个接口用于...  // 太啰嗦
```

## 🎉 完成

现在你只需要：
1. 运行 `python3 sync_api_descriptions.py`
2. 重启应用
3. 操作日志自动包含描述！
