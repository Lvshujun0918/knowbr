package models

import (
	"encoding/json"
	"time"
)

// 会话状态流转。
const (
	StatusDiagnosed = "diagnosed" // 已完成材料完整性检查与诊断
	StatusTutoring  = "tutoring"  // 正在分层辅导
	StatusTransfer  = "transfer"  // 已进入迁移测试
	StatusCompleted = "completed" // 已完成迁移测试与反馈
)

// Submission 是学生提交的实验材料。混淆矩阵可以按表格填写（ConfusionMatrix），
// 也可以粘贴文本（ConfusionMatrixText），两者同时存在时以表格为准。
type Submission struct {
	ExperimentTitle           string      `json:"experimentTitle"`
	ExperimentGoal            string      `json:"experimentGoal"`
	DatasetDescription        string      `json:"datasetDescription"`
	ModelName                 string      `json:"modelName"`
	CodeSnippet               string      `json:"codeSnippet"`
	ExecutionLog              string      `json:"executionLog"`
	ConfusionMatrix           [][]float64 `json:"confusionMatrix"`
	ConfusionMatrixText       string      `json:"confusionMatrixText"`
	ClassLabels               []string    `json:"classLabels"`
	MetricsText               string      `json:"metricsText"`
	StudentInitialExplanation string      `json:"studentInitialExplanation"`
	Question                  string      `json:"question"`
	DemoCaseID                string      `json:"demoCaseId"`
	UseLLM                    *bool       `json:"useLLM"`
}

// Hint 是一条已提供的分层提示记录。
type Hint struct {
	Level              int    `json:"level"`
	Content            string `json:"content"`
	CreatedAt          string `json:"createdAt"`
	ArchetypeCode      string `json:"archetypeCode,omitempty"`
	Socratic           bool   `json:"socratic"`
	RequestedByStudent bool   `json:"requestedByStudent"`
}

// Revision 是学生提交的一次修正解释及其反馈。
type Revision struct {
	Content           string   `json:"content"`
	CreatedAt         string   `json:"createdAt"`
	Feedback          string   `json:"feedback"`
	Progress          string   `json:"progress"` // improved | partial | none
	Points            []string `json:"points"`
	AddressedKeywords []string `json:"addressedKeywords"`
	MissingKeywords   []string `json:"missingKeywords"`
}

// HintState 记录提示等级推进状态与教师介入判断。
type HintState struct {
	NextLevel           int    `json:"nextLevel"`
	MaxLevel            int    `json:"maxLevel"`
	UsedLevels          []int  `json:"usedLevels"`
	NoProgressRounds    int    `json:"noProgressRounds"`
	TeacherIntervention bool   `json:"teacherIntervention"`
	TeacherReason       string `json:"teacherReason"`
}

