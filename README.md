# 中英句子练习

个人用的中文句子 → 英文翻译练习小站。把想练的句子记在本机 SQLite 里，练习时随机抽出一句中文，先自己写英文，再和参考译文并排对照。不需要登录。

## 运行

需要 Go 1.22 或更高版本。SQLite 使用纯 Go 驱动 [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)，不需要 CGO，也不需要单独安装 SQLite。

```bash
go run .
```

浏览器打开 <http://localhost:8080>。

```bash
go test ./...
```

编译成单个可执行文件：

```bash
CGO_ENABLED=0 go build -o translation-practice .
./translation-practice
```

## 配置

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 端口 |
| `DB_PATH` | `data/sentences.db` | SQLite 文件路径 |
| `SEED` | 不设置 | 设为 `1` 时，如果数据库是空的，写入几句示例 |

示例：

```bash
PORT=3000 DB_PATH=data/sentences.db SEED=1 go run .
```

第一次启动会自动创建 `data/` 目录和数据库文件。`data/` 里的数据库已加入 `.gitignore`（保留 `data/.gitkeep`）。页面上的保存时间按运行程序的电脑时区显示，文件里存的是 UTC。

## 用法

1. 打开「添加」，填写中文句子和英文译文，保存。
2. 打开「练习」。页面随机显示一句已保存的中文，不显示英文。
3. 写下你的英文并提交。你的译文和参考译文会并排显示。
4. 用「再练一句」换一句，或「再试一次」重做刚才那句。

表 `sentences` 的字段：`id`，`chinese`，`english`，`created_at`。
