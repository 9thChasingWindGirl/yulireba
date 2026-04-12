
# PlayFast

> 一款基于 sing-box 的游戏加速器，采用 Wails 框架开发

## 简介

PlayFast 是一个专为游戏优化的网络加速工具，基于强大的 sing-box 内核，提供稳定可靠的加速服务。

**源自项目：** https://github.com/danbai225/gpp

## 特性

- 🎮 **主机游戏加速**：支持 PlayStation、Xbox、Switch 等主机
- ⚙️ **自定义配置**：支持自定义节点和规则配置
- 🚀 **高性能内核**：基于 sing-box 核心，性能稳定
- 💻 **现代界面**：基于 Wails 框架的原生 GUI

## 预览

| 加速前 | 加速后 |
|--------|--------|
| ![加速前](./res/1.png) | ![加速后](./res/2.png) |

## 部署指南

### 📦 客户端构建

1. **克隆项目**
   ```bash
   git clone https://github.com/narwhal-cloud/playfast
   cd playfast
   ```

2. **配置域名**
   ```bash
   # 修改 internal/api/patch.go 中的域名配置
   ```

3. **构建应用**
   ```cmd
   # Windows
   build.bat
   ```

### 🌐 后端部署

后端需要提供以下 API 端点：

#### 📢 公告接口
- **路径**：`/announcement`
- **说明**：返回 HTML 格式的公告内容，将在客户端显示

#### 📋 规则文件

##### 🚫 黑名单规则
- **文件**：`black-list.json`
- **格式**：[sing-box 规则集格式](https://sing-box.sagernet.org/configuration/rule-set/source-format/)

##### 🔗 直连规则  
- **文件**：`direct-list.json`
- **格式**：同黑名单规则格式

##### 🌍 代理节点配置
- **文件**：`proxy.json`
- **示例**：
```json
[
  {
    "name": "香港节点1",
    "protocol": "shadowsocks",
    "method": "ase-256-gcm",
    "password": "your_password",
    "host": "1.2.3.4",
    "port": 1234
  },
  {
    "name": "美国节点1", 
    "protocol": "vless",
    "password": "your_uuid",
    "host": "5.6.7.8",
    "port": 443
  }
]
```

##### 🔄 版本更新配置
- **文件**：`version.json`
- **示例**：
```json
{
  "version": "v1.0.0",
  "url_windows": "https://api.example.com/PlayFast.exe",
  "sha256_windows": "66e2d9ca30a774061f3d9860757bb46799a2a8126b33c00db3a33546434c2248"
}
```

## 支持的协议

- Shadowsocks
- VLESS
- SOCKS5

## 许可证

本项目遵循开源许可证，具体请查看 LICENSE 文件。

## 贡献

欢迎提交 Issue 和 Pull Request 来帮助改进项目！
