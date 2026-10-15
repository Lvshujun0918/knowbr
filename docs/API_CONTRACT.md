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
  "hints": [ {"level":1,"content":"…","createdAt":"…","archetypeCode":"E1","socratic":true,"requestedByStudent":false,"why":"为什么选择这一级提示","targetKnowledge":"本轮针对的知识点","addressesClaim":"C1"} ],
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
  "taskType": "classification_metrics|model_architecture|training_stability|code_environment|regression_metrics|clustering_metrics|data_preparation|generic",
  "taskTypeLabel": "训练稳定性与优化",
  "taskFocus": ["本任务优先讨论的知识点"],
  "taskOffTopic": ["本任务不应要求的其它任务知识点"],
  "categories": [{"code":"interpretation","label":"实验结果解释问题"}],
  "primaryCategory": "interpretation", "primaryCategoryName": "实验结果解释问题",
  "mainIssue": "一句话主要问题",
  "evidence": ["由你提交的矩阵计算：总样本 1000 个…", "学生原话：…", "运行日志：epoch 4 train_loss=6.84 grad_norm=95.3"],
  "evidenceSource": ["混淆矩阵计算","学生输入","运行日志"],
  "evidenceNeeds": [{"name":"训练日志（含 loss 与梯度范数等数值）","required":true,"present":true,"impact":"缺失时限制的结论","note":"已提供，可以支撑当前任务的结论。"}],
  "claims": [{"id":"C1","quote":"学生原话","knowledgePoint":"假设与结论的区分","taskType":"training_stability","status":"mastered|partial|error|insufficient_evidence|not_mentioned|not_applicable","statusLabel":"部分掌握","judgment":"判断","reason":"原因","evidence":"学生原话：…","evidenceBoundary":"证据边界","nextQuestion":"下一步追问","signals":["TR-HYPOTHESIS"]}],
  "knowledge": [{"code":"K1","name":"单变量对照实验设计","status":"not_mentioned","statusLabel":"未涉及","note":"未涉及不等于未掌握…","required":true,"applicable":true}],
  "primaryClaimId": "C1",
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

约定：

- `taskType` 由提交材料判定（标题/目标、运行日志、代码、解释、数据描述加权），诊断、提示、迁移题与修正反馈都限定在该任务类型的知识范围内：训练稳定性任务不会要求混淆矩阵或分类指标；回归任务（`regression_metrics`：误差量纲、泛化估计、基线对照）不会要求混淆矩阵、逐类召回率或类别不平衡；聚类任务（`clustering_metrics`：无监督任务的评价方式、簇数与业务含义、标准化与稳定性）不会要求混淆矩阵、逐类指标或回归误差指标；预处理任务（`data_preparation`：预处理与划分的顺序、特征在预测时刻的可得性、评估流程一致性）不会要求混淆矩阵、逐类指标、聚类内部指标或回归误差指标。
- `claims` 是学生的逐条主张，每条都带证据、判断、原因、证据边界与下一步追问；`knowledge` 使用六种状态，`not_mentioned` 不会被计入缺陷。
- `evidenceNeeds` 区分必需与可选材料，缺失时 `impact` 说明限制了哪些结论。

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

### 智能体决策

- `GET /api/sessions/:id/agent` → `{ ok:true, agent: AgentStep }`（只读，不产生任何副作用）
- `POST /api/sessions/:id/agent/step` → 先决策再执行可执行的动作：
  `{ ok:true, agent: AgentStep, executed:true|false, executedNote:"中文说明", hint?:Hint, hints?:[...], hintState?:{...}, session: Session }`

**AgentStep**
```json
{
  "action": "ask_evidence|give_hint|verify_revision|advance_transfer|escalate|complete",
  "actionLabel": "给出下一级提示", "title": "一句话说明这一步做什么",
  "reason": "为什么这样决定（引用证据状态、提示/修正记录与记忆）",
  "requires": ["需要学生补充的必需材料"],
  "suggestHintLevel": 1, "blocked": false, "blockedReason": "", "canAct": true,
  "plan": [{"step":"submit","title":"提交实验材料","status":"done|current|pending","note":""}],
  "tools": [{"name":"evidence_check","purpose":"工具用途","result":"工具返回的关键结果"}],
  "memory": {"lookedAt":2,"repeated":[{"code":"E1","label":"…","count":2,"advice":"…"}],"mastered":[],"milestones":[],"note":"记忆来源局限说明"},
  "decidedAt": "RFC3339", "policyNote": "本步骤由确定性策略引擎计算得出…"
}
```

约定：

- `action` 由策略引擎按优先级选定：迁移题已作答→`complete`；已出题未作答→`advance_transfer`；**必需证据缺失且尚未使用提示→`ask_evidence`（`blocked=true`、`canAct=false`、`suggestHintLevel=0`，拒绝在证据不足时给出具体计算示例）**；连续无进展≥2 轮→`escalate`；仍有未解决主张→`give_hint`；否则等待修正解释。
- `give_hint` 时 `POST /agent/step` 会按 `suggestHintLevel` **自动**生成提示（`requestedByStudent=false`）；`escalate` 时写入教师介入标记，重复调用不重复记录；其余动作不改变会话状态，仅返回决策。
- `tools` 是真实执行的工具轨迹（证据完备性、混淆矩阵指标、提示与修正进度、本机历史查询），不是装饰性文案；未提交矩阵时指标工具明确返回“未计算”。
- `memory` 来自本机历史记录，用于识别重复误区与已掌握知识点；本项目无账号体系，**无法确认历史记录是否属于同一名学生**，`memory.note` 会如实说明这一点，前端必须原样展示。

