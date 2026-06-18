package attention

import (
	"math"
	"testing"
	"time"
)

// =============================================================================
// Test Configuration
// =============================================================================

// TOLERANCE is the allowed absolute deviation for 0-100 scale values.
// Increase this if rounding differences are acceptable, decrease for stricter checks.
// Default: 5 (values within ±5 of expected are considered passing).
const TOLERANCE = 5.0

// =============================================================================
// Test Time Fixtures
// =============================================================================

var testNow = time.Date(2026, 6, 15, 14, 0, 0, 0, time.Local)

// "recent" timestamps are within CurWindow (60min)
var r1 = testNow.Add(-1 * time.Minute)  // 1min ago — becomes cur in most tests
var r2 = testNow.Add(-2 * time.Minute)
var r3 = testNow.Add(-3 * time.Minute)
var r4 = testNow.Add(-4 * time.Minute)
var r5 = testNow.Add(-5 * time.Minute)
var r8 = testNow.Add(-8 * time.Minute)
var r10 = testNow.Add(-10 * time.Minute)
var r20 = testNow.Add(-20 * time.Minute)

// "old" timestamps are outside CurWindow (>60min)
var o70 = testNow.Add(-70 * time.Minute)
var o80 = testNow.Add(-80 * time.Minute)
var o90 = testNow.Add(-90 * time.Minute)
var o95 = testNow.Add(-95 * time.Minute)

// =============================================================================
// Helpers
// =============================================================================

// checkHL verifies highlight value is within TOLERANCE of expected.
func checkHL(t *testing.T, got int, want int, label string) {
	t.Helper()
	if math.Abs(float64(got-want)) > TOLERANCE {
		t.Errorf("%s Highlight: got %d, want %d (diff=%d, tolerance=%.0f)",
			label, got, want, got-want, TOLERANCE)
	}
}

// checkFog verifies fog% value is within TOLERANCE of expected.
func checkFog(t *testing.T, got int, want int, label string) {
	t.Helper()
	if math.Abs(float64(got-want)) > TOLERANCE {
		t.Errorf("%s FogPct: got %d, want %d (diff=%d, tolerance=%.0f)",
			label, got, want, got-want, TOLERANCE)
	}
}

// checkCur verifies IsCurrent flag.
func checkCur(t *testing.T, got bool, want bool, label string) {
	t.Helper()
	if got != want {
		t.Errorf("%s IsCurrent: got %v, want %v", label, got, want)
	}
}

// checkRaw verifies raw score within a small tolerance.
func checkRaw(t *testing.T, got float64, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 0.02 {
		t.Errorf("%s Score(raw): got %.2f, want %.2f", label, got, want)
	}
}

// checkRawFog verifies raw fog score within a small tolerance.
func checkRawFog(t *testing.T, got float64, want float64, label string) {
	t.Helper()
	if math.Abs(got-want) > 0.002 {
		t.Errorf("%s FogScore(raw): got %.3f, want %.3f", label, got, want)
	}
}

// =============================================================================
// TC01: 主线专注 1h → 支线 waiting 30min 被提醒（无 focus）
// =============================================================================
func TestTC01_PrimaryFocus1h_SecondaryWaiting30m(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 60, Total: 60, LastActive: r1, IsPrimary: true},
		"S1": {Waiting: 30, Streak: 0, Total: 20, LastActive: r5, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	// P: cur → highlight=0, fog=0
	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")
	checkCur(t, p.IsCurrent, true, "P")

	// S1: raw=3600 → hl=89, fog=0
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 89, "S1")
	checkFog(t, s1.FogPct, 0, "S1")
	checkCur(t, s1.IsCurrent, false, "S1")
	checkRaw(t, s1.Score, 3600, "S1")
}

// =============================================================================
// TC02: 主线 5min + 支线 waiting 45min（无 focus）
// =============================================================================
func TestTC02_Primary5m_SecondaryWaiting45m(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 5, Total: 5, LastActive: r1, IsPrimary: true},
		"S1": {Waiting: 45, Streak: 0, Total: 30, LastActive: r3, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1: raw=506 → hl=68
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 68, "S1")
	checkFog(t, s1.FogPct, 0, "S1")
}

// =============================================================================
// TC03: 主线 3min 无 focus + 支线 waiting 30min
// =============================================================================
func TestTC03_Primary3m_SecondaryWaiting30m(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 3, Total: 3, LastActive: r1, IsPrimary: true},
		"S1": {Waiting: 30, Streak: 0, Total: 0, LastActive: r2, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1: raw=81 → hl=48
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 48, "S1")
	checkFog(t, s1.FogPct, 0, "S1")
}

