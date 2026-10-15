// 与 docs/API_CONTRACT.md（冻结版 v1）及 server/ 真实返回对齐的接口定义。
// 字段可空性按服务端实际行为标注：Go 的 json.RawMessage + omitempty 在为空时整个字段缺席；
// 部分切片字段（如 completeness.missing、transfer.missingPoints）为空时返回 null。

/** 统一错误信封：{ ok:false, error:{...} } */
export interface ApiErrorBody {
  code: string
  field?: string
  message: string
  detail?: string
}

/* ---------------- 元信息 ---------------- */

export interface MetaMode {
  code: string
  label: string
  notice: string
  llmConfigured: boolean
  llmModel: string
}

export interface StepDef {
  key: string
  title: string
}

export interface CategoryDef {
  code: string
  label: string
}

export interface ArchetypeDef {
  code: string
  name: string
  knowledgePoint: string
  categories: CategoryDef[]
  keywords: string[]
  teacherCondition: string
  direct: boolean
}

export interface StorageInfo {
  location: string
  note: string
}

export interface CoverageTaskType {
  code: string
  label: string
  families: number
}

export interface CoverageInfo {
  taskTypes: CoverageTaskType[]
  taskTypeCount: number
  ruleCount: number
  caseCount: number
  note: string
}

export interface Meta {
  productName: string
  tagline: string
  course: string
  focus: string
  mode: MetaMode
  steps: StepDef[]
  categories: CategoryDef[]
  archetypes: ArchetypeDef[]
  honesty: string[]
  storage: StorageInfo
  coverage?: CoverageInfo
}

export interface MetaResponse extends Meta {
  ok: boolean
}

/* ---------------- 演示案例 ---------------- */

export interface DemoSubmission {
  experimentTitle: string
  experimentGoal: string
  datasetDescription: string
  modelName: string
  codeSnippet: string
  executionLog: string
  confusionMatrixText: string
  classLabels: string[]
  metricsText: string
  studentInitialExplanation: string
  question: string
}

export interface DemoCase {
  id: string
  name: string
  description: string
  dataNotice: string
  expectedFocus: string[]
  submission: DemoSubmission
}

export interface DemoCasesResponse {
  ok: boolean
  notice: string
  cases: DemoCase[]
}

/* ---------------- 混淆矩阵与指标 ---------------- */

export interface ClassMetric {
  index: number
  label: string
  support: number
  predicted: number
  truePositive: number
  falsePositive: number
  falseNegative: number
  recall: number
  recallUndefined: boolean
  precision: number
  precisionUndefined: boolean
  f1: number
}

export interface MatrixMetrics {
  labels: string[]
  values: number[][]
  total: number
  correct: number
  accuracy: number
  classes: ClassMetric[]
  macroRecall: number
  macroPrecision: number
  macroF1: number
  weightedF1: number
  maxClassRatio: number
  imbalanceRatio: number
  notes: string[]
  zeroRecallClasses: string[]
  imbalanced: boolean
}

export interface MetricsComputeRequest {
  confusionMatrix?: number[][]
  confusionMatrixText?: string
  classLabels?: string[]
}

export interface MetricsComputeResponse {
  ok: boolean
  metrics: MatrixMetrics | null
  labels?: string[]
  message?: string
}

/* ---------------- 提交材料 ---------------- */

export interface Submission {
  experimentTitle?: string
  experimentGoal?: string
  datasetDescription?: string
  modelName?: string
  codeSnippet?: string
  executionLog?: string
  confusionMatrix?: number[][]
  confusionMatrixText?: string
  classLabels?: string[]
  metricsText?: string
  studentInitialExplanation: string
  question?: string
  demoCaseId?: string
  useLLM?: boolean
}

/* ---------------- 诊断 ---------------- */

export interface CategoryRef {
  code: string
  label: string
}

export interface ArchetypeRef {
  code: string
  name: string
  knowledgePoint: string
}

export interface Contradiction {
  detected: boolean
  claim: string
  detail: string
}

export interface Completeness {
  sufficient: boolean
  haveExplanation: boolean
  haveMatrix: boolean
  havePerClassMetrics: boolean
  haveDistribution: boolean
  haveTaskGoal: boolean
  haveRunEvidence: boolean
  missing: string[] | null
  limitations: string[] | null
}

export interface EvidenceNeed {
  name: string
  required: boolean
  present: boolean
  impact: string
  note: string
}

