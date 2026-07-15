// Package suggest implements the "军情哨" analysis engine for pflow.
// It evaluates ~20 scenarios (core + long-tail) based on session states,
// project priorities, activity metrics, and time-of-day heuristics to
// produce actionable suggestions for the user.
//
// Design: docs/design/08-suggest.md
package suggest

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/pancake-lee/pflow/internal/timetrack"
)

// Thresholds for time-based scenarios, tuned for message-count-based
// TodayMinutes (1 message ≈ 3 active minutes). See design doc §8:
// docs/design/09-active-time-calculation.md
const (
	// S15: primary ≥ 20 messages (≈60 min today) → positive feedback
	thresholdEfficientMinutes = 60

	// S20: primary ≥ 30 messages (≈90 min today) → suggest switching
	thresholdOver4hMinutes = 90

	// S10: secondary > primary + 2 messages (≈6 min) → attention imbalance
	thresholdImbalanceDelta = 6.0

	// S11: normal project > primary AND ≥ 15 min today → meaningful check
	thresholdNormalMinMeaningful = 15
)

// ── Types ─────────────────────────────────────────────────────────────

// SessionInfo is the unified view of a single agent session used by the
// suggestion engine. The CLI layer is responsible for converting
// claude.SessionSummary / hermes.SessionSummary into this format.
type SessionInfo struct {
	AgentType    string    // "claude" or "hermes"
	AgentName    string    // display name (Claude -n name or first user message)
	ProjectPath  string    // working directory
	Status       string    // "busy", "waiting", "idle", "unknown"
	LastActive   time.Time // most recent activity timestamp
	FirstActive  time.Time // first activity in window (for today's usage)
	PID          int
	IsRunning    bool   // process is alive
	LastReq      string // latest user request (for waiting hint)
	MatchedRoot  string // matched project root path
	RootPriority string // "primary", "secondary", "normal", ""
	MessageCount int    // number of messages today, used for time estimation
}

// ProjectSummary holds per-project (matched root) aggregated metrics.
type ProjectSummary struct {
	Path         string    // project root path (key)
	Priority     string    // "primary", "secondary", "normal"
	Name         string    // display name (basename of path)
	TodayMinutes float64   // cumulative focus minutes today
	LastActive   time.Time // most recent activity across all sessions
	SessionCount int
	BusyCount    int
	WaitingCount int
	IdleCount    int
	IdleMinutes  float64 // minutes since last activity
}

// Suggestion is one analysis recommendation with a priority for ordering.
type Suggestion struct {
	ScenarioID string // scenario identifier, e.g. "scenario_001"
	Icon       string // emoji indicator
	Text       string // formatted text (may contain newlines)
	Priority   int    // lower = more urgent, used for stable sort
}

// Input is the complete dataset needed for suggestion analysis.
type Input struct {
	Sessions       []SessionInfo
	Projects       []ProjectSummary
	CurrentProject string // project root path the user is currently viewing
	Now            time.Time
}

// KnowledgeTip is a cognitive-science knowledge card associated with one or
// more suggest scenarios. It explains the "why" behind a suggestion.
type KnowledgeTip struct {
	ID               string   // unique identifier, e.g. "kt_attention_residue"
	Title            string   // theory name in Chinese
	Theory           string   // 1-2 sentence theoretical basis
	Design           string   // 1 sentence design mapping
	RelatedScenarios []string // scenario IDs this tip explains
}

// KnowledgeTipJSON is the API-serializable subset of KnowledgeTip.
type KnowledgeTipJSON struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Theory string `json:"theory"`
	Design string `json:"design"`
}

// ToJSON converts a KnowledgeTip to its JSON-safe form.
func (k *KnowledgeTip) ToJSON() *KnowledgeTipJSON {
	if k == nil {
		return nil
	}
	return &KnowledgeTipJSON{
		ID:     k.ID,
		Title:  k.Title,
		Theory: k.Theory,
		Design: k.Design,
	}
}

