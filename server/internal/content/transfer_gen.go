package content

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ScenarioParam 是题目模板中的一个可替换槽位。槽位之外的文字保持稳定，
// 槽位取值变化用于生成同一考查能力、不同数据条件的新题。
type ScenarioParam struct {
	Slot    string
	Label   string
	Options []string
}

// ScenarioVariant 是某个任务类型下的一道题目模板。
// Context 与 Question 中的 {{槽位名}} 会在生成时被 ScenarioParam 的取值替换。
type ScenarioVariant struct {
	Code              string
	Topic             string
	KnowledgePoint    string
	Archetypes        []string
	Scenario          string // 情境标签，用于与示例情境比对
	ExampleScenario   string // 非空表示沿用了演示/提示中出现过的示例情境，生成时降权
	Context           string
	Question          string
	RequiredData      []string
	AnswerRequirement string
	Rubric            []RubricDim
	ExpectedPoints    []string
	ReferenceAnswer   string
}

// ScenarioFamily 是一个任务类型下的题目族。
type ScenarioFamily struct {
	Code     string
	TaskType string
	Label    string
	Params   []ScenarioParam
	Variants []ScenarioVariant
}

// TransferFamilies 汇总全部题目族，由各族文件在 init 中注册。
var TransferFamilies []ScenarioFamily

func registerFamily(f ScenarioFamily) {
	for _, existing := range TransferFamilies {
		if existing.Code == f.Code {
			return
		}
	}
	TransferFamilies = append(TransferFamilies, f)
}

