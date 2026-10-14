# 知行桥 接口契约（前后端冻结版 v1）

后端：Go + Gin + GORM(SQLite)，默认 `http://127.0.0.1:8080`。
前端开发服务器通过 Vite 代理 `/api` 到 `http://127.0.0.1:8080`。
所有响应均为 JSON，成功为 `{"ok":true,...}`，失败为：

```json
{ "ok": false, "error": { "code": "MATRIX_NOT_SQUARE", "field": "confusionMatrix", "message": "中文可读提示", "detail": "" } }
```

**列表字段约定**：所有数组字段（如 `evidence`、`matchedArchetypes`、`categories`、`hints`、`revisions`、`hintState.usedLevels`、`rubric`、`rubricFeedback`、`missing`、`limitations`、`keyEvidence`、`studentNextSteps`、`teacherNotes`）在为空时返回 `[]`，**不会返回 `null`**；对象字段（`matrix`、`metrics`、`diagnosis`、`transfer`、`summary`）未产生时**整个字段省略**。前端可直接遍历数组。

前端必须：捕获错误 → 显示 `error.message` → **保留用户已填写的全部表单内容**，不得清空。

## 1. 元信息

### GET /api/health
`{ ok, service, version, time, uptime, llmEnabled, llmModel, records, runningMode }`

### GET /api/meta
```json
{
  "ok": true,
  "productName": "知行桥——人工智能基础课程教学衔接智能体",
  "tagline": "以实验结果为证据，通过诊断、分层提示和迁移检验，帮助学生真正理解人工智能实验。",
  "course": "《人工智能基础（A）》",
  "focus": "首期聚焦“机器学习分类实验”，其他课程模块属于后续扩展范围。",
  "mode": { "code": "rule|llm", "label": "规则模板模式", "notice": "……", "llmConfigured": false, "llmModel": "" },
  "steps": [ {"key":"submit","title":"提交材料"}, {"key":"diagnosis","title":"学习诊断"}, {"key":"tutoring","title":"分层辅导"}, {"key":"transfer","title":"迁移测试"}, {"key":"report","title":"学习反馈"} ],
  "categories": [ {"code":"environment|concept|design|metric_selection|interpretation|boundary|insufficient","label":"中文名"} ],
  "archetypes": [ {"code":"E0..E5","name":"…","knowledgePoint":"…","categories":[{"code","label"}],"keywords":["…"],"teacherCondition":"…","direct":false} ],
  "honesty": ["…五条诚实声明…"],
  "storage": {"location":"服务端 SQLite（默认 data/knowbr.db）","note":"…"}
}
```

### GET /api/demo-cases
`{ ok, notice, cases: [ { id, name, description, dataNotice, expectedFocus: [..], submission: DemoSubmission } ] }`

`DemoSubmission` 字段与 `Submission` 完全一致（见下），可直接填入表单。

### POST /api/metrics/compute
请求：`{ "confusionMatrix": [[900,0,0],[90,0,0],[10,0,0]] }` 或 `{ "confusionMatrixText": "900 0 0\n90 0 0\n10 0 0", "classLabels": ["类别A","类别B","类别C"] }`
成功：`{ ok:true, metrics: MatrixMetrics, labels: [...] }`；失败：400 + 校验错误（非数字 / 负数 / 行列不一致 / 全 0）。

**MatrixMetrics**
```json
{
  "labels": ["类别A","类别B","类别C"],
  "values": [[900,0,0],[90,0,0],[10,0,0]],
  "total": 1000, "correct": 900, "accuracy": 0.9,
  "classes": [{ "index":0,"label":"类别A","support":900,"predicted":900,"truePositive":900,"falsePositive":0,"falseNegative":0,
                "recall":1,"recallUndefined":false,"precision":1,"precisionUndefined":false,"f1":1 }],
  "macroRecall": 0.3333, "macroPrecision": 0.3333, "macroF1": 0.3333, "weightedF1": 0.9,
  "maxClassRatio": 0.9, "imbalanceRatio": 90, "imbalanced": true,
  "notes": ["…零分母约定说明…"], "zeroRecallClasses": ["类别B","类别C"]
}
```
零分母约定：某类没有预测样本 → `precision=0` 且 `precisionUndefined=true`；某类真实样本为 0 → `recall=0` 且 `recallUndefined=true`。界面必须原样展示 `notes` 并标注“由程序计算”。