/** 一条被单独分析的学生主张（需求文档 2.1 / 2.7）。 */
export interface Claim {
  id: string
  quote: string
  knowledgePoint: string
  taskType: string
  status: string
  statusLabel: string
  judgment: string
  reason: string
  evidence: string
  evidenceBoundary: string
  nextQuestion: string
  signals: string[] | null
}

/** 当前任务下的知识点状态：未涉及不等于未掌握。 */
export interface KnowledgeItem {
  code: string
  name: string
  status: string
  statusLabel: string
  note: string
  required: boolean
  applicable: boolean
}

export interface Diagnosis {
  taskType: string
  taskTypeLabel: string
  taskFocus: string[] | null
  taskOffTopic: string[] | null
  categories: CategoryRef[]
  primaryCategory: string
  primaryCategoryName: string
  mainIssue: string
  evidence: string[]
  evidenceSource: string[]
  evidenceNeeds: EvidenceNeed[] | null
  claims: Claim[] | null
  knowledge: KnowledgeItem[] | null
  primaryClaimId: string
  reasoning: string
  masteryStatus: string
  masteryLabel: string
  nextStep: string
  matchedArchetypes: ArchetypeRef[]
  primaryArchetype: string
  contradiction: Contradiction
  teacherIntervention: boolean
  teacherReason: string
  completeness: Completeness
  matrixFacts: string[] | null
  generatedAt: string
  llmUsed: boolean
  llmNote: string
  source: string
}

/* ---------------- 辅导 ---------------- */

export interface Hint {
  level: number
  content: string
  createdAt: string
  archetypeCode?: string
  socratic: boolean
  requestedByStudent: boolean
  why?: string
  targetKnowledge?: string
  addressesClaim?: string
}

export interface Revision {
  content: string
  createdAt: string
  feedback: string
  progress: string // improved | partial | none
  points: string[] | null
  addressedKeywords: string[] | null
  missingKeywords: string[] | null
}

export interface HintState {
  nextLevel: number
  maxLevel: number
  usedLevels: number[] | null
  noProgressRounds: number
  teacherIntervention: boolean
  teacherReason: string
}

/* ---------------- 迁移测试 ---------------- */

export interface RubricDimRef {
  code: string
  title: string
  maxScore: number
}

export interface RubricFeedback {
  code: string
  title: string
  score: number
  maxScore: number
  reason: string
  matchedKeywords: string[] | null
}

export interface TransferState {
  itemId: string
  topic: string
  scenario: string
  knowledgePoint: string
  taskType: string
  taskTypeLabel: string
  context: string
  question: string
  requiredData: string[] | null
  answerRequirement: string
  maxScore: number
  rubric: RubricDimRef[]
  answer?: string
  answered: boolean
  score: number
  scoreLabel: string
  rubricFeedback: RubricFeedback[] | null
  referenceAnswer?: string
  expectedPoints?: string[] | null
  scoredAt?: string
  completed: boolean
  disclaimer: string
  generationNote: string
  source: string
  sourceLabel: string
  variantCode?: string
  familyLabel?: string
  paramsUsed?: Record<string, string> | null
  selectionReason: string
  dedupNote: string
  comparedItems: string[] | null
  similarityMax: number
  matchedKeywords: string[] | null
  missingPoints: string[] | null
}

/* ---------------- 学习反馈 ---------------- */

export interface TransferSummary {
  itemId: string
  topic: string
  scenario: string
  taskType: string
  taskTypeLabel: string
  knowledgePoint: string
  question: string
  answer: string
  score: number
  maxScore: number
  scored: boolean
  completed: boolean
  source: string
  sourceLabel: string
  selectionReason: string
  dedupNote: string
  generationNote: string
  rubricFeedback: RubricFeedback[] | null
  matchedKeywords: string[] | null
  missingPoints: string[] | null
  referenceAnswer?: string
}

export interface MasteryStatusRef {
  status: string
  label: string
}

export interface Summary {
  generatedAt: string
  learningGoal: string
  initialExplanation: string
  keyEvidence: string[] | null
  taskType: string
  taskTypeLabel: string
  claims: Claim[] | null
  knowledge: KnowledgeItem[] | null
  stillOpen: string[] | null
  resolved: string[] | null
  evidenceLimits: string[] | null
  mainIssue: string
  issueEvidence: string[] | null
  diagnosisReasoning: string
  categories: CategoryRef[]
  hintLevels: number[] | null
  hintTrail: Hint[] | null
  revisionSummary: string
  revisionImproved: boolean
  revisionCount: number
  transfer?: TransferSummary | null
  mastery: MasteryStatusRef
  studentNextSteps: string[] | null
  teacherNotes: string[] | null
  teacherSummary: string
  mode: string
  modeLabel: string
  llmUsed: boolean
  dataNotice: string
  disclaimer: string
  teacherIntervention: boolean
}

