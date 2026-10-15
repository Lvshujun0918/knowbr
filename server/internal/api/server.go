package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"knowbr/server/internal/content"
	"knowbr/server/internal/models"
	"knowbr/server/internal/services"
	"knowbr/server/internal/store"
)

// Config 是服务端运行配置。
type Config struct {
	Port    string
	WebDist string
	Version string
}

// Server 持有依赖并对外提供路由。
type Server struct {
	store     *store.Store
	llm       *services.LLMClient
	llmModel  string
	webDist   string
	version   string
	startedAt time.Time
}

// NewServer 构造服务端。
func NewServer(st *store.Store, cfg Config, llm *services.LLMClient) *Server {
	model := ""
	if llm != nil {
		model = llm.Model()
	}
	return &Server{
		store:     st,
		llm:       llm,
		llmModel:  model,
		webDist:   cfg.WebDist,
		version:   cfg.Version,
		startedAt: time.Now(),
	}
}

// Router 组装全部路由。
func (s *Server) Router() *gin.Engine {
	if strings.ToLower(os.Getenv("ZXQ_DEBUG")) != "true" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), cors())

	apiGroup := r.Group("/api")
	{
		apiGroup.GET("/health", s.Health)
		apiGroup.GET("/meta", s.Meta)
		apiGroup.GET("/demo-cases", s.DemoCases)
		apiGroup.POST("/metrics/compute", s.ComputeMetricsEndpoint)
		apiGroup.GET("/sessions", s.ListSessions)
		apiGroup.POST("/sessions", s.CreateSession)
		apiGroup.DELETE("/sessions", s.ClearSessions)
		apiGroup.GET("/sessions/:id", s.GetSession)
		apiGroup.DELETE("/sessions/:id", s.DeleteSession)
		apiGroup.POST("/sessions/:id/hints", s.AddHint)
		apiGroup.GET("/sessions/:id/agent", s.GetAgentDecision)
		apiGroup.POST("/sessions/:id/agent/step", s.AgentStepEndpoint)
		apiGroup.POST("/sessions/:id/revisions", s.AddRevision)
		apiGroup.POST("/sessions/:id/transfer/question", s.TransferQuestion)
		apiGroup.POST("/sessions/:id/transfer/answer", s.TransferAnswer)
		apiGroup.POST("/sessions/:id/transfer/reference", s.TransferReference)
		apiGroup.POST("/sessions/:id/transfer/complete", s.TransferComplete)
		apiGroup.GET("/sessions/:id/summary", s.GetSummary)
		apiGroup.GET("/sessions/:id/export", s.ExportSummary)
		apiGroup.GET("/teacher/overview", s.TeacherOverview)
	}

	s.mountStatic(r)
	return r
}

// mountStatic 在生产模式下托管前端构建产物（web/dist），并支持单页应用回退。
func (s *Server) mountStatic(r *gin.Engine) {
	if s.webDist == "" {
		return
	}
	index := filepath.Join(s.webDist, "index.html")
	if _, err := os.Stat(index); err != nil {
		return
	}
	r.Static("/assets", filepath.Join(s.webDist, "assets"))
	favicon := filepath.Join(s.webDist, "favicon.ico")
	if _, err := os.Stat(favicon); err == nil {
		r.StaticFile("/favicon.ico", favicon)
	}
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			fail(c, http.StatusNotFound, "NOT_FOUND", "接口不存在："+c.Request.URL.Path, "")
			return
		}
		c.File(index)
	})
}

// Health 处理 GET /api/health。
func (s *Server) Health(c *gin.Context) {
	total, _ := s.store.Count()
	c.JSON(200, gin.H{
		"ok":          true,
		"service":     "知行桥服务端",
		"version":     s.version,
		"time":        time.Now().Format(time.RFC3339),
		"uptime":      time.Since(s.startedAt).Round(time.Second).String(),
		"llmEnabled":  s.llm.Available(),
		"llmModel":    s.llmModel,
		"records":     total,
		"runningMode": appMode(s.llm),
	})
}