## 2. 学习会话

### POST /api/sessions
请求体（`Submission`，全部字段可选，除 `studentInitialExplanation` 必填）：
```json
{
  "experimentTitle": "分类模型评价实验",
  "experimentGoal": "", "datasetDescription": "", "modelName": "",
  "codeSnippet": "", "executionLog": "",
  "confusionMatrix": [[900,0,0],[90,0,0],[10,0,0]],
  "confusionMatrixText": "900 0 0\n90 0 0\n10 0 0",
  "classLabels": ["类别A","类别B","类别C"],
  "metricsText": "", "studentInitialExplanation": "必填", "question": "",
  "demoCaseId": "", "useLLM": true
}
```
`confusionMatrix`（表格）优先于 `confusionMatrixText`（粘贴文本）。
成功：`{ ok:true, session: Session }`；解释为空 → 400 `EXPLANATION_REQUIRED`；矩阵非法 → 400（见上）。

### Session 结构
```json
{
  "id": "uuid", "createdAt": "RFC3339", "updatedAt": "RFC3339",
  "status": "diagnosed|tutoring|transfer|completed",
  "mode": "demo|rule|llm", "modeLabel": "规则模板模式", "modeNotice": "诚实说明",
  "llmUsed": false, "llmFallbackReason": "", "isDemo": false, "demoCaseId": "", "dataNotice": "模拟案例…",
  "experiment": { "experimentTitle","experimentGoal","datasetDescription","modelName","codeSnippet","executionLog","classLabels","metricsText","studentInitialExplanation","question" },
  "matrix": [[900,0,0],[90,0,0],[10,0,0]],
  "metrics": MatrixMetrics | null,
  "diagnosis": Diagnosis,
  "hints": [ {"level":1,"content":"…","createdAt":"…","archetypeCode":"E1","socratic":true,"requestedByStudent":false} ],
  "hintState": { "nextLevel":1,"maxLevel":3,"usedLevels":[],"noProgressRounds":0,"teacherIntervention":false,"teacherReason":"" },
  "revisions": [ {"content":"…","createdAt":"…","feedback":"…","progress":"improved|partial|none","points":["…"],"addressedKeywords":["…"],"missingKeywords":["…"]} ],
  "transfer": TransferState | null,
  "summary": Summary | null,
  "teacherSummary": "文本", "mainIssue": "…", "categories": ["E1 对应的类别 code"], "transferScore": -1
}
```

**Diagnosis**
```json
{
  "categories": [{"code":"interpretation","label":"实验结果解释问题"}],
  "primaryCategory": "interpretation", "primaryCategoryName": "实验结果解释问题",
  "mainIssue": "一句话主要问题",
  "evidence": ["由你提交的矩阵计算：总样本 1000 个…", "学生原话：…"],
  "evidenceSource": ["混淆矩阵计算","学生输入"],
  "reasoning": "诊断理由",
  "masteryStatus": "demonstrated|partial|not_demonstrated|insufficient_evidence",
  "masteryLabel": "尚未体现",
  "nextStep": "建议下一步",
  "matchedArchetypes": [{"code":"E1","name":"…","knowledgePoint":"…"}],
  "primaryArchetype": "E1 | NONE",
  "contradiction": {"detected":false,"claim":"","detail":""},
  "teacherIntervention": false, "teacherReason": "",
  "completeness": {"sufficient":true,"haveExplanation":true,"haveMatrix":true,"havePerClassMetrics":true,"haveDistribution":true,"haveTaskGoal":false,"haveRunEvidence":false,"missing":["…"],"limitations":["…"]},
  "matrixFacts": ["…"],
  "generatedAt":"RFC3339","llmUsed":false,"llmNote":"","source":"rule|llm"
}
```

### GET /api/sessions
`{ ok, records:[RecordDTO], storage:{location,total,demoRecords,realRecords,note} }`

RecordDTO：`{ id, title, createdAt, updatedAt, status, statusLabel, mode, modeLabel, llmUsed, isDemo, mainIssue, categories[], categoryLabels[], primaryArchetype, hintLevelsUsed[], revisionImproved, revisionCount, transferScore, transferMaxScore, teacherIntervention }`