// allKnowledgeTips holds the complete set of 12 knowledge tips.
// See docs/design/10-tips.md §4 for the full content reference.
var allKnowledgeTips = []KnowledgeTip{
	{
		ID:    "kt_attention_residue",
		Title: "注意力残留与认知惰性",
		Theory: "从任务 A 切换到 B 时，注意力不会完全转移——仍有部分「残留」在 A 上。重启中断的任务，需额外消耗 30%-40% 的脑能量。",
		Design: "主线保护期 15 分钟不推送干扰，正是为了尊重这一能耗规律。",
		RelatedScenarios: []string{
			"scenario_001", "scenario_002", "scenario_003", "scenario_004", "scenario_012",
		},
	},
	{
		ID:    "kt_cognitive_offloading",
		Title: "认知卸载的双面性",
		Theory: "「替代性卸载」（AI 替你做判断）会导致能力退化；「互补性卸载」（AI 替你记忆，你做判断）才带来认知扩增。",
		Design: "pflow 只展示状态，永远不替你做「切不切」的决策——把判断留给你，把监控交给系统。",
		RelatedScenarios: []string{
			"scenario_008", "scenario_009", "scenario_015", "scenario_018",
		},
	},
	{
		ID:    "kt_metacognitive_bottleneck",
		Title: "元认知监控的代价",
		Theory: "大脑无法同时运行「当前任务」和「监控当前任务」——后者会占用近 50% 的执行控制资源。",
		Design: "遮罩层和红绿灯把监控「外包」给视觉皮层——扫一眼就知道全局，不用费脑去记。",
		RelatedScenarios: []string{
			"scenario_010", "scenario_011", "scenario_006",
		},
	},
	{
		ID:    "kt_embodied_cognition",
		Title: "物理环境即思维外挂",
		Theory: "物理环境（屏幕布局、光标位置）是思维最强大的外挂支架——视觉皮层能以无意识速度处理环境线索。",
		Design: "pflow 不内嵌终端，就是让你用「物理性切换窗口」的动作重置大脑上下文。",
		RelatedScenarios: []string{
			"scenario_006", "scenario_007", "scenario_019",
		},
	},
	{
		ID:    "kt_interruption_recovery",
		Title: "中断恢复的代价",
		Theory: "被中断后，平均需要 23 分钟才能回到原有的深度工作状态。中断越频繁，有效深度工作时间越短。",
		Design: "提醒分数用「幂函数放大」机制，避免多项目同时高亮——只有远超阈值的才推送通知。",
		RelatedScenarios: []string{
			"scenario_001", "scenario_002", "scenario_003", "scenario_004",
		},
	},
	{
		ID:    "kt_prediction_error",
		Title: "预测误差与判断信心",
		Theory: "自己思考得出答案时，大脑会产生奖励信号；AI 直接给出答案时，信号消失，长期会削弱判断信心。",
		Design: "Agent 持续运行超 10 分钟时提醒「可能卡住，建议检查」——让你重新介入判断，保持异常监测敏感度。",
		RelatedScenarios: []string{
			"scenario_014",
		},
	},
	{
		ID:    "kt_primary_secondary_strategy",
		Title: "为何强制设定 1 个主线",
		Theory: "工作记忆容量有限（约 4 个组块），超出容量的多线程管理本身就是认知负担。",
		Design: "强制设定 1 主线 + 最多 2 支线，把「排序」决策前置化，避免工作过程中反复纠结优先级。",
		RelatedScenarios: []string{
			"scenario_010", "scenario_011", "scenario_018",
		},
	},
	{
		ID:    "kt_multitasking_illusion",
		Title: "多任务只是快速切换的幻觉",
		Theory: "大脑本质上是串行处理器——所谓的「并行」只是极速切换的幻觉。每次切换都有能耗。",
		Design: "红绿灯状态撕掉「并行」伪装：此刻只有一个 🟡 的会话在占用你的思考排队名额。",
		RelatedScenarios: []string{
			"scenario_009", "scenario_012", "scenario_018",
		},
	},
	{
		ID:    "kt_chunking",
		Title: "组块化认知与零归类设计",
		Theory: "长时记忆依靠「组块」压缩信息——把零散信息打包成有意义的单元，是大脑处理复杂信息的核心机制。",
		Design: "「路径即项目」——用目录路径作为天然分组依据，避免手动维护分组的心力消耗。",
		RelatedScenarios: []string{},
	},
	{
		ID:    "kt_positive_feedback",
		Title: "正向反馈驱动持续专注",
		Theory: "多巴胺系统在获得正向反馈时被激活，能增强持续专注的动机。小胜利的记录比大目标更能维持日常动力。",
		Design: "一切正常或高效完成时给出 ✅ / 🎉 级正向反馈——不是空话，是对大脑奖励机制的调用。",
		RelatedScenarios: []string{
			"scenario_008", "scenario_015",
		},
	},
	{
		ID:    "kt_decision_fatigue",
		Title: "决策疲劳与调度前置",
		Theory: "每做一个决策都消耗认知资源。决策次数累积后，后续决策质量会下降——这就是决策疲劳。",
		Design: "把「切不切」的判断前置到策略设定阶段，执行阶段只需看状态、按计划行动，大幅减少执行中的决策次数。",
		RelatedScenarios: []string{
			"scenario_006",
		},
	},
	{
		ID:    "kt_offloading_boundary",
		Title: "卸载的边界：保留不可让渡的阵地",
		Theory: "价值观判断、审美选择、生死攸关的直觉——这些领域的卸载会导致不可逆的能力丧失。",
		Design: "pflow 帮你「记住状态」，但永远不替你「做出选择」。你是统帅，兵权不可让渡。",
		RelatedScenarios: []string{},
	},
}