// =============================================================================
// TC04: 显式专注模式，保护期内（focus=on, streak=3, target=15）
// =============================================================================
func TestTC04_FocusProtectionActive(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 3, Total: 3, LastActive: r1, IsPrimary: true},
		"S1": {Waiting: 30, Streak: 0, Total: 0, LastActive: r2, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, true, "", 15)

	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1: protection → hl=0, fog=0.82 → 82
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 0, "S1")
	checkFog(t, s1.FogPct, 82, "S1")
}

// =============================================================================
// TC05: 显式专注模式，已过保护期（focus=on, streak=20, target=15）
// =============================================================================
func TestTC05_FocusProtectionExpired(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 20, Total: 20, LastActive: r1, IsPrimary: true},
		"S1": {Waiting: 30, Streak: 0, Total: 10, LastActive: r3, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, true, "", 15)

	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1: protection expired → raw=3600, hl=89
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 89, "S1")
	checkFog(t, s1.FogPct, 0, "S1")
}

// =============================================================================
// TC06: 支线活跃 → 主线 waiting 获 PrimaryBonus（含今日校正）
// =============================================================================
// NOTE: today_correction applies because primary_total(10) < sec_median(40).
// Without correction: raw=10000→hl=100. With correction: raw=13225→hl=100.
func TestTC06_SecondaryActive_PrimaryBonus(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 25, Streak: 0, Total: 10, LastActive: r2, IsPrimary: true},
		"S1": {Waiting: 0, Streak: 20, Total: 40, LastActive: r1, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	// P: cur=sec, target=pri → bonus, raw=13225 → hl=100 (capped)
	p := result["P"]
	checkHL(t, p.Highlight, 100, "P")
	checkFog(t, p.FogPct, 0, "P")
	checkCur(t, p.IsCurrent, false, "P")

	// S1: cur → hl=0
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 0, "S1")
	checkFog(t, s1.FogPct, 0, "S1")
	checkCur(t, s1.IsCurrent, true, "S1")
}

// =============================================================================
// TC07: 今日累计校正 — 支线远超主线
// =============================================================================
func TestTC07_TodayCorrection_MultipleSecondaries(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 5, Streak: 0, Total: 10, LastActive: r2, IsPrimary: true},
		"S1": {Waiting: 0, Streak: 20, Total: 60, LastActive: r1, IsPrimary: false},
		"S2": {Waiting: 3, Streak: 0, Total: 40, LastActive: r3, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	// P: raw=1600 → hl=80
	p := result["P"]
	checkHL(t, p.Highlight, 80, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1: cur → hl=0
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 0, "S1")
	checkFog(t, s1.FogPct, 0, "S1")

	// S2: raw=36 → hl=39, raw_fog=0.293 → 29
	s2 := result["S2"]
	checkHL(t, s2.Highlight, 39, "S2")
	checkFog(t, s2.FogPct, 29, "S2")
}

// =============================================================================
// TC08: 无当前活跃任务（含今日校正：primary_total=5 < sec_median=20）
// =============================================================================
func TestTC08_NoCurrentProject(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 8, Streak: 0, Total: 5, LastActive: o70, IsPrimary: true},
		"S1": {Waiting: 12, Streak: 0, Total: 20, LastActive: o80, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	// P: raw=240 → hl=60, no cur+primary → fog=0
	p := result["P"]
	checkHL(t, p.Highlight, 60, "P")
	checkFog(t, p.FogPct, 0, "P")
	checkCur(t, p.IsCurrent, false, "P")

	// S1: raw=0 → hl=0, no cur+not primary → fog=0.7 → 70
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 0, "S1")
	checkFog(t, s1.FogPct, 70, "S1")
}

// =============================================================================
// TC09: MinFactor 兜底（streak 不可测量，无 floor 触发）
// =============================================================================
func TestTC09_MinFactorFallback(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 0, Total: 0, LastActive: r20, IsPrimary: true},
		"S1": {Waiting: 120, Streak: 0, Total: 0, LastActive: r10, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	// S1 is cur (most recent at 10min ago), sinceRecent=10 >= 5 → no floor
	s1 := result["S1"]
	checkCur(t, s1.IsCurrent, true, "S1")
	checkHL(t, s1.Highlight, 0, "S1")
	checkFog(t, s1.FogPct, 0, "S1")

	// P: all zeros, primary, maxReminder=0 → fog=0 (primary is clear when no competition)
	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")
}

// =============================================================================
// TC10: Streak floor 触发（recent activity 4min ago, streak unmeasured）
// =============================================================================
func TestTC10_StreakFloorTriggered(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 0, Total: 5, LastActive: r4, IsPrimary: true},
		"S1": {Waiting: 120, Streak: 0, Total: 0, LastActive: r10, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	// P is cur (most recent at 4min ago), floor → streakCur=5
	p := result["P"]
	checkCur(t, p.IsCurrent, true, "P")
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1: raw=3600 → hl=89
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 89, "S1")
	checkFog(t, s1.FogPct, 0, "S1")
}