// FamiliesByTaskType 返回某任务类型的题目族，按编号排序。
func FamiliesByTaskType(taskType string) []ScenarioFamily {
	out := []ScenarioFamily{}
	for _, f := range TransferFamilies {
		if f.TaskType == taskType {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// ExampleScenarioTexts 是演示案例与提示文本中已经出现过的情境。
// 生成迁移题时用于相似度检查，避免把示例改个标题当成新题。
var ExampleScenarioTexts = []string{
	"疾病筛查：阳性样本约占百分之一，漏诊的代价明显高于一次误报。",
	"三分类任务：类别 A 九百个、类别 B 九十个、类别 C 十个，模型把全部样本预测为类别 A。",
	"垃圾分类：有害垃圾一百二十个样本，召回率百分之三十五，大部分被预测为其他。",
	"未知样本占比百分之九十，全部判为零风险，漏报率达到百分之百。",
	"总样本一千个，其中某类只有十个，模型把这类样本全部预测成另一类。",
	"把实验代码复制到只有六十条数据、异常类只有三条的新数据上，代码同样运行成功。",
}

// GeneratedItemID 由题目族、变体与参数组合共同决定，因此题目可以按编号精确重建。
func GeneratedItemID(familyCode, variantCode string, paramIndex int) string {
	return fmt.Sprintf("%s:%s:%d", familyCode, variantCode, paramIndex)
}

// SplitGeneratedItemID 解析生成题编号。
func SplitGeneratedItemID(id string) (familyCode, variantCode string, paramIndex int, ok bool) {
	parts := strings.Split(id, ":")
	if len(parts) != 3 {
		return "", "", 0, false
	}
	n, err := strconv.Atoi(parts[2])
	if err != nil || n < 0 {
		return "", "", 0, false
	}
	return parts[0], parts[1], n, true
}

// ParamCombinationCount 返回题目族的参数组合总数（至少为 1）。
func ParamCombinationCount(f ScenarioFamily) int {
	total := 1
	for _, p := range f.Params {
		if len(p.Options) == 0 {
			continue
		}
		total *= len(p.Options)
	}
	return total
}

// ParamValues 把线性参数序号还原为各槽位的取值。
func ParamValues(f ScenarioFamily, index int) map[string]string {
	out := map[string]string{}
	total := ParamCombinationCount(f)
	if total <= 0 {
		return out
	}
	index = ((index % total) + total) % total
	rest := index
	for _, p := range f.Params {
		if len(p.Options) == 0 {
			continue
		}
		out[p.Slot] = p.Options[rest%len(p.Options)]
		rest /= len(p.Options)
	}
	return out
}

func fillSlots(text string, params map[string]string) string {
	out := text
	for slot, value := range params {
		out = strings.ReplaceAll(out, "{{"+slot+"}}", value)
	}
	return out
}

// BuildVariantItem 按题目族、变体与参数序号构建一道具体的迁移题。
// 同样的输入永远得到同样的题目，因此题目可以按编号重建，而参考答案与关键词不会存入学生记录。
func BuildVariantItem(f ScenarioFamily, variantIndex, paramIndex int) (TransferItem, bool) {
	if variantIndex < 0 || variantIndex >= len(f.Variants) {
		return TransferItem{}, false
	}
	v := f.Variants[variantIndex]
	params := ParamValues(f, paramIndex)
	rubric := make([]RubricDim, len(v.Rubric))
	for i, dim := range v.Rubric {
		rubric[i] = dim
		rubric[i].Title = fillSlots(dim.Title, params)
		rubric[i].FullHitHint = fillSlots(dim.FullHitHint, params)
		rubric[i].PartialHint = fillSlots(dim.PartialHint, params)
		rubric[i].MissHint = fillSlots(dim.MissHint, params)
	}
	maxScore := 0
	for _, dim := range rubric {
		maxScore += dim.MaxScore
	}
	item := TransferItem{
		ID:                GeneratedItemID(f.Code, v.Code, paramIndex),
		Topic:             fillSlots(v.Topic, params),
		Scenario:          fillSlots(v.Scenario, params),
		KnowledgePoint:    v.KnowledgePoint,
		TaskType:          f.TaskType,
		Archetypes:        append([]string(nil), v.Archetypes...),
		ExampleScenario:   v.ExampleScenario,
		Context:           fillSlots(v.Context, params),
		Question:          fillSlots(v.Question, params),
		RequiredData:      fillSlots0(v.RequiredData, params),
		AnswerRequirement: fillSlots(v.AnswerRequirement, params),
		MaxScore:          maxScore,
		Rubric:            rubric,
		ExpectedPoints:    fillSlots0(v.ExpectedPoints, params),
		ReferenceAnswer:   fillSlots(v.ReferenceAnswer, params),
		Source:            SourceRuleTemplate,
		VariantCode:       v.Code,
		FamilyCode:        f.Code,
		FamilyLabel:       f.Label,
		ParamsUsed:        params,
	}
	return item, true
}

func fillSlots0(list []string, params map[string]string) []string {
	if len(list) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, fillSlots(s, params))
	}
	return out
}

// TransferItemSource 常量：题目来源类型（需求文档 4.3 / 5 节要求标注）。
const (
	SourceRuleTemplate = "rule_template" // 规则模板按当前任务与学生薄弱点组合生成
	SourceFixedBank    = "fixed_bank"    // 内置固定题库
)

// ResolveTransferItem 按编号重建迁移题：优先固定题库，其次按生成规则重建。
func ResolveTransferItem(id string) (TransferItem, bool) {
	if item, ok := TransferItemByID(id); ok {
		return item, true
	}
	familyCode, variantCode, paramIndex, ok := SplitGeneratedItemID(id)
	if !ok {
		return TransferItem{}, false
	}
	for _, f := range TransferFamilies {
		if f.Code != familyCode {
			continue
		}
		for i, v := range f.Variants {
			if v.Code != variantCode {
				continue
			}
			return BuildVariantItem(f, i, paramIndex)
		}
	}
	return TransferItem{}, false
}

// Similarity 返回两段文本的字符二元组 Jaccard 相似度（0—1）。
// 用于检查新题是否与示例题或前序题目高度重复。
func Similarity(a, b string) float64 {
	sa := shingles(a)
	sb := shingles(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	inter := 0
	for k := range sa {
		if sb[k] {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// SimilarityThreshold 是判定“高度重复”的阈值。
const SimilarityThreshold = 0.45

func shingles(s string) map[string]bool {
	out := map[string]bool{}
	var buf []rune
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '，' || r == '。' || r == '：' || r == '；' ||
			r == '、' || r == '（' || r == '）' || r == '“' || r == '”' || r == '？' || r == '！' {
			continue
		}
		buf = append(buf, r)
	}
	if len(buf) < 2 {
		if len(buf) == 1 {
			out[string(buf)] = true
		}
		return out
	}
	for i := 0; i+1 < len(buf); i++ {
		out[string(buf[i:i+2])] = true
	}
	return out
}