// scenarioTipIndex is a map from scenario ID to the first matching KnowledgeTip.
// Built once at init time from allKnowledgeTips.
var scenarioTipIndex map[string]*KnowledgeTip

func init() {
	scenarioTipIndex = make(map[string]*KnowledgeTip, len(allKnowledgeTips)*3)
	for i := range allKnowledgeTips {
		tip := &allKnowledgeTips[i]
		for _, sid := range tip.RelatedScenarios {
			if _, exists := scenarioTipIndex[sid]; !exists {
				scenarioTipIndex[sid] = tip
			}
		}
	}
}

// LookupTip returns the KnowledgeTip associated with a scenario ID, or nil
// if no tip is mapped to that scenario.
func LookupTip(scenarioID string) *KnowledgeTip {
	return scenarioTipIndex[scenarioID]
}

// ── Public API ────────────────────────────────────────────────────────

// Generate runs all scenario checks and returns applicable suggestions
// ordered by priority (most urgent first).
func Generate(input Input) []Suggestion {
	var out []Suggestion

	// Build lookup maps for fast access
	projMap := make(map[string]ProjectSummary, len(input.Projects))
	for _, p := range input.Projects {
		projMap[p.Path] = p
	}

	// Find primary and secondary projects
	primary := findProject(input.Projects, "primary")
	secondary := findProject(input.Projects, "secondary")

	// Which project the user is currently viewing
	currentProject := input.CurrentProject

	// Collect waiting sessions ordered by wait duration
	waitingSessions := filterByStatus(input.Sessions, "waiting")
	busySessions := filterByStatus(input.Sessions, "busy")
	activeProjects := filterActiveProjects(input.Projects)

	// ── Scenario evaluation (priority order) ───────────────────

	// S1: 🔴 waiting > 5 min (any project)
	out = appendIf(out, checkUrgentWaiting(waitingSessions, input.Now, 5*time.Minute, primary))

	// S2: 🟡 primary waiting > 2 min
	out = appendIf(out, checkPrimaryWaiting(waitingSessions, primary, input.Now, 2*time.Minute))

	// S12: 🔴 multiple waiting sessions
	out = appendIf(out, checkMultipleWaiting(waitingSessions, primary, secondary, input.Now))

	// S3: 🟡 primary idle 30–60 min (no waiting)
	out = appendIf(out, checkPrimaryIdle(primary, projMap, input.Now, 30*time.Minute, 60*time.Minute, waitingSessions, "🟡", "scenario_003"))

	// S4: 🔶 primary idle > 60 min
	out = appendIf(out, checkPrimaryIdle(primary, projMap, input.Now, 60*time.Minute, 0, waitingSessions, "🔶", "scenario_004"))

	// S5: 🔵 secondary waiting but primary active
	out = appendIf(out, checkSecondaryWaitingPrimaryActive(waitingSessions, secondary, primary, currentProject, input.Now))

	// S14: ⚠️ agent stuck (busy > 10 min)
	out = appendIf(out, checkAgentStuck(busySessions, input.Now, 10*time.Minute))

	// S10: 🔵 attention imbalance (secondary today > primary + threshold)
	out = appendIf(out, checkAttentionImbalance(primary, secondary, projMap, thresholdImbalanceDelta))

	// S11: ⚪ normal project > primary today
	out = appendIf(out, checkNormalExceedsPrimary(primary, input.Projects, projMap))

	// S13: ⚠️ abnormal exit (mapping exists, process dead)
	out = appendIf(out, checkAbnormalExit(input.Sessions))

	// S8: ✅ all normal
	out = appendIf(out, checkAllNormal(primary, input.Projects, waitingSessions, currentProject))

	// S9: ✅ multiple busy (positive)
	out = appendIf(out, checkMultipleBusy(busySessions, primary, secondary))

	// S15: 🎉 primary today 60–90 min + all good
	out = appendIf(out, checkTodayEfficient(primary, projMap, waitingSessions, thresholdEfficientMinutes, thresholdOver4hMinutes))

	// S20: 🏆 primary today ≥ 30 msgs (~90 min)
	out = appendIf(out, checkPrimaryOver4h(primary, projMap, thresholdOver4hMinutes))

	// S6: ⚪ no active sessions
	out = appendIf(out, checkNoActiveSessions(input.Sessions, primary))

	// S7: ⚪ all idle > 10 min
	out = appendIf(out, checkAllIdle(input.Sessions, input.Now, 10*time.Minute, len(input.Sessions)))

	// ── Long-tail scenarios ───────────────────────────────────

	// S18: 📊 3+ projects active
	out = appendIf(out, checkMultiProject(activeProjects, primary))

	// S19: ⏰ global idle > 30 min
	out = appendIf(out, checkGlobalIdle(input.Projects, input.Now, 30*time.Minute))

	// S16: 🌆 time > 18:00
	out = appendIf(out, checkEndOfDay(input.Now))

	// S17: 🌤️ time 13:00-14:00
	out = appendIf(out, checkAfterLunch(input.Now))

	// Stable sort by priority (lower = more urgent)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority < out[j].Priority
	})

	// Deduplicate by text body (strip icon prefix) as a safety net.
	// Same text body with different icons should only appear once,
	// keeping the first one (which is the highest priority after sort).
	return deduplicate(out)
}