// Meta 处理 GET /api/meta（产品信息、运行模式、步骤、诊断类别与规则库）。
func (s *Server) Meta(c *gin.Context) {
	archetypes := make([]gin.H, 0, len(content.Archetypes))
	for _, a := range content.Archetypes {
		cats := make([]gin.H, 0, len(a.Categories))
		for _, code := range a.Categories {
			cats = append(cats, gin.H{"code": code, "label": content.CategoryLabel(code)})
		}
		archetypes = append(archetypes, gin.H{
			"code":             a.Code,
			"name":             a.Name,
			"knowledgePoint":   a.KnowledgePoint,
			"categories":       cats,
			"keywords":         a.RawKeywords(),
			"teacherCondition": a.TeacherCondition,
			"direct":           a.DirectTroubleshooting,
		})
	}
	// 覆盖范围：把所有已实现的教学场景列出来，便于使用者（含评委/教师）当场核对能力边界。
	covered := make([]gin.H, 0, len(content.TaskTypes))
	for _, t := range content.TaskTypes {
		if t.Code == content.TaskGeneric {
			continue
		}
		families := len(content.FamiliesByTaskType(t.Code))
		covered = append(covered, gin.H{"code": t.Code, "label": t.Label, "families": families})
	}
	categories := make([]gin.H, 0, len(content.Categories))
	for _, cc := range content.Categories {
		categories = append(categories, gin.H{"code": cc.Code, "label": cc.Label})
	}
	mode := appMode(s.llm)
	label, notice := appModeInfo(s.llm)
	c.JSON(200, gin.H{
		"ok":          true,
		"productName": "知行桥——人工智能基础课程教学衔接智能体",
		"tagline":     "以实验结果为证据，通过诊断、分层提示和迁移检验，帮助学生真正理解人工智能实验。",
		"course":      "《人工智能基础（A）》",
		"focus":       "覆盖《人工智能基础（A）》机器学习实验的核心任务：分类指标、模型结构、训练稳定性、环境排障、回归、聚类与数据预处理；其他课程模块属于后续扩展范围。",
		"mode":        gin.H{"code": mode, "label": label, "notice": notice, "llmConfigured": s.llm.Available(), "llmModel": s.llmModel},
		"steps": []gin.H{
			{"key": "submit", "title": "提交材料"},
			{"key": "diagnosis", "title": "学习诊断"},
			{"key": "tutoring", "title": "分层辅导"},
			{"key": "transfer", "title": "迁移测试"},
			{"key": "report", "title": "学习反馈"},
		},
		"categories": categories,
		"archetypes": archetypes,
		"coverage": gin.H{
			"taskTypes": covered,
			"taskTypeCount": len(covered),
			"ruleCount":     len(content.Archetypes),
			"caseCount":     len(content.DemoCases),
			"note": "覆盖范围仅指内置规则库已实现的教学场景：诊断、三级提示、迁移题与修正反馈都限定在对应任务类型的知识范围内；未覆盖的课程模块不会被声称为已支持。",
		},
		"honesty": []string{
			"系统区分真实计算结果、学生输入与模拟演示数据，所有指标均由程序计算。",
			"系统不会声称运行过学生代码，也不会假设学生未提供的实验结果。",
			"未配置大模型 API Key 时，应用使用内置规则模板运行，功能不缺失。",
			"评分仅为规则化反馈，不代表正式考试成绩或学习成效测量。",
			"学习记录保存在本机，不收集姓名、学号等非必要个人信息。",
		},
		"storage": gin.H{"location": "服务端 SQLite（默认 data/knowbr.db）", "note": "记录用于演示与教学回顾，可随时删除。"},
	})
}

// DemoCases 处理 GET /api/demo-cases。
func (s *Server) DemoCases(c *gin.Context) {
	c.JSON(200, gin.H{"ok": true, "cases": content.DemoCases, "notice": content.DemoDataNotice})
}

// TeacherOverview 处理 GET /api/teacher/overview。
func (s *Server) TeacherOverview(c *gin.Context) {
	list, err := s.store.List(0)
	if err != nil {
		fail(c, 500, "INTERNAL", "读取记录失败："+err.Error(), "")
		return
	}
	records := make([]gin.H, 0, len(list))
	for i := range list {
		records = append(records, gin.H{
			"record":       s.recordDTO(&list[i]),
			"mainIssue":    list[i].MainIssue,
			"hintLevels":   hintLevels(list[i]),
			"summary":      list[i].TeacherSummary,
			"intervention": list[i].TeacherIntervention,
		})
	}
	real, demo := countDemo(list)
	note := "以下统计仅基于本机已保存的学习记录，不是真实班级或学业成绩统计。"
	if demo > 0 {
		note = note + " 其中 " + itoa(demo) + " 条来自预置模拟案例，仅用于演示。"
	}
	c.JSON(200, gin.H{
		"ok":           true,
		"records":      records,
		"commonIssues": commonIssues(list),
		"note":         note,
		"counts":       gin.H{"total": len(list), "real": real, "demo": demo},
	})
}

func hintLevels(s models.Session) []int {
	state, ok := decodeJSON[models.HintState](s.HintStateJSON)
	if !ok {
		return []int{}
	}
	if state.UsedLevels == nil {
		return []int{}
	}
	return state.UsedLevels
}

// appMode 返回首页/健康检查展示的当前应用运行模式。
func appMode(llm *services.LLMClient) string {
	if llm != nil && llm.Available() {
		return ModeLLM
	}
	return ModeRule
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Accept")
			c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func ctxOrBackground() context.Context { return context.Background() }

func isValidationError(err error) bool {
	var ve *services.ValidationError
	return errors.As(err, &ve)
}