// =============================================================================
// TC11: 幂函数差异化（waiting 5 vs 10 → 100 vs 400）
// =============================================================================
func TestTC11_PowerFunctionDifferentiation(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":   {Waiting: 0, Streak: 30, Total: 30, LastActive: r1, IsPrimary: true},
		"S5":  {Waiting: 5, Streak: 0, Total: 0, LastActive: r2, IsPrimary: false},
		"S10": {Waiting: 10, Streak: 0, Total: 0, LastActive: r3, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S5: raw=100 → hl=50, fog=0.225 → 23
	s5 := result["S5"]
	checkHL(t, s5.Highlight, 50, "S5")
	checkFog(t, s5.FogPct, 23, "S5")

	// S10: raw=400 → hl=65, fog=0
	s10 := result["S10"]
	checkHL(t, s10.Highlight, 65, "S10")
	checkFog(t, s10.FogPct, 0, "S10")
}

// =============================================================================
// TC12: Streak cap（超长专注 180min 不会过度放大）
// =============================================================================
func TestTC12_StreakCap(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 180, Total: 180, LastActive: r1, IsPrimary: true},
		"S1": {Waiting: 10, Streak: 0, Total: 5, LastActive: r2, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1: streak capped at 2.0 → raw=400, hl=65
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 65, "S1")
	checkFog(t, s1.FogPct, 0, "S1")
}

// =============================================================================
// TC13: 单项目 — cur 永远无高亮无迷雾
// =============================================================================
func TestTC13_SingleProject(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P": {Waiting: 5, Streak: 10, Total: 10, LastActive: r1, IsPrimary: true},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	p := result["P"]
	checkCur(t, p.IsCurrent, true, "P")
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")
}

// =============================================================================
// TC14: 三路支线竞争（waiting 45/15/3）
// =============================================================================
func TestTC14_ThreeSecondariesCompeting(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":    {Waiting: 0, Streak: 30, Total: 30, LastActive: r1, IsPrimary: true},
		"S_急": {Waiting: 45, Streak: 0, Total: 10, LastActive: r5, IsPrimary: false},
		"S_中": {Waiting: 15, Streak: 0, Total: 25, LastActive: r8, IsPrimary: false},
		"S_缓": {Waiting: 3, Streak: 0, Total: 5, LastActive: r10, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S_急: raw=8100 → hl=98, fog=0
	checkHL(t, result["S_急"].Highlight, 98, "S_急")
	checkFog(t, result["S_急"].FogPct, 0, "S_急")

	// S_中: raw=900 → hl=74, fog=0.267 → 27
	checkHL(t, result["S_中"].Highlight, 74, "S_中")
	checkFog(t, result["S_中"].FogPct, 27, "S_中")

	// S_缓: raw=36 → hl=39, fog=0.299 → 30
	checkHL(t, result["S_缓"].Highlight, 39, "S_缓")
	checkFog(t, result["S_缓"].FogPct, 30, "S_缓")
}

// =============================================================================
// TC15: 空输入
// =============================================================================
func TestTC15_EmptyInput(t *testing.T) {
	if result := CalculateScores(nil, testNow, false, "", 0); result != nil {
		t.Error("expected nil for nil input")
	}
	if result := CalculateScores(map[string]ReminderInput{}, testNow, false, "", 0); result != nil {
		t.Error("expected nil for empty map")
	}
}

// =============================================================================
// TC16: cur=nil + 今日校正
// =============================================================================
func TestTC16_NoCurWithTodayCorrection(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 5, Streak: 0, Total: 10, LastActive: o80, IsPrimary: true},
		"S1": {Waiting: 0, Streak: 0, Total: 60, LastActive: o90, IsPrimary: false},
		"S2": {Waiting: 0, Streak: 0, Total: 40, LastActive: o95, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	// P: raw=625 → hl=70, no cur+primary → fog=0
	p := result["P"]
	checkHL(t, p.Highlight, 70, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1, S2: no cur+not primary → fog=0.7 → 70
	checkHL(t, result["S1"].Highlight, 0, "S1")
	checkFog(t, result["S1"].FogPct, 70, "S1")
	checkHL(t, result["S2"].Highlight, 0, "S2")
	checkFog(t, result["S2"].FogPct, 70, "S2")
}

// =============================================================================
// TC17: 双主线，cur=主线时无 PrimaryBonus
// =============================================================================
func TestTC17_DualPrimary_NoBonus(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P1": {Waiting: 0, Streak: 20, Total: 20, LastActive: r1, IsPrimary: true},
		"P2": {Waiting: 15, Streak: 0, Total: 5, LastActive: r3, IsPrimary: true},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	// P1: cur → hl=0
	p1 := result["P1"]
	checkCur(t, p1.IsCurrent, true, "P1")
	checkHL(t, p1.Highlight, 0, "P1")
	checkFog(t, p1.FogPct, 0, "P1")

	// P2: raw=900 → hl=74, no bonus (cur is primary)
	p2 := result["P2"]
	checkHL(t, p2.Highlight, 74, "P2")
	checkFog(t, p2.FogPct, 0, "P2")
}

// =============================================================================
// TC18: 支线活跃+多主线 bonus（含今日校正）
// =============================================================================
func TestTC18_SecondaryCur_MultiplePrimariesWithBonus(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P1": {Waiting: 10, Streak: 0, Total: 5, LastActive: r2, IsPrimary: true},
		"P2": {Waiting: 5, Streak: 0, Total: 3, LastActive: r3, IsPrimary: true},
		"S1": {Waiting: 0, Streak: 20, Total: 30, LastActive: r1, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	// S1: cur → hl=0
	s1 := result["S1"]
	checkCur(t, s1.IsCurrent, true, "S1")
	checkHL(t, s1.Highlight, 0, "S1")
	checkFog(t, s1.FogPct, 0, "S1")

	// P1: bonus+correction → raw=2756 → hl=86
	p1 := result["P1"]
	checkHL(t, p1.Highlight, 86, "P1")
	checkFog(t, p1.FogPct, 0, "P1")

	// P2: bonus+correction → raw=1122 → hl=76, fog=0.178 → 18
	p2 := result["P2"]
	checkHL(t, p2.Highlight, 76, "P2")
	checkFog(t, p2.FogPct, 18, "P2")
}

// =============================================================================
// TC19: focus 保护 + zero measured streak（protect fog=0.9）
// =============================================================================
func TestTC19_FocusProtection_ZeroMeasuredStreak(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 0, Total: 0, LastActive: r1, IsPrimary: true},
		"S1": {Waiting: 50, Streak: 0, Total: 0, LastActive: r2, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, true, "", 15)

	p := result["P"]
	checkCur(t, p.IsCurrent, true, "P")
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1: protection (streak=0 < 15), fog=0.9 → 90
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 0, "S1")
	checkFog(t, s1.FogPct, 90, "S1")
}

// =============================================================================
// TC20: Fog 保护期渐近（streak=14, focus=15 → remain≈0.07 → fog≈0.527）
// =============================================================================
func TestTC20_FogProtectGradient(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 14, Total: 14, LastActive: r1, IsPrimary: true},
		"S1": {Waiting: 30, Streak: 0, Total: 0, LastActive: r2, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, true, "", 15)

	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1: fog=0.527 → 53
	s1 := result["S1"]
	checkHL(t, s1.Highlight, 0, "S1")
	checkFog(t, s1.FogPct, 53, "S1")
}