// ── Scenario check functions ──────────────────────────────────────────

// S1: Waiting > 5 min (urgent, any project).
func checkUrgentWaiting(waiting []SessionInfo, now time.Time, threshold time.Duration, primary *ProjectSummary) *Suggestion {
	for _, s := range waiting {
		waitMin := now.Sub(s.LastActive).Minutes()
		if waitMin >= threshold.Minutes() {
			hint := s.LastReq
			if hint == "" {
				hint = "需要你的操作"
			}
			name := agentDisplay(s)
			projLabel := projectLabel(s.RootPriority)
			icon := "🔴"
			if primary != nil && s.MatchedRoot == primary.Path {
				icon = "🔴"
			}
			return &Suggestion{
				ScenarioID: "scenario_001",
				Priority:   1,
				Icon:       icon,
				Text:       fmt.Sprintf("%s %s %s会话已等待 %.0f 分钟，%s。\n", icon, projLabel, name, waitMin, hint),
			}
		}
	}
	return nil
}

// S2: Primary waiting > 2 min.
func checkPrimaryWaiting(waiting []SessionInfo, primary *ProjectSummary, now time.Time, threshold time.Duration) *Suggestion {
	if primary == nil {
		return nil
	}
	for _, s := range waiting {
		if s.MatchedRoot == primary.Path {
			waitMin := now.Sub(s.LastActive).Minutes()
			if waitMin >= threshold.Minutes() {
				return &Suggestion{
					ScenarioID: "scenario_002",
					Priority:   3,
					Icon:       "🟡",
					Text:       fmt.Sprintf("🟡 主线任务【%s】的 %s 会话已等待 %.0f 分钟，建议尽快处理。", primary.Name, agentDisplay(s), waitMin),
				}
			}
		}
	}
	return nil
}

// S12: Multiple waiting sessions.
func checkMultipleWaiting(waiting []SessionInfo, _ *ProjectSummary, _ *ProjectSummary, now time.Time) *Suggestion {
	if len(waiting) < 2 {
		return nil
	}

	// Get the two most urgent waiting sessions
	sort.Slice(waiting, func(i, j int) bool {
		return now.Sub(waiting[i].LastActive) > now.Sub(waiting[j].LastActive)
	})

	s1, s2 := waiting[0], waiting[1]
	wait1 := now.Sub(s1.LastActive).Minutes()
	wait2 := now.Sub(s2.LastActive).Minutes()

	mainLabel := projectLabel(s1.RootPriority)
	secLabel := projectLabel(s2.RootPriority)

	return &Suggestion{
		ScenarioID: "scenario_012",
		Priority:   2,
		Icon:       "🔴",
		Text: fmt.Sprintf("🔴 多个会话需要授权：%s【%s】等待 %.0fm，%s【%s】等待 %.0fm。建议先处理主线。",
			mainLabel, agentDisplay(s1), wait1,
			secLabel, agentDisplay(s2), wait2),
	}
}

