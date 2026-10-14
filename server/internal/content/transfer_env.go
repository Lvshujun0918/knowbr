package content

func init() {
	registerFamily(environmentFamily)
}

// environmentFamily 是“环境与代码排障”任务下的迁移题族。
// 环境问题考的是定位与验证能力，因此题目只涉及报错信息、依赖与数据检查，
// 不涉及任何模型指标或概念辨析。
var environmentFamily = ScenarioFamily{
	Code:     "ENV",
	TaskType: TaskCodeEnv,
	Label:    "环境与代码排障（规则模板）",
	Params: []ScenarioParam{
		{Slot: "pkg", Label: "相关依赖", Options: []string{"scikit-learn", "pandas"}},
		{Slot: "ver", Label: "课程要求的版本", Options: []string{"1.3.0", "2.2.1"}},
	},
	Variants: []ScenarioVariant{
		{
			Code:              "ENV-1",
			Topic:             "换一台机器后依赖导入失败",
			KnowledgePoint:    "报错信息定位与依赖版本一致性",
			Archetypes:        []string{"E0"},
			Scenario:          "在另一台机器上导入 {{pkg}} 失败（预期版本 {{ver}}）",
			Context:           "同学在另一台电脑上运行同一份实验代码，导入 {{pkg}} 时出现报错，本机运行正常。课程环境要求的 {{pkg}} 版本是 {{ver}}。",
			Question:          "请说明你会按什么顺序定位这个问题：先看什么、再看什么、如何确认依赖版本是否一致？请给出你希望在报错输出中看到的至少两项信息。",
			RequiredData:      []string{"报错原文（含最后几行）", "两端的依赖版本信息"},
			AnswerRequirement: "先定位报错类型，再检查依赖版本与安装方式，最后说明如何验证修复成功。",
			Rubric: []RubricDim{
				{
					Code:     "locate",
					Title:    "先定位报错信息本身（0—2 分）",
					MaxScore: 2,
					KeywordGroups: []KeywordGroup{
						{"报错", "最后一行", "traceback", "错误信息", "异常类型"},
						{"模块名", "缺失", "no module named", "importerror", "包名"},
					},
					FullHitHint: "先读报错本身，并指出要从报错里取出模块名，这是排障的正确起点。",
					PartialHint: "提到了看报错或找模块名中的一项，建议把“读报错最后一行”作为第一步说清楚。",
					MissHint:    "没有从报错信息入手，直接跳到改代码或重装，缺少定位依据。",
				},
				{
					Code:     "env",
					Title:    "检查依赖版本与运行环境（0—2 分）",
					MaxScore: 2,
					KeywordGroups: []KeywordGroup{
						{"版本", "{{ver}}", "pip list", "conda list", "requirements"},
						{"虚拟环境", "解释器", "python 版本", "环境", "安装方式"},
					},
					FullHitHint: "同时检查了依赖版本与运行环境，覆盖了“本机能跑、他机不能跑”的典型原因。",
					PartialHint: "只检查了版本或环境中的一项，建议把另一项也纳入排查。",
					MissHint:    "没有检查版本或运行环境，无法解释两台机器上的差异。",
				},
				{
					Code:     "verify",
					Title:    "说明如何验证修复（0—2 分）",
					MaxScore: 2,
					KeywordGroups: []KeywordGroup{
						{"重新运行", "再跑", "验证", "确认", "复现"},
						{"导入", "import", "打印版本", "输出", "结果"},
					},
					FullHitHint: "给出了可执行的验证方式：安装后重新导入并打印版本，再跑一次实验。",
					PartialHint: "提到了重新运行或打印版本中的一项，建议把验证步骤写完整。",
					MissHint:    "没有说明如何确认问题已经解决。",
				},
			},
			ExpectedPoints: []string{
				"先读报错的最后一行，从 No module named / ImportError 里取出缺失的模块名。",
				"再检查该模块在两台机器上的版本与安装方式（虚拟环境、解释器）。",
				"修复后重新导入并打印版本，再运行一次实验确认问题消失。",
			},
			ReferenceAnswer: "先读报错：如果最后一行是 ModuleNotFoundError 或 ImportError，直接取出缺失的模块名。接着检查依赖：在两端分别执行 `pip list | grep -i {{pkg}}`（或 conda list），确认版本是否为课程要求的 {{ver}}，并确认是否在同一个虚拟环境与解释器中运行。最后验证修复：重新执行导入语句并打印 `{{pkg}}.__version__`，再跑一次实验，确认输出与预期一致。整个过程中，报错原文与版本信息都是可核查的证据，不需要修改实验逻辑。",
		},
		{
			Code:              "ENV-2",
			Topic:             "数据文件找不到",
			KnowledgePoint:    "数据路径检查与相对路径问题",
			Archetypes:        []string{"E0"},
			Scenario:          "在其它目录运行实验时读不到数据文件（{{pkg}} 版本 {{ver}}）",
			Context:           "实验代码能运行，但换到另一个目录或在脚本里调用时抛出 FileNotFoundError；{{pkg}} 版本为 {{ver}}。",
			Question:          "请说明你会如何定位这个路径问题，以及在代码里加什么输出可以一次性确认读取位置是否正确。",
			RequiredData:      []string{"完整报错", "代码中读取文件的写法"},
			AnswerRequirement: "说明检查步骤、要打印什么信息，以及怎样改成不依赖当前工作目录的写法。",
			Rubric: []RubricDim{
				{
					Code:     "locate",
					Title:    "定位到读取路径而不是猜测（0—2 分）",
					MaxScore: 2,
					KeywordGroups: []KeywordGroup{
						{"报错", "filenotfound", "路径", "找不到", "位置"},
						{"实际读取", "打印路径", "abspath", "当前目录"},
					},
					FullHitHint: "先确认程序实际读了哪个路径，这是定位文件问题的关键一步。",
					PartialHint: "提到了报错或路径检查中的一项，建议把“打印实际路径”写进步骤。",
					MissHint:    "没有确认实际读取位置，直接猜测文件是否存在。",
				},
				{
					Code:     "cause",
					Title:    "解释相对路径与工作目录的关系（0—2 分）",
					MaxScore: 2,
					KeywordGroups: []KeywordGroup{
						{"相对路径", "工作目录", "执行目录", "cwd"},
						{"绝对路径", "基准目录", "脚本所在", "os.path.join", "pathlib"},
					},
					FullHitHint: "说明了相对路径依赖当前工作目录，并给出改用脚本所在目录或绝对路径的做法。",
					PartialHint: "提到相对路径或工作目录中的一项，建议把两者的因果关系说清。",
					MissHint:    "没有解释为什么换个目录就读不到文件。",
				},
				{
					Code:     "verify",
					Title:    "给出可验证的修复方式（0—2 分）",
					MaxScore: 2,
					KeywordGroups: []KeywordGroup{
						{"重新运行", "再跑", "验证", "确认"},
						{"导入成功", "读取成功", "形状", "shape", "没有报错"},
					},
					FullHitHint: "给出了明确的验证信号：重新运行后能读到数据并检查 shape。",
					PartialHint: "提到重新运行或检查结果中的一项，建议补上验证信号。",
					MissHint:    "没有说明怎样确认问题已经解决。",
				},
			},
			ExpectedPoints: []string{
				"先看报错中给出的路径，确认程序实际读取的位置。",
				"用 os.path.abspath 或 pathlib 打印绝对路径，判断是否是相对路径导致的。",
				"改成以脚本所在目录为基准的路径，重新运行并检查数据形状。",
			},
			ReferenceAnswer: "FileNotFoundError 的报错里通常包含程序实际尝试打开的路径，第一步是把这条路径打出来核对。第二步在读取数据前加一行 `print(os.path.abspath(path))`，这样可以看到相对路径解析到了哪个目录；如果结果与你以为的位置不同，说明是工作目录问题，应改为以脚本所在目录为基准（如 `pathlib.Path(__file__).parent / 'data.csv'`）或使用绝对路径。最后重新运行，确认数据能被读取并打印 `data.shape` 以核对是否读到预期内容。",
		},
		{
			Code:              "ENV-3",
			Topic:             "数据形状不一致导致的报错",
			KnowledgePoint:    "形状与类型检查",
			Archetypes:        []string{"E0"},
			Scenario:          "训练时报形状不匹配错误（{{pkg}} {{ver}}）",
			Context:           "训练时报错提示输入与标签数量或维度不一致，代码在别人的机器上可以运行，本机的 {{pkg}} 版本为 {{ver}}。",
			Question:          "请说明你会打印哪些信息来定位形状问题，以及如何判断是数据问题还是代码问题。",
			RequiredData:      []string{"完整报错", "数据与标签的维度信息"},
			AnswerRequirement: "列出要打印的字段、判断依据与验证方式。",
			Rubric: []RubricDim{
				{
					Code:     "locate",
					Title:    "打印形状与长度而不是猜测（0—2 分）",
					MaxScore: 2,
					KeywordGroups: []KeywordGroup{
						{"shape", "维度", "形状", "len(", "数量"},
						{"打印", "输出", "检查", "对比"},
					},
					FullHitHint: "用打印形状与长度来定位，这是可核查的做法。",
					PartialHint: "提到形状或打印中的一项，建议把要打印的具体字段列出来。",
					MissHint:    "没有给出任何可核查的检查方式。",
				},
				{
					Code:     "cause",
					Title:    "区分数据问题与代码问题（0—2 分）",
					MaxScore: 2,
					KeywordGroups: []KeywordGroup{
						{"样本数", "标签", "对齐", "一一对应"},
						{"代码", "预处理", "读取", "版本差异", "数据"},
					},
					FullHitHint: "同时考虑了数据侧与代码侧的差异，判断依据清晰。",
					PartialHint: "只提到一侧的原因，建议补充另一侧的检查。",
					MissHint:    "没有说明如何区分是数据还是代码导致的不一致。",
				},
				{
					Code:     "verify",
					Title:    "给出验证与修复步骤（0—2 分）",
					MaxScore: 2,
					KeywordGroups: []KeywordGroup{
						{"重新运行", "再跑", "验证", "确认"},
						{"一致", "匹配", "对齐", "通过", "成功"},
					},
					FullHitHint: "修复后给出了明确的验证信号：形状匹配且训练可以继续。",
					PartialHint: "提到重新运行或匹配中的一项，建议补全验证信号。",
					MissHint:    "没有说明修复后如何确认问题消失。",
				},
			},
			ExpectedPoints: []string{
				"打印 X.shape、y.shape 与 len(y)，确认样本数与标签是否一一对应。",
				"检查数据读取与预处理步骤，判断是数据本身的问题还是代码处理造成的。",
				"修复后重新运行，确认形状匹配且训练正常进行。",
			},
			ReferenceAnswer: "形状类报错应先取证：打印 `X.shape`、`y.shape` 与 `len(y)`，确认样本数与标签数量是否一一对应，这一步能区分大部分不一致。如果是标签与样本数不等，检查数据读取与过滤步骤（例如对 X 做了过滤但没有同步过滤 y）；如果维度不符，检查是否需要 reshape 以及预处理是否与队友一致。修复后重新运行，以“形状匹配、训练可以继续”作为验证信号，并把报错原文与形状信息一起记录下来，便于对照。",
		},
	},
}