/* ---------------- 会话与记录 ---------------- */

export interface ExperimentDTO {
  experimentTitle: string
  experimentGoal: string
  datasetDescription: string
  modelName: string
  codeSnippet: string
  executionLog: string
  classLabels: string[] | null
  metricsText: string
  studentInitialExplanation: string
  question: string
}

export interface Session {
  id: string
  createdAt: string
  updatedAt: string
  status: string
  mode: string
  modeLabel: string
  modeNotice: string
  llmUsed: boolean
  llmFallbackReason?: string
  isDemo: boolean
  demoCaseId?: string
  dataNotice?: string
  experiment: ExperimentDTO
  matrix?: number[][] | null
  metrics?: MatrixMetrics | null
  diagnosis?: Diagnosis | null
  hints: Hint[]
  hintState: HintState
  revisions: Revision[]
  transfer?: TransferState | null
  summary?: Summary | null
  teacherSummary: string
  mainIssue: string
  categories: string[]
  transferScore: number
}

export interface RecordDTO {
  id: string
  title: string
  createdAt: string
  updatedAt: string
  status: string
  statusLabel: string
  mode: string
  modeLabel: string
  llmUsed: boolean
  isDemo: boolean
  mainIssue: string
  categories: string[]
  categoryLabels: string[]
  primaryArchetype: string
  hintLevelsUsed: number[] | null
  revisionImproved: boolean
  revisionCount: number
  transferScore: number
  transferMaxScore: number
  teacherIntervention: boolean
}

export interface SessionListStorage {
  location: string
  total: number
  demoRecords: number
  realRecords: number
  note: string
}

/* ---------------- 教师视图 ---------------- */

export interface TeacherRecord {
  record: RecordDTO
  mainIssue: string
  hintLevels: number[] | null
  summary: string
  intervention: boolean
}

export interface CommonIssue {
  code: string
  label: string
  count: number
  recordIds: string[]
}

export interface TeacherCounts {
  total: number
  real: number
  demo: number
}

/* ---------------- 接口响应 ---------------- */

export interface SessionResponse {
  ok: boolean
  session: Session
}

export interface SessionListResponse {
  ok: boolean
  records: RecordDTO[]
  storage: SessionListStorage
}

/* ---------------- 智能体决策 ---------------- */

export interface AgentToolCall {
  name: string
  purpose: string
  result: string
}

export interface AgentPlanItem {
  step: string
  title: string
  status: string
  note?: string
}

export interface AgentRepeatedIssue {
  code: string
  label: string
  count: number
  advice: string
}

export interface AgentMemory {
  lookedAt: number
  repeated: AgentRepeatedIssue[] | null
  mastered: string[] | null
  milestones: string[] | null
  note: string
}

export interface AgentStep {
  action: string
  actionLabel: string
  title: string
  reason: string
  requires: string[] | null
  suggestHintLevel: number
  blocked: boolean
  blockedReason?: string
  canAct: boolean
  plan: AgentPlanItem[] | null
  tools: AgentToolCall[] | null
  memory?: AgentMemory | null
  decidedAt: string
  policyNote: string
}

export interface HintResponse {
  ok: boolean
  hint: Hint
  hints: Hint[]
  hintState: HintState
  alreadyProvided: boolean
}

export interface AgentDecisionResponse {
  ok: true
  agent: AgentStep
}

export interface AgentStepResponse {
  ok: true
  agent: AgentStep
  executed: boolean
  executedNote: string
  hint?: Hint
  hints: Hint[]
  hintState: HintState
  session: Session
}

export interface RevisionResponse {
  ok: boolean
  revision: Revision
  revisions: Revision[]
  hintState: HintState
  improved: boolean
}

export interface TransferQuestionResponse {
  ok: boolean
  transfer: TransferState
  reused: boolean
}

export interface TransferResponse {
  ok: boolean
  transfer: TransferState
}

export interface TransferCompleteResponse {
  ok: boolean
  transfer: TransferState
  summary: Summary
  markdown: string
  session: Session
}

export interface SummaryResponse {
  ok: boolean
  summary: Summary
  markdown: string
  teacherSummary: string
}

export interface TeacherOverviewResponse {
  ok: boolean
  records: TeacherRecord[]
  commonIssues: CommonIssue[]
  note: string
  counts: TeacherCounts
}

export interface DeleteResponse {
  ok: boolean
  deleted?: number
}