// S3/S4: Primary idle > threshold (no waiting sessions on primary).
// upperThreshold, if > 0, suppresses this scenario when idle reaches the
// higher threshold — used to keep escalation pairs (S3→S4) mutually exclusive.
func checkPrimaryIdle(primary *ProjectSummary, projMap map[string]ProjectSummary, _ time.Time, threshold time.Duration, upperThreshold time.Duration, waiting []SessionInfo, icon string, scenarioID string) *Suggestion {
	if primary == nil {
		return nil
	}
	p, ok := projMap[primary.Path]
	if !ok || p.IdleMinutes < threshold.Minutes() {
		return nil
	}
	// Don't fire if a higher-severity escalation covers this range
	if upperThreshold > 0 && p.IdleMinutes >= upperThreshold.Minutes() {
		return nil
	}
	// Don't fire if primary has waiting sessions (S1/S2 handle that)
	if hasWaitingOnProject(waiting, primary.Path) {
		return nil
	}
	idleMin := p.IdleMinutes
	if idleMin >= 60 {
		return &Suggestion{
			ScenarioID: scenarioID,
			Priority:   4,
			Icon:       icon,
			Text:       fmt.Sprintf("%s 主线任务【%s】已空闲超过 1 小时，是否应该切换回来？", icon, primary.Name),
		}
	}
	return &Suggestion{
		ScenarioID: scenarioID,
		Priority:   5,
		Icon:       icon,
		Text:       fmt.Sprintf("%s 主线任务【%s】已空闲 %.0f 分钟，建议回到主线继续工作。", icon, primary.Name, idleMin),
	}
}

// S5: Secondary waiting but primary is active.
func checkSecondaryWaitingPrimaryActive(waiting []SessionInfo, _ *ProjectSummary, primary *ProjectSummary, currentProject string, _ time.Time) *Suggestion {
	if primary == nil || len(waiting) == 0 {
		return nil
	}
	// Primary must be currently active
	if currentProject != primary.Path {
		return nil
	}
	for _, s := range waiting {
		if s.MatchedRoot != primary.Path {
			projLabel := projectLabel(s.RootPriority)
			projName := projectNameFromPath(s.MatchedRoot)
			return &Suggestion{
				ScenarioID: "scenario_005",
				Priority:   10,
				Icon:       "🔵",
				Text: fmt.Sprintf("🔵 %s任务【%s】需要授权，但主线【%s】正在活跃中。是否稍后处理？",
					projLabel, projName, primary.Name),
			}
		}
	}
	return nil
}

// S14: Agent stuck — busy > threshold.
func checkAgentStuck(busy []SessionInfo, now time.Time, threshold time.Duration) *Suggestion {
	for _, s := range busy {
		dur := now.Sub(s.LastActive).Minutes()
		if dur >= threshold.Minutes() {
			return &Suggestion{
				ScenarioID: "scenario_014",
				Priority:   8,
				Icon:       "⚠️",
				Text:       fmt.Sprintf("⚠️ %s 会话已持续运行 %.0f 分钟，可能卡住，建议检查。", agentDisplay(s), dur),
			}
		}
	}
	return nil
}

// S10: Attention imbalance — secondary today > primary + delta minutes.
func checkAttentionImbalance(primary *ProjectSummary, _ *ProjectSummary, projMap map[string]ProjectSummary, deltaMinutes float64) *Suggestion {
	if primary == nil {
		return nil
	}
	primaryMin := primary.TodayMinutes
	if p, ok := projMap[primary.Path]; ok {
		primaryMin = p.TodayMinutes
	}

	// Check all secondary projects
	for _, proj := range projMap {
		if proj.Priority != "secondary" {
			continue
		}
		if proj.TodayMinutes > primaryMin+deltaMinutes {
			return &Suggestion{
				ScenarioID: "scenario_010",
				Priority:   12,
				Icon:       "🔵",
				Text: fmt.Sprintf("🔵 支线【%s】今日已占用 %s，超过主线【%s】的 %s。建议调整注意力分配。",
					proj.Name, formatMinutes(int(proj.TodayMinutes)),
					primary.Name, formatMinutes(int(primaryMin))),
			}
		}
	}
	return nil
}

// S11: Normal project today > primary today.
func checkNormalExceedsPrimary(primary *ProjectSummary, projects []ProjectSummary, projMap map[string]ProjectSummary) *Suggestion {
	if primary == nil {
		return nil
	}
	primaryMin := primary.TodayMinutes
	if p, ok := projMap[primary.Path]; ok {
		primaryMin = p.TodayMinutes
	}

	for _, proj := range projects {
		if proj.Priority != "normal" || proj.Path == primary.Path {
			continue
		}
		if proj.TodayMinutes > primaryMin && proj.TodayMinutes > thresholdNormalMinMeaningful {
			return &Suggestion{
				ScenarioID: "scenario_011",
				Priority:   15,
				Icon:       "⚪",
				Text: fmt.Sprintf("⚪ 普通项目【%s】今日用时 %s 已超过主线【%s】的 %s，建议检查优先级是否合理。",
					proj.Name, formatMinutes(int(proj.TodayMinutes)),
					primary.Name, formatMinutes(int(primaryMin))),
			}
		}
	}
	return nil
}

