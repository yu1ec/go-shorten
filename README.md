# Go Shorten

最快实现短链接跳转

### Running the Application

本地运行

```bash
go mod tidy
go run cmd/main.go
```

The server will start on `localhost:8080`. You can then use the endpoints to shorten URLs.

## API Endpoints

### POST /shorten

| 参数名      | 类型   | 是否必填 | 说明         |
| ----------- | ------ | -------- | ------------ |
| target_url  | string | 是       | 目标跳转地址 |
| short_code  | string | 否       | 自定义短码，不传则自动生成 |
| remark      | string | 否       | 备注         |

**请求示例：**
```json
{
  "target_url": "https://example.com",
  "short_code": "abc123",
  "remark": "示例"
}
```

**响应：**
- 成功：`200 OK`，body 为短码字符串
- 短码冲突：`409 Conflict`

---

### GET /:short_code

- 直接访问 `/abc123`，会跳转到对应的目标地址。
- 未找到短码时返回 404。


## 快速运行
```yaml
services:
  go-shorten:
    image: ghcr.io/yu1ec/go-shorten:latest
    container_name: go-shorten
    ports:
      - "5768:5768"
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```