// =============================================================================
// TC21: 全零 reminder → 所有非 cur 项目 fog=FogBaseNonProtect=0.3
// =============================================================================
func TestTC21_AllZeroReminders(t *testing.T) {
	inputs := map[string]ReminderInput{
		"P":  {Waiting: 0, Streak: 30, Total: 30, LastActive: r1, IsPrimary: true},
		"S1": {Waiting: 0, Streak: 0, Total: 10, LastActive: r5, IsPrimary: false},
		"S2": {Waiting: 0, Streak: 0, Total: 5, LastActive: r10, IsPrimary: false},
	}

	result := CalculateScores(inputs, testNow, false, "", 0)

	p := result["P"]
	checkHL(t, p.Highlight, 0, "P")
	checkFog(t, p.FogPct, 0, "P")

	// S1, S2: maxRem=0 → fog=0.3 → 30
	checkHL(t, result["S1"].Highlight, 0, "S1")
	checkFog(t, result["S1"].FogPct, 30, "S1")
	checkHL(t, result["S2"].Highlight, 0, "S2")
	checkFog(t, result["S2"].FogPct, 30, "S2")
}

// =============================================================================
// TC22: 验证 Highlight 映射表（score → 0-100 对照）
// =============================================================================
func TestTC22_HighlightMappingTable(t *testing.T) {
	tests := []struct {
		score float64
		want  int
	}{
		{0, 0},
		{2, 12},
		{5, 19},
		{10, 26},
		{25, 35},
		{50, 43},
		{100, 50},
		{200, 58},
		{400, 65},
		{900, 74},
		{1600, 80},
		{3600, 89},
		{5000, 92},
		{8100, 98},
		{10000, 100}, // capped at 100
	}
	for _, tt := range tests {
		got := reminderToHighlight(tt.score)
		if math.Abs(float64(got-tt.want)) > TOLERANCE {
			t.Errorf("reminderToHighlight(%.0f): got %d, want %d", tt.score, got, tt.want)
		}
	}
}