// S13: Abnormal exit — session with mapping but dead process.
func checkAbnormalExit(sessions []SessionInfo) *Suggestion {
	for _, s := range sessions {
		if s.PID > 0 && !s.IsRunning {
			name := agentDisplay(s)
			return &Suggestion{
				ScenarioID: "scenario_013",
				Priority:   7,
				Icon:       "⚠️",
				Text:       fmt.Sprintf("⚠️ 会话【%s】已异常退出。是否重新启动？ → 运行 pflow %s 重新启动", name, s.AgentType),
			}
		}
	}
	return nil
}

// S8: All normal (positive feedback).
func checkAllNormal(primary *ProjectSummary, _ []ProjectSummary, waiting []SessionInfo, currentProject string) *Suggestion {
	if len(waiting) > 0 {
		return nil // not "all normal" if something is waiting
	}
	if primary == nil {
		return nil
	}
	if currentProject == primary.Path && len(waiting) == 0 {
		return &Suggestion{
			ScenarioID: "scenario_008",
			Priority:   20,
			Icon:       "✅",
			Text:       fmt.Sprintf("✅ 所有会话运行正常。主线【%s】正在处理中，继续保持！", primary.Name),
		}
	}
	return nil
}

// S9: Multiple sessions busy (positive).
func checkMultipleBusy(busy []SessionInfo, primary *ProjectSummary, secondary *ProjectSummary) *Suggestion {
	if len(busy) < 2 {
		return nil
	}
	if primary == nil {
		return nil
	}
	secName := ""
	if secondary != nil {
		secName = secondary.Name
	} else {
		// Find another busy project that's not primary
		for _, s := range busy {
			if s.MatchedRoot != primary.Path && s.MatchedRoot != "" {
				secName = projectNameFromPath(s.MatchedRoot)
				break
			}
		}
	}
	if secName == "" {
		return nil
	}
	return &Suggestion{
		ScenarioID: "scenario_009",
		Priority:   22,
		Icon:       "✅",
		Text:       fmt.Sprintf("✅ 主线【%s】和支线【%s】都在推进中，工作状态良好。", primary.Name, secName),
	}
}

// S15: Primary today ≥ threshold + all good (positive celebration).
// upperThresholdMinutes, if > 0, suppresses this scenario when today's
// minutes reach the higher threshold — keeps S15/S20 mutually exclusive.
func checkTodayEfficient(primary *ProjectSummary, projMap map[string]ProjectSummary, waiting []SessionInfo, thresholdMinutes float64, upperThresholdMinutes float64) *Suggestion {
	if primary == nil || len(waiting) > 0 {
		return nil
	}
	p, ok := projMap[primary.Path]
	if !ok || p.TodayMinutes < thresholdMinutes {
		return nil
	}
	// Don't fire if a higher-severity escalation (S20) covers this range
	if upperThresholdMinutes > 0 && p.TodayMinutes >= upperThresholdMinutes {
		return nil
	}
	hours := p.TodayMinutes / 60
	return &Suggestion{
		ScenarioID: "scenario_015",
		Priority:   25,
		Icon:       "🎉",
		Text:       fmt.Sprintf("🎉 主线【%s】今日已专注 %.1fh，所有会话状态良好。可以考虑休息或处理轻松任务。", primary.Name, hours),
	}
}

// S20: Primary today > 4h.
func checkPrimaryOver4h(primary *ProjectSummary, projMap map[string]ProjectSummary, thresholdMinutes float64) *Suggestion {
	if primary == nil {
		return nil
	}
	p, ok := projMap[primary.Path]
	if !ok || p.TodayMinutes < thresholdMinutes {
		return nil
	}
	hours := p.TodayMinutes / 60
	return &Suggestion{
		ScenarioID: "scenario_020",
		Priority:   30,
		Icon:       "🏆",
		Text:       fmt.Sprintf("🏆 主线【%s】今日已专注 %.1f 小时，可考虑切换到支线任务。", primary.Name, hours),
	}
}