// Session 是一次完整学习过程的持久化记录。
// 复合结构以 JSON 文本列保存，便于在 SQLite 中直接查看与迁移。
type Session struct {
	ID        string    `gorm:"primaryKey;size:40" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Status            string `gorm:"size:20;index" json:"status"`
	Mode              string `gorm:"size:10" json:"mode"` // demo | rule | llm
	LLMUsed           bool   `json:"llmUsed"`
	LLMFallbackReason string `gorm:"size:255" json:"llmFallbackReason"`
	IsDemo            bool   `json:"isDemo"`
	DemoCaseID        string `gorm:"size:40" json:"demoCaseId"`

	ExperimentTitle    string `gorm:"size:200" json:"experimentTitle"`
	ExperimentGoal     string `gorm:"type:text" json:"experimentGoal"`
	DatasetDescription string `gorm:"type:text" json:"datasetDescription"`
	ModelName          string `gorm:"size:200" json:"modelName"`
	CodeSnippet        string `gorm:"type:text" json:"codeSnippet"`
	ExecutionLog       string `gorm:"type:text" json:"executionLog"`
	ClassLabelsJSON    string `gorm:"type:text" json:"-"`
	MatrixJSON         string `gorm:"type:text" json:"-"`
	MatrixRaw          string `gorm:"type:text" json:"-"`
	MetricsText        string `gorm:"type:text" json:"metricsText"`

	StudentInitialExplanation string `gorm:"type:text" json:"studentInitialExplanation"`
	StudentQuestion           string `gorm:"type:text" json:"question"`

	DiagnosisJSON string `gorm:"type:text" json:"-"`
	MetricsJSON   string `gorm:"type:text" json:"-"`
	HintsJSON     string `gorm:"type:text" json:"-"`
	RevisionsJSON string `gorm:"type:text" json:"-"`
	TransferJSON  string `gorm:"type:text" json:"-"`
	SummaryJSON   string `gorm:"type:text" json:"-"`
	HintStateJSON string `gorm:"type:text" json:"-"`

	MainIssue           string `gorm:"size:255" json:"mainIssue"`
	CategoriesCSV       string `gorm:"size:255" json:"-"`
	PrimaryArchetype    string `gorm:"size:10" json:"primaryArchetype"`
	TransferScore       int    `json:"transferScore"` // -1 表示尚未评分
	TeacherIntervention bool   `json:"teacherIntervention"`
	TeacherSummary      string `gorm:"type:text" json:"teacherSummary"`
}

// ExperimentDTO 是提交材料在接口返回中的结构。
type ExperimentDTO struct {
	ExperimentTitle    string   `json:"experimentTitle"`
	ExperimentGoal     string   `json:"experimentGoal"`
	DatasetDescription string   `json:"datasetDescription"`
	ModelName          string   `json:"modelName"`
	CodeSnippet        string   `json:"codeSnippet"`
	ExecutionLog       string   `json:"executionLog"`
	ClassLabels        []string `json:"classLabels"`
	MetricsText        string   `json:"metricsText"`
	InitialExplanation string   `json:"studentInitialExplanation"`
	Question           string   `json:"question"`
}

// SessionDTO 是返回给前端的完整会话结构。
type SessionDTO struct {
	ID                string          `json:"id"`
	CreatedAt         string          `json:"createdAt"`
	UpdatedAt         string          `json:"updatedAt"`
	Status            string          `json:"status"`
	Mode              string          `json:"mode"`
	ModeLabel         string          `json:"modeLabel"`
	ModeNotice        string          `json:"modeNotice"`
	LLMUsed           bool            `json:"llmUsed"`
	LLMFallbackReason string          `json:"llmFallbackReason,omitempty"`
	IsDemo            bool            `json:"isDemo"`
	DemoCaseID        string          `json:"demoCaseId,omitempty"`
	DataNotice        string          `json:"dataNotice,omitempty"`
	Experiment        ExperimentDTO   `json:"experiment"`
	Matrix            json.RawMessage `json:"matrix,omitempty"`
	Metrics           json.RawMessage `json:"metrics,omitempty"`
	Diagnosis         json.RawMessage `json:"diagnosis,omitempty"`
	Hints             []Hint          `json:"hints"`
	HintState         HintState       `json:"hintState"`
	Revisions         []Revision      `json:"revisions"`
	Transfer          json.RawMessage `json:"transfer,omitempty"`
	Summary           json.RawMessage `json:"summary,omitempty"`
	TeacherSummary    string          `json:"teacherSummary"`
	MainIssue         string          `json:"mainIssue"`
	Categories        []string        `json:"categories"`
	TransferScore     int             `json:"transferScore"`
}

// RecordDTO 是学习记录列表中的一行。
type RecordDTO struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
	Status              string   `json:"status"`
	StatusLabel         string   `json:"statusLabel"`
	Mode                string   `json:"mode"`
	ModeLabel           string   `json:"modeLabel"`
	LLMUsed             bool     `json:"llmUsed"`
	IsDemo              bool     `json:"isDemo"`
	MainIssue           string   `json:"mainIssue"`
	Categories          []string `json:"categories"`
	CategoryLabels      []string `json:"categoryLabels"`
	PrimaryArchetype    string   `json:"primaryArchetype"`
	HintLevelsUsed      []int    `json:"hintLevelsUsed"`
	RevisionImproved    bool     `json:"revisionImproved"`
	RevisionCount       int      `json:"revisionCount"`
	TransferScore       int      `json:"transferScore"`
	TransferMaxScore    int      `json:"transferMaxScore"`
	TeacherIntervention bool     `json:"teacherIntervention"`
}
