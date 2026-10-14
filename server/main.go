package main

import (
	"log"
	"os"

	"knowbr/server/internal/api"
	"knowbr/server/internal/services"
	"knowbr/server/internal/store"
)

// 知行桥服务端入口：Gin + GORM(SQLite)。
func main() {
	cfg := api.Config{
		Port:    envOr("ZXQ_PORT", "8080"),
		WebDist: envOr("ZXQ_WEB_DIST", "../web/dist"),
		Version: "1.0.0",
	}
	dbPath := envOr("ZXQ_DB_PATH", "data/knowbr.db")

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败：%v", err)
	}

	llmCfg := services.LoadLLMConfig()
	llm := services.NewLLMClient(llmCfg)
	if llm.Available() {
		log.Printf("大模型接入：已启用（模型 %s，密钥仅保存在服务端）", llmCfg.Model)
	} else {
		log.Printf("大模型接入：未配置 API Key，按规则模板模式运行（功能完整可用）")
	}

	srv := api.NewServer(st, cfg, llm)
	log.Printf("知行桥服务端已启动：http://127.0.0.1:%s （数据库 %s）", cfg.Port, dbPath)
	if err := srv.Router().Run(":" + cfg.Port); err != nil {
		log.Fatalf("服务启动失败：%v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