// S6: No active sessions at all.
func checkNoActiveSessions(sessions []SessionInfo, primary *ProjectSummary) *Suggestion {
	if len(sessions) > 0 {
		// Check if ALL sessions are inactive
		hasActive := false
		for _, s := range sessions {
			if s.Status == "busy" || s.Status == "waiting" || s.Status == "idle" {
				hasActive = true
				break
			}
		}
		if hasActive {
			return nil
		}
	}

	if primary != nil {
		return &Suggestion{
			ScenarioID: "scenario_006",
			Priority:   35,
			Icon:       "⚪",
			Text:       fmt.Sprintf("⚪ 当前没有活跃的 Agent 会话。是否启动一个新任务？ → 建议启动主线【%s】", primary.Name),
		}
	}
	return &Suggestion{
		ScenarioID: "scenario_006",
		Priority:   35,
		Icon:       "⚪",
		Text:       "⚪ 当前没有活跃的 Agent 会话。是否启动一个新任务？",
	}
}

// S7: All sessions idle > 10 min.
func checkAllIdle(sessions []SessionInfo, now time.Time, threshold time.Duration, totalCount int) *Suggestion {
	if totalCount == 0 {
		return nil
	}
	for _, s := range sessions {
		if s.Status != "idle" && s.Status != "unknown" {
			return nil // at least one session is busy or waiting
		}
		if now.Sub(s.LastActive) < threshold {
			return nil // at least one session was recently active
		}
	}
	return &Suggestion{
		ScenarioID: "scenario_007",
		Priority:   36,
		Icon:       "⚪",
		Text:       "⚪ 所有会话已空闲超过 10 分钟。建议启动一个任务开始工作。",
	}
}

// S18: 3+ projects concurrently active.
func checkMultiProject(activeProjects []ProjectSummary, primary *ProjectSummary) *Suggestion {
	if len(activeProjects) < 3 {
		return nil
	}
	primaryName := ""
	if primary != nil {
		primaryName = primary.Name
	}
	return &Suggestion{
		ScenarioID: "scenario_018",
		Priority:   40,
		Icon:       "📊",
		Text:       fmt.Sprintf("📊 %d 个项目同时推进中，注意主线【%s】的时间占比。", len(activeProjects), primaryName),
	}
}

// S19: Global idle > 30 min (user away).
func checkGlobalIdle(projects []ProjectSummary, _ time.Time, threshold time.Duration) *Suggestion {
	if len(projects) == 0 {
		return nil
	}
	// Check if ALL projects have been idle > threshold
	allIdle := true
	for _, p := range projects {
		if p.IdleMinutes < threshold.Minutes() {
			allIdle = false
			break
		}
	}
	if !allIdle {
		return nil
	}
	return &Suggestion{
		ScenarioID: "scenario_019",
		Priority:   45,
		Icon:       "⏰",
		Text:       "⏰ 已空闲较长时间，是否有阻塞问题需要解决？",
	}
}

// S16: End of day reminder (after 18:00).
func checkEndOfDay(now time.Time) *Suggestion {
	hour := now.Hour()
	if hour >= 18 && hour < 22 {
		return &Suggestion{
			ScenarioID: "scenario_016",
			Priority:   50,
			Icon:       "🌆",
			Text:       "🌆 今日工作即将结束，建议总结今日进展。",
		}
	}
	return nil
}

