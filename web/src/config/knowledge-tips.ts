// Knowledge tips for the KnowledgeAnchor component.
// Used for general rotation when no suggestion has an associated knowledge_tip.
// See docs/design/10-tips.md §4 for the full content reference.

export interface KnowledgeTipData {
  id: string
  title: string
  theory: string
  design: string
  related_scenarios: string[]
}

export const ALL_TIPS: KnowledgeTipData[] = [
  {
    id: 'kt_attention_residue',
    title: '注意力残留与任务切换成本',
    theory: '从任务 A 切换到 B 时，注意力不会完全转移——仍有部分「残留」在 A 上，影响对新任务的投入（注意力残留，Leroy, 2009）。任务切换本身也伴随真实的认知成本（switch cost，Monsell, 2003）。',
    design: '主线切换后设 15 分钟保护期不推送干扰——给注意力留出「脱离旧任务、投入新任务」的过渡时间。',
    related_scenarios: ['scenario_001', 'scenario_002', 'scenario_003', 'scenario_004', 'scenario_012'],
  },
  {
    id: 'kt_cognitive_offloading',
    title: '认知卸载的双面性',
    theory: '「替代性卸载」（AI 替你做判断）可能让相关能力因缺少练习而退化；「互补性卸载」（AI 替你记忆，你做判断）则保留你的判断力（Risko & Gilbert, 2016）。',
    design: 'pflow 只展示状态，永远不替你做「切不切」的决策——把判断留给你，把监控交给系统。',
    related_scenarios: ['scenario_008', 'scenario_009', 'scenario_015', 'scenario_018'],
  },
  {
    id: 'kt_metacognitive_bottleneck',
    title: '元认知监控的代价',
    theory: '边工作边监控自己「有没有分心」几乎不可能——自我监控与任务本身争夺同一份有限的注意力（双任务干扰，Pashler, 1994）。',
    design: '遮罩层和红绿灯把「监控」变成一眼可读的视觉信号——扫一眼就知道全局，不必在脑内另开一条监控线程。',
    related_scenarios: ['scenario_010', 'scenario_011', 'scenario_006'],
  },
  {
    id: 'kt_embodied_cognition',
    title: '物理环境即思维外挂',
    theory: '物理环境（屏幕布局、光标位置）是思维的「外挂」——把状态放在眼前，比记在脑里更省力（延展心智，Clark & Chalmers, 1998）。',
    design: 'pflow 不内嵌终端，就是让你用「物理性切换窗口」的动作重置大脑上下文——用身体动作完成「上下文切换」。',
    related_scenarios: ['scenario_006', 'scenario_007', 'scenario_019'],
  },
  {
    id: 'kt_interruption_recovery',
    title: '中断恢复的代价',
    theory: '被中断后，平均需要约 23 分钟才能回到原有的深度工作状态（Mark 等, 2008）。中断越频繁，有效深度工作时间越短。',
    design: '提醒分数用「幂函数放大」机制，避免多项目同时高亮——只有远超阈值的才推送通知。',
    related_scenarios: ['scenario_001', 'scenario_002', 'scenario_003', 'scenario_004'],
  },
  {
    id: 'kt_prediction_error',
    title: '预测误差与判断信心',
    theory: '自己思考得出答案时，大脑会产生奖励信号；AI 直接给出答案时，这个信号不会产生。长期依赖现成答案，独立判断可能缺少锻炼。',
    design: 'Agent 持续运行超 10 分钟时提醒「可能卡住，建议检查」——让你重新介入判断，保持异常监测敏感度。',
    related_scenarios: ['scenario_014'],
  },
  {
    id: 'kt_primary_secondary_strategy',
    title: '为何强制设定 1 个主线',
    theory: '工作记忆容量有限（约 4 个组块），超出容量的多线程管理本身就是认知负担。',
    design: '强制设定 1 主线 + 最多 2 支线，把「排序」决策前置化，避免工作过程中反复纠结优先级。',
    related_scenarios: ['scenario_010', 'scenario_011', 'scenario_018'],
  },
  {
    id: 'kt_multitasking_illusion',
    title: '多任务只是快速切换的幻觉',
    theory: '大脑本质上是串行处理器——所谓的「并行」只是极速切换的幻觉。每次切换都有能耗。',
    design: '红绿灯状态撕掉「并行」伪装：此刻只有一个 🟡 的会话在占用你的思考排队名额。',
    related_scenarios: ['scenario_009', 'scenario_012', 'scenario_018'],
  },
  {
    id: 'kt_chunking',
    title: '组块化认知与零归类设计',
    theory: '长时记忆依靠「组块」压缩信息——把零散信息打包成有意义的单元，是大脑处理复杂信息的核心机制。',
    design: '「路径即项目」——用目录路径作为天然分组依据，避免手动维护分组的心力消耗。',
    related_scenarios: [],
  },
  {
    id: 'kt_positive_feedback',
    title: '正向反馈驱动持续专注',
    theory: '多巴胺系统在获得正向反馈时被激活，能增强持续专注的动机。小胜利的记录比大目标更能维持日常动力。',
    design: '一切正常或高效完成时给出 ✅ / 🎉 级正向反馈——不是空话，是对大脑奖励机制的调用。',
    related_scenarios: ['scenario_008', 'scenario_015'],
  },
  {
    id: 'kt_decision_fatigue',
    title: '决策疲劳与调度前置',
    theory: '每做一个决策都消耗认知资源。决策次数累积后，后续决策质量会下降——这就是决策疲劳。',
    design: '把「切不切」的判断前置到策略设定阶段，执行阶段只需看状态、按计划行动，大幅减少执行中的决策次数。',
    related_scenarios: ['scenario_006'],
  },
  {
    id: 'kt_offloading_boundary',
    title: '卸载的边界：保留不可让渡的阵地',
    theory: '价值观判断、审美选择、生死攸关的直觉——这些领域一旦过度卸载，相关能力可能明显退化。卸载不是越彻底越好。',
    design: 'pflow 帮你「记住状态」，但永远不替你「做出选择」。你是统帅，兵权不可让渡。',
    related_scenarios: [],
  },
]

// Generic tips are those without specific scenario associations.
// They are used for the default auto-rotation state.
export const GENERIC_TIPS = ALL_TIPS.filter((t) => t.related_scenarios.length === 0)