### POST /api/sessions/:id/revisions
请求 `{ "content": "学生修正后的解释" }`；空内容 → 400 `REVISION_EMPTY`。
成功 `{ ok:true, revision:{...}, revisions:[...], hintState:{...}, improved:false }`

### 迁移测试
- `POST /api/sessions/:id/transfer/question` → `{ ok:true, transfer: TransferState, reused:false }`（题目生成一次，重复调用返回同一题）
- `POST /api/sessions/:id/transfer/answer` `{ "answer":"…" }` → `{ ok:true, transfer: TransferState }`（此时 `referenceAnswer` 仍为空）
- `POST /api/sessions/:id/transfer/reference` → `{ ok:true, transfer }`（未作答时 400 `ANSWER_REQUIRED`；作答后返回 `referenceAnswer` 与 `expectedPoints`）
- `POST /api/sessions/:id/transfer/complete` → `{ ok:true, transfer, summary, markdown, session }`

题目生成约定（规则模板组合生成，未调用大模型）：

1. 先按当前实验的 `taskType` 选定题目族（分类指标 / 模型结构 / 训练稳定性 / 环境排障）；
2. 在题目族内按学生命中的问题类型（`matchedArchetypes`）打分选变体，已掌握的知识点会被降权；
3. 变体的情境与数据条件由模板参数决定，因此不同会话会得到不同情境与不同数据；
4. 生成后与演示案例、提示示例以及本机保存的历史题目做字符二元组相似度检查（阈值 0.45），与示例情境相同的固定题会被降权；
5. `itemId` 为固定题库编号（`T1`—`T3`）或生成编号（`<题目族>:<变体>:<参数序号>`），后者可按编号确定性重建，因此关键词与参考答案不进入学生记录。

**TransferState**
```json
{
  "itemId":"TRN:TRN-4:2","topic":"…","scenario":"情境标签","knowledgePoint":"考查的知识点",
  "taskType":"training_stability","taskTypeLabel":"训练稳定性与优化",
  "context":"…","question":"…","requiredData":["题干已提供的数据"],"answerRequirement":"作答要求","maxScore":6,
  "rubric":[{"code":"goal","title":"区分日志中的事实与推断（0—2 分）","maxScore":2}],
  "answer":"","answered":false,"score":0,"scoreLabel":"0 / 6",
  "rubricFeedback":[{"code","title","score","maxScore","reason","matchedKeywords":[]}],
  "referenceAnswer":"","expectedPoints":[],"scoredAt":"","completed":false,
  "disclaimer":"评分由内置规则…","generationNote":"本题由内置规则模板…未调用大模型",
  "source":"rule_template|fixed_bank","sourceLabel":"规则模板组合生成",
  "variantCode":"TRN-4","familyLabel":"训练稳定性与优化（规则模板）","paramsUsed":{"lrate":"0.1"},
  "selectionReason":"选题依据（任务类型 + 命中的问题类型 + 避开已掌握内容）",
  "dedupNote":"已与 6 条示例情境、N 道历史题目比对，最高相似度 0.00（阈值 0.45）",
  "comparedItems":["示例情境 1","历史题目：…"],"similarityMax":0,
  "matchedKeywords":[],"missingPoints":[]
}
```
`rubric` 与 `disclaimer` 在作答前也要展示；`referenceAnswer`/`expectedPoints` 只能在作答后出现；`source` 必须与 `generationNote` 一起展示，避免把模板组合说成开放式动态生成。

### GET /api/sessions/:id/summary
`{ ok, summary: Summary, markdown: "Markdown 文本", teacherSummary: "文本" }`

**Summary**：`{ generatedAt, learningGoal, initialExplanation, keyEvidence[], taskType, taskTypeLabel, claims[], knowledge[], stillOpen[], resolved[], evidenceLimits[], mainIssue, issueEvidence[], diagnosisReasoning, categories[], hintLevels[], hintTrail[], revisionSummary, revisionImproved, revisionCount, transfer:{itemId,topic,scenario,taskType,taskTypeLabel,knowledgePoint,question,answer,score,maxScore,scored,completed,source,sourceLabel,selectionReason,dedupNote,generationNote,rubricFeedback[],matchedKeywords[],missingPoints[],referenceAnswer}, mastery:{status,label}, studentNextSteps[], teacherNotes[], teacherSummary, mode, modeLabel, llmUsed, dataNotice, disclaimer, teacherIntervention }`

`stillOpen` / `resolved` 只来自真实发生的主张诊断；未使用提示、未提交的修正与未完成的迁移测试都会在反馈中写明「未使用 / 尚未完成」，不会写成已完成。

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