// S17: After lunch (13:00-14:00).
func checkAfterLunch(now time.Time) *Suggestion {
	hour, min := now.Hour(), now.Minute()
	t := hour*60 + min
	if t >= 13*60 && t < 14*60 {
		return &Suggestion{
			ScenarioID: "scenario_017",
			Priority:   55,
			Icon:       "🌤️",
			Text:       "🌤️ 午休结束，建议检查各会话状态。",
		}
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────

func appendIf(out []Suggestion, s *Suggestion) []Suggestion {
	if s != nil {
		return append(out, *s)
	}
	return out
}

func findProject(projects []ProjectSummary, priority string) *ProjectSummary {
	for i := range projects {
		if projects[i].Priority == priority {
			return &projects[i]
		}
	}
	return nil
}

func filterByStatus(sessions []SessionInfo, status string) []SessionInfo {
	var out []SessionInfo
	for _, s := range sessions {
		if s.Status == status {
			out = append(out, s)
		}
	}
	return out
}

func filterActiveProjects(projects []ProjectSummary) []ProjectSummary {
	var out []ProjectSummary
	for _, p := range projects {
		if p.BusyCount > 0 || p.WaitingCount > 0 {
			out = append(out, p)
		}
	}
	return out
}

func hasWaitingOnProject(waiting []SessionInfo, projectPath string) bool {
	for _, s := range waiting {
		if s.MatchedRoot == projectPath {
			return true
		}
	}
	return false
}

func agentDisplay(s SessionInfo) string {
	if s.AgentName != "" {
		return s.AgentName
	}
	switch s.AgentType {
	case "claude":
		return "Claude"
	case "hermes":
		return "Hermes"
	default:
		return s.AgentType
	}
}

func projectLabel(priority string) string {
	switch priority {
	case "primary":
		return "主线任务【"
	case "secondary":
		return "支线任务【"
	default:
		return ""
	}
}

func projectNameFromPath(path string) string {
	if path == "" {
		return "未知"
	}
	return filepath.Base(path)
}

func formatMinutes(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	h := minutes / 60
	m := minutes % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

// deduplicate removes suggestions whose text body (icon prefix stripped) is
// identical to a higher-priority suggestion already in the list. The input
// must already be sorted by priority (ascending). Different projects
// naturally produce different text bodies because the project name is
// embedded in the text.
func deduplicate(in []Suggestion) []Suggestion {
	if len(in) <= 1 {
		return in
	}
	seen := make(map[string]bool, len(in))
	out := make([]Suggestion, 0, len(in))
	for _, s := range in {
		body := textBody(s.Text)
		if seen[body] {
			continue
		}
		seen[body] = true
		out = append(out, s)
	}
	return out
}

// textBody returns the suggestion text with the leading icon+space prefix
// stripped. All suggestion texts follow the pattern "emoji body...".
func textBody(text string) string {
	runes := []rune(text)
	if len(runes) >= 2 && runes[1] == ' ' {
		return string(runes[2:])
	}
	return text
}

// ComputeProjectSummaries builds ProjectSummary from sessions and project roots.
// If focusLog is non-nil and has data for a project, its TodayMinutes override
// the per-session estimates (tmux focus events take highest priority in the
// degradation chain). This is the primary data transformation function used by
// the CLI layer.
func ComputeProjectSummaries(sessions []SessionInfo, now time.Time, focusLog *timetrack.FocusLog) []ProjectSummary {
	type agg struct {
		path         string
		priority     string
		name         string
		todayMinutes float64
		lastActive   time.Time
		sessionCount int
		busyCount    int
		waitingCount int
		idleCount    int
	}

	groups := make(map[string]*agg)
	var order []string
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, s := range sessions {
		key := s.MatchedRoot
		if key == "" {
			key = s.ProjectPath
		}
		if key == "" {
			continue
		}

		if _, ok := groups[key]; !ok {
			groups[key] = &agg{
				path:     key,
				priority: s.RootPriority,
				name:     projectNameFromPath(key),
			}
			order = append(order, key)
		}
		g := groups[key]

		// Use session's priority if it's higher-priority than what we have
		if g.priority == "" || g.priority == "normal" {
			if s.RootPriority == "primary" || s.RootPriority == "secondary" {
				g.priority = s.RootPriority
			}
		}

		g.sessionCount++
		switch s.Status {
		case "busy":
			g.busyCount++
		case "waiting":
			g.waitingCount++
		case "idle":
			g.idleCount++
		}

		if s.LastActive.After(g.lastActive) {
			g.lastActive = s.LastActive
		}

		// Compute today's usage contribution using time estimation
		// (msg count × 3 min, or wall-clock × 0.3 fallback)
		g.todayMinutes += timetrack.SessionTodayMinutes(
			s.MessageCount, s.FirstActive, s.LastActive, todayStart, now)
	}

	result := make([]ProjectSummary, 0, len(order))
	for _, key := range order {
		g := groups[key]
		idleMin := 0.0
		if !g.lastActive.IsZero() {
			idleMin = now.Sub(g.lastActive).Minutes()
		}
		result = append(result, ProjectSummary{
			Path:         g.path,
			Priority:     g.priority,
			Name:         g.name,
			TodayMinutes: g.todayMinutes,
			LastActive:   g.lastActive,
			SessionCount: g.sessionCount,
			BusyCount:    g.busyCount,
			WaitingCount: g.waitingCount,
			IdleCount:    g.idleCount,
			IdleMinutes:  idleMin,
		})
	}


	// Tier 1: override with tmux focus events if available.
	// Focus log has higher precision than per-session message estimates.
	if focusLog != nil {
		for i := range result {
			if fm := focusLog.ProjectMinutes(result[i].Path, todayStart, now); fm > 0 {
				result[i].TodayMinutes = fm
			}
		}
	}
	return result
}