### GET /api/sessions/:id → `{ ok, session: Session }`
### DELETE /api/sessions/:id → `{ ok:true }`
### DELETE /api/sessions → `{ ok:true, deleted: n }`

### POST /api/sessions/:id/hints
请求 `{ "level": 1|2|3, "requestedByStudent": true }`
成功 `{ ok:true, hint:{...}, hints:[...], hintState:{...}, alreadyProvided:false }`
等级非法 → 400 `HINT_LEVEL_INVALID`。

### POST /api/sessions/:id/revisions
请求 `{ "content": "学生修正后的解释" }`；空内容 → 400 `REVISION_EMPTY`。
成功 `{ ok:true, revision:{...}, revisions:[...], hintState:{...}, improved:false }`

### 迁移测试
- `POST /api/sessions/:id/transfer/question` → `{ ok:true, transfer: TransferState, reused:false }`（题目生成一次，重复调用返回同一题）
- `POST /api/sessions/:id/transfer/answer` `{ "answer":"…" }` → `{ ok:true, transfer: TransferState }`（此时 `referenceAnswer` 仍为空）
- `POST /api/sessions/:id/transfer/reference` → `{ ok:true, transfer }`（未作答时 400 `ANSWER_REQUIRED`；作答后返回 `referenceAnswer` 与 `expectedPoints`）
- `POST /api/sessions/:id/transfer/complete` → `{ ok:true, transfer, summary, markdown, session }`

**TransferState**
```json
{
  "itemId":"T1","topic":"…","context":"…","question":"…","maxScore":6,
  "rubric":[{"code":"goal","title":"识别任务目标与数据分布（0—2 分）","maxScore":2}],
  "answer":"","answered":false,"score":0,"scoreLabel":"0 / 6",
  "rubricFeedback":[{"code","title","score","maxScore","reason","matchedKeywords":[]}],
  "referenceAnswer":"","expectedPoints":[],"scoredAt":"","completed":false,
  "disclaimer":"评分由内置规则…","matchedKeywords":[],"missingPoints":[]
}
```
`rubric` 与 `disclaimer` 在作答前也要展示；`referenceAnswer`/`expectedPoints` 只能在作答后出现。

### GET /api/sessions/:id/summary
`{ ok, summary: Summary, markdown: "Markdown 文本", teacherSummary: "文本" }`

**Summary**：`{ generatedAt, learningGoal, initialExplanation, keyEvidence[], mainIssue, issueEvidence[], diagnosisReasoning, categories[], hintLevels[], hintTrail[], revisionSummary, revisionImproved, revisionCount, transfer:{itemId,topic,question,answer,score,maxScore,scored,completed,rubricFeedback[],matchedKeywords[],missingPoints[],referenceAnswer}, mastery:{status,label}, studentNextSteps[], teacherNotes[], teacherSummary, mode, modeLabel, llmUsed, dataNotice, disclaimer, teacherIntervention }`

### GET /api/sessions/:id/export?format=md|txt&download=1
返回 `text/markdown` 或 `text/plain`（前端可用 `?download=1` 直接触发下载，或取文本后自行复制）。

### GET /api/teacher/overview
`{ ok, records:[{record:RecordDTO, mainIssue, hintLevels[], summary, intervention}], commonIssues:[{code,label,count,recordIds[]}], note, counts:{total,real,demo} }`
统计只来自本机保存的记录，界面必须同时展示 `note`。

## 3. 前端路由约定（建议）

| 路由 | 页面 |
|---|---|
| `/` | 首页：产品定位、五步流程、入口按钮（开始学习诊断 / 体验演示案例 / 学习记录·教师反馈 / 使用说明）、运行模式徽标、聚焦范围说明 |
| `/submit` | 提交实验材料：10 个字段表单 + 加载演示案例 + 清空 + 开始诊断 |
| `/session/:id` | 单页多步骤流程：诊断结果 → 分层辅导 → 迁移测试 → 学习反馈（步骤指示器固定展示“提交材料 → 学习诊断 → 分层辅导 → 迁移测试 → 学习反馈”） |
| `/records` | 学习记录列表（查看详情 / 删除 / 清空） |
| `/teacher` | 教师视图：记录摘要、共性问题、教师关注点 |
| `/help` | 使用说明：如何加载预置案例、如何完成一轮流程、已实现与未实现功能、已知限制 |
