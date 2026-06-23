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
	Icon     string // emoji indicator
	Text     string // formatted text (may contain newlines)
	Priority int    // lower = more urgent, used for stable sort
}

// Input is the complete dataset needed for suggestion analysis.
type Input struct {
	Sessions       []SessionInfo
	Projects       []ProjectSummary
	CurrentProject string // project root path the user is currently viewing
	Now            time.Time
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

	// S3: 🟡 primary idle > 30 min (no waiting)
	out = appendIf(out, checkPrimaryIdle(primary, projMap, input.Now, 30*time.Minute, waitingSessions, "🟡"))

	// S4: 🔶 primary idle > 60 min
	out = appendIf(out, checkPrimaryIdle(primary, projMap, input.Now, 60*time.Minute, waitingSessions, "🔶"))

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

	// S15: 🎉 primary today ≥ 20 msgs (~60 min) + all good
	out = appendIf(out, checkTodayEfficient(primary, projMap, waitingSessions, thresholdEfficientMinutes))

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

	return out
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
				Priority: 1,
				Icon:     icon,
				Text:     fmt.Sprintf("%s %s %s会话已等待 %.0f 分钟，%s。\n", icon, projLabel, name, waitMin, hint),
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
					Priority: 3,
					Icon:     "🟡",
					Text:     fmt.Sprintf("🟡 主线任务【%s】的 %s 会话已等待 %.0f 分钟，建议尽快处理。", primary.Name, agentDisplay(s), waitMin),
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
		Priority: 2,
		Icon:     "🔴",
		Text: fmt.Sprintf("🔴 多个会话需要授权：%s【%s】等待 %.0fm，%s【%s】等待 %.0fm。建议先处理主线。",
			mainLabel, agentDisplay(s1), wait1,
			secLabel, agentDisplay(s2), wait2),
	}
}

// S3/S4: Primary idle > threshold (no waiting sessions on primary).
func checkPrimaryIdle(primary *ProjectSummary, projMap map[string]ProjectSummary, _ time.Time, threshold time.Duration, waiting []SessionInfo, icon string) *Suggestion {
	if primary == nil {
		return nil
	}
	p, ok := projMap[primary.Path]
	if !ok || p.IdleMinutes < threshold.Minutes() {
		return nil
	}
	// Don't fire if primary has waiting sessions (S1/S2 handle that)
	if hasWaitingOnProject(waiting, primary.Path) {
		return nil
	}
	idleMin := p.IdleMinutes
	if idleMin >= 60 {
		return &Suggestion{
			Priority: 4,
			Icon:     icon,
			Text:     fmt.Sprintf("%s 主线任务【%s】已空闲超过 1 小时，是否应该切换回来？", icon, primary.Name),
		}
	}
	return &Suggestion{
		Priority: 5,
		Icon:     icon,
		Text:     fmt.Sprintf("%s 主线任务【%s】已空闲 %.0f 分钟，建议回到主线继续工作。", icon, primary.Name, idleMin),
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
				Priority: 10,
				Icon:     "🔵",
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
				Priority: 8,
				Icon:     "⚠️",
				Text:     fmt.Sprintf("⚠️ %s 会话已持续运行 %.0f 分钟，可能卡住，建议检查。", agentDisplay(s), dur),
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
				Priority: 12,
				Icon:     "🔵",
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
				Priority: 15,
				Icon:     "⚪",
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
				Priority: 7,
				Icon:     "⚠️",
				Text:     fmt.Sprintf("⚠️ 会话【%s】已异常退出。是否重新启动？ → 运行 pflow %s 重新启动", name, s.AgentType),
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
			Priority: 20,
			Icon:     "✅",
			Text:     fmt.Sprintf("✅ 所有会话运行正常。主线【%s】正在处理中，继续保持！", primary.Name),
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
		Priority: 22,
		Icon:     "✅",
		Text:     fmt.Sprintf("✅ 主线【%s】和支线【%s】都在推进中，工作状态良好。", primary.Name, secName),
	}
}

// S15: Primary today > 2h + all good (positive celebration).
func checkTodayEfficient(primary *ProjectSummary, projMap map[string]ProjectSummary, waiting []SessionInfo, thresholdMinutes float64) *Suggestion {
	if primary == nil || len(waiting) > 0 {
		return nil
	}
	p, ok := projMap[primary.Path]
	if !ok || p.TodayMinutes < thresholdMinutes {
		return nil
	}
	hours := p.TodayMinutes / 60
	return &Suggestion{
		Priority: 25,
		Icon:     "🎉",
		Text:     fmt.Sprintf("🎉 主线【%s】今日已专注 %.1fh，所有会话状态良好。可以考虑休息或处理轻松任务。", primary.Name, hours),
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
		Priority: 30,
		Icon:     "🏆",
		Text:     fmt.Sprintf("🏆 主线【%s】今日已专注 %.1f 小时，可考虑切换到支线任务。", primary.Name, hours),
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
			Priority: 35,
			Icon:     "⚪",
			Text:     fmt.Sprintf("⚪ 当前没有活跃的 Agent 会话。是否启动一个新任务？ → 建议启动主线【%s】", primary.Name),
		}
	}
	return &Suggestion{
		Priority: 35,
		Icon:     "⚪",
		Text:     "⚪ 当前没有活跃的 Agent 会话。是否启动一个新任务？",
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
		Priority: 36,
		Icon:     "⚪",
		Text:     "⚪ 所有会话已空闲超过 10 分钟。建议启动一个任务开始工作。",
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
		Priority: 40,
		Icon:     "📊",
		Text:     fmt.Sprintf("📊 %d 个项目同时推进中，注意主线【%s】的时间占比。", len(activeProjects), primaryName),
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
		Priority: 45,
		Icon:     "⏰",
		Text:     "⏰ 已空闲较长时间，是否有阻塞问题需要解决？",
	}
}

// S16: End of day reminder (after 18:00).
func checkEndOfDay(now time.Time) *Suggestion {
	hour := now.Hour()
	if hour >= 18 && hour < 22 {
		return &Suggestion{
			Priority: 50,
			Icon:     "🌆",
			Text:     "🌆 今日工作即将结束，建议总结今日进展。",
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
			Priority: 55,
			Icon:     "🌤️",
			Text:     "🌤️ 午休结束，建议检查各会话状态。",
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
