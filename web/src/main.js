import './styles.css'
import './matters.css'
import { createApiClient } from './api.js'

const api = createApiClient()

const demoAppointments = [
  { id: 'LG-0716-082', patient: '案卷 C082 · 上海某公司', department: '合同纠纷', doctor: '林律师', scheduledAt: '2026-07-16T09:30:00+08:00', status: '待办理' },
  { id: 'LG-0716-081', patient: '案卷 C081 · 王某', department: '劳动争议', doctor: '沈律师', scheduledAt: '2026-07-16T09:45:00+08:00', status: '已立案' },
  { id: 'LG-0716-080', patient: '案卷 C080 · 星河科技', department: '知识产权', doctor: '赵律师', scheduledAt: '2026-07-16T10:00:00+08:00', status: '已结案' },
  { id: 'LG-0716-079', patient: '案卷 C079 · 李某', department: '合同纠纷', doctor: '林律师', scheduledAt: '2026-07-16T10:15:00+08:00', status: '待立案' },
  { id: 'LG-0716-078', patient: '案卷 C078 · 云杉贸易', department: '公司治理', doctor: '周律师', scheduledAt: '2026-07-16T10:30:00+08:00', status: '待立案' },
]

const demoFollowups = [
  { id: 'LG-0716-012', patient: '案卷 C082', summary: '证据清单与举证期限', dueAt: '今天 16:00', status: '待完成' },
  { id: 'LG-0716-011', patient: '案卷 C081', summary: '庭审材料校对', dueAt: '今天 17:30', status: '待完成' },
  { id: 'LG-0716-010', patient: '案卷 C080', summary: '合同合规意见归档', dueAt: '明天 09:30', status: '待完成' },
  { id: 'LG-0715-009', patient: '案卷 C079', summary: '结案文书归档', dueAt: '已完成', status: '已完成' },
]

const demoMatters = [
  { id: 'LF-0720-003', subjectAlias: '演示案卷-003', caseType: '知识产权', priority: '高', deadline: '2026-07-23', assignee: '赵律师', status: '待结案', documents: [{ id: 'DOC-003', name: '证据目录.pdf', kind: 'evidence', checksum: 'sha256:demo-003' }], tasks: [{ id: 'TASK-003', title: '结案文书复核', assignee: '赵律师', status: '待处理' }], events: [{ id: 'EV-003-1', action: '创建案件', actor: '系统', createdAt: '2026-07-16T01:00:00Z', toStatus: '待委托' }, { id: 'EV-003-2', action: '分配负责人', actor: '许汝林', createdAt: '2026-07-16T03:00:00Z', toStatus: '已立案' }, { id: 'EV-003-3', action: '归档文档', actor: '赵律师', createdAt: '2026-07-16T05:00:00Z' }] },
  { id: 'LF-0720-002', subjectAlias: '演示案卷-002', caseType: '劳动争议', priority: '中', deadline: '2026-07-22', assignee: '沈律师', status: '协同中', documents: [], tasks: [{ id: 'TASK-002', title: '证据清单核验', assignee: '沈律师', status: '待处理' }], events: [] },
  { id: 'LF-0720-004', subjectAlias: '演示案卷-004', caseType: '合同审查', priority: '低', deadline: '2026-07-24', assignee: '', status: '待委托', documents: [], tasks: [], events: [] },
  { id: 'LF-0720-005', subjectAlias: '演示案卷-005', caseType: '公司治理', priority: '中', deadline: '2026-07-25', assignee: '林律师', status: '已结案', closureResult: '材料已归档，待客户确认', documents: [{ id: 'DOC-005', name: '结案摘要.docx', kind: 'closure', checksum: 'sha256:demo-005' }], tasks: [], events: [] },
]

const demoDashboard = { todayAppointments: 86, averageWaitMinutes: 12, completed: 58, checkedIn: 42, pendingFollowups: 12 }
const statusColors = { 待立案: 'coral', 已立案: 'indigo', 待办理: 'amber', 办理中: 'green', 已结案: 'green', 已撤案: 'gray', 待委托: 'coral', 协同中: 'indigo', 待结案: 'amber' }
const nav = [
  ['overview', '运营总览', '⌂'],
  ['queue', '案件队列', '▤'],
  ['doctors', '律师排班', '◉'],
  ['patients', '当事人档案', '♧'],
  ['followups', '合规任务', '✓'],
  ['mobile', '移动端体验', '⌁'],
  ['matters', '案件协同', '◇'],
]

let appointments = demoAppointments.map((item) => ({ ...item }))
let followupTasks = demoFollowups.map((item) => ({ ...item }))
let dashboard = { ...demoDashboard }
let page = 'overview'
let toast = ''
let toastTimer
let dataSource = '演示数据'
let isSyncing = false
let matters = demoMatters.map((item) => ({ ...item }))
let selectedMatterId = matters[0]?.id || ''
const matterEventsById = new Map(matters.map((item) => [item.id, item.events || []]))

function timeLabel(value) {
  const match = String(value ?? '').match(/T(\d{2}:\d{2})/)
  return match?.[1] || String(value ?? '').slice(0, 5) || '--:--'
}

function normalizeAppointment(item) {
  return {
    id: item.id,
    patientId: item.patientId,
    patient: item.patient || '未命名当事人',
    department: item.department || '待分诊',
    doctor: item.doctor || '待安排',
    scheduledAt: item.scheduledAt || '',
    status: item.status || '待立案',
  }
}

function normalizeFollowup(item) {
  return {
    id: item.id,
    patientId: item.patientId,
    patient: item.patient || '未命名当事人',
    summary: item.summary || '法务合规任务',
    dueAt: item.dueAt || '--',
    status: item.status || '待完成',
  }
}

function showToast(message) {
  toast = message
  render()
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    toast = ''
    render()
  }, 2200)
}

function appointmentAction(appointment) {
  if (appointment.status === '待立案') return `<button class="text-action" data-action="checkin" data-appointment-id="${appointment.id}">提交立案</button>`
  if (appointment.status === '已立案') return `<button class="text-action" data-action="status" data-next-status="待办理" data-appointment-id="${appointment.id}">进入办理</button>`
  if (appointment.status === '待办理') return `<button class="text-action" data-action="status" data-next-status="办理中" data-appointment-id="${appointment.id}">开始办理</button>`
  if (appointment.status === '办理中') return `<button class="text-action" data-action="status" data-next-status="已结案" data-appointment-id="${appointment.id}">完成办理</button>`
  return '<button class="text-action" data-toast="该案件已完成，无需重复操作">查看详情</button>'
}

function header(title) {
  return `<header><span>工作台　/　<strong>${title}</strong></span><span class="header-tools"><span>2026 年 7 月 16 日</span><span class="data-source ${dataSource === 'API 数据' ? 'remote' : ''}">● ${isSyncing ? '同步中' : dataSource}</span><button class="refresh" data-refresh ${isSyncing ? 'disabled' : ''}>↻ 刷新</button></span></header>`
}

function render() {
  const title = nav.find((item) => item[0] === page)?.[1] || '运营总览'
  const content = page === 'overview' ? overview() : page === 'queue' ? queue() : page === 'doctors' ? doctors() : page === 'patients' ? patients() : page === 'followups' ? followups() : page === 'matters' ? mattersView() : mobileView()
  document.querySelector('#app').innerHTML = `<div class="shell"><aside><div class="brand"><span>✚</span><div><strong>LegalFlow</strong><small>法务运营中心</small></div></div><div class="clinic">● 上海静安联合法务　⌄</div><p class="caption">案件运营</p><nav>${nav.map((item) => `<button class="${page === item[0] ? 'active' : ''}" data-page="${item[0]}"><i>${item[2]}</i>${item[1]}${item[0] === 'queue' ? '<em>8</em>' : ''}</button>`).join('')}</nav><div class="user"><b>许</b><span><strong>许汝林</strong><small>运营管理员</small></span></div></aside><main>${header(title)}<section class="heading"><div><p>THURSDAY, JUL 16 · LEGALFLOW</p><h1>${title} <i>✦</i></h1><label>让每一次案件，都有被照顾的下一步。</label></div><button class="primary" data-action="create-appointment">＋ 新建案件</button></section>${content}<footer>LegalFlow 法务案件与合规协同 · 免费开源 · 演示数据不含诊断与真实当事人信息</footer><div class="toast" ${toast ? '' : 'hidden'}>${toast}</div></main></div>`
  bind()
}

function overview() {
  return `<section class="metrics"><article class="metric dark"><span>今日案件</span><strong>${dashboard.todayAppointments}</strong><small>↗ 较昨日 +14.6%</small></article><article class="metric"><span>平均办理周期</span><strong>${dashboard.averageWaitMinutes}<small> 天</small></strong><small class="good">较上周 -3 天</small></article><article class="metric"><span>今日结案</span><strong>${dashboard.completed}<small> 件</small></strong><div class="progress"><i style="width:68%"></i></div></article><article class="metric warm"><span>待合规</span><strong>${dashboard.pendingFollowups}<small> 条</small></strong><small class="coral">今日需完成</small></article></section><section class="grid"><article class="panel calendar"><div class="panel-head"><div><h2>今日案件队列</h2><p>7 月 16 日 · 周四 · 共 ${dashboard.todayAppointments} 个案件</p></div><button class="link" data-page="queue">查看队列 →</button></div><div class="timeline">${appointments.slice(0, 4).map((appointment) => `<div class="time-row"><span>${timeLabel(appointment.scheduledAt)}</span><i class="time-dot ${statusColors[appointment.status] || 'indigo'}"></i><div><strong>${appointment.patient}</strong><small>${appointment.department} · ${appointment.status}</small></div><b class="status ${statusColors[appointment.status] || 'indigo'}">${appointment.status}</b></div>`).join('')}</div></article><article class="panel"><div class="panel-head"><div><h2>业务领域负载</h2><p>当前时段律师利用率</p></div><button class="link" data-page="doctors">排班管理 →</button></div><div class="load-list">${[['合同纠纷', '32 / 40', '80%', 'indigo'], ['劳动争议', '18 / 24', '75%', 'coral'], ['知识产权', '12 / 18', '67%', 'green'], ['公司治理', '8 / 12', '66%', 'amber']].map((item) => `<div class="load"><div><strong>${item[0]}</strong><span>${item[1]}</span></div><div class="load-bar"><i class="${item[3]}" style="width:${item[2]}"></i></div><b>${item[2]}</b></div>`).join('')}</div></article></section><section class="grid lower"><article class="panel"><div class="panel-head"><div><h2>合规完成趋势</h2><p>近 7 日任务完成率</p></div><span class="legend">本周平均 84%</span></div><div class="spark"><i style="height:38%"></i><i style="height:58%"></i><i style="height:46%"></i><i style="height:74%"></i><i style="height:66%"></i><i style="height:88%"></i><i class="today" style="height:80%"></i></div><div class="days"><span>周五</span><span>周六</span><span>周日</span><span>周一</span><span>周二</span><span>周三</span><span>今天</span></div></article><article class="panel tasks"><div class="panel-head"><div><h2>待办提醒</h2><p>需要运营人员跟进的事项</p></div></div><div class="task"><span class="task-icon coral">!</span><div><strong>3 个案件需要补充材料</strong><small>案件队列 · 10 分钟前</small></div><button data-page="queue">处理</button></div><div class="task"><span class="task-icon amber">✓</span><div><strong>${dashboard.pendingFollowups} 条合规今日到期</strong><small>法务合规 · 32 分钟前</small></div><button data-page="followups">查看</button></div></article></section>`
}

function queue() {
  return `<section class="panel full"><div class="panel-head"><div><h2>案件队列</h2><p>${dataSource === 'API 数据' ? 'API 实时案件' : '20 条演示案件'} · 支持立案、办理、结案和归档</p></div><span class="chip">今天　⌄</span></div><div class="table"><div class="th"><span>案件编号 / 当事人</span><span>业务领域</span><span>时间</span><span>状态</span><span>操作</span></div>${appointments.concat(dataSource === 'API 数据' ? [] : appointments.slice(0, 3)).map((appointment) => `<div class="tr"><span><strong>${appointment.id}</strong><small>${appointment.patient}</small></span><span>${appointment.department}</span><span>${timeLabel(appointment.scheduledAt)}</span><b class="status ${statusColors[appointment.status] || 'indigo'}">${appointment.status}</b><span>${appointmentAction(appointment)}</span></div>`).join('')}</div></section>`
}

function doctors() {
  return `<section class="panel full"><div class="panel-head"><div><h2>律师排班</h2><p>8 位律师 · 今日 42 个可案件时段</p></div><button class="primary small" data-toast="排班编辑器已打开">编辑排班</button></div><div class="doctor-grid">${[['林律师', '合同纠纷', '32 个办理中', 'indigo'], ['沈律师', '劳动争议', '18 个待复核', 'coral'], ['赵律师', '知识产权', '办理中', 'green'], ['周律师', '公司治理', '8 个排期中', 'amber'], ['陈律师', '合同纠纷', '午间休息', 'gray'], ['王律师', '劳动争议', '6 个排期中', 'indigo']].map((lawyer) => `<article><div class="doctor-avatar ${lawyer[3]}">${lawyer[0][0]}</div><div><strong>${lawyer[0]}</strong><small>${lawyer[1]}</small></div><span>${lawyer[2]}</span><div class="schedule-line"><i style="width:78%"></i></div></article>`).join('')}</div></section>`
}

function patients() {
  return `<section class="panel full"><div class="panel-head"><div><h2>当事人档案</h2><p>30 条虚构档案 · 仅用于界面演示</p></div><button class="link" data-toast="导出任务已创建">导出列表 ↓</button></div><div class="table"><div class="th"><span>当事人 / 编号</span><span>业务领域</span><span>最近案件</span><span>合规状态</span><span>操作</span></div>${[['上海某公司', 'LG-2038', '合同纠纷', '07/16', '待合规'], ['王某', 'LG-2037', '劳动争议', '07/15', '进行中'], ['星河科技', 'LG-2036', '知识产权', '07/14', '已结案'], ['李某', 'LG-2035', '合同纠纷', '07/13', '待合规'], ['云杉贸易', 'LG-2034', '公司治理', '07/12', '已结案']].map((client) => `<div class="tr"><span><strong>${client[0]}</strong><small>${client[1]}</small></span><span>${client[2]}</span><span>${client[3]}</span><b class="status ${client[4] === '已结案' ? 'green' : 'coral'}">${client[4]}</b><button class="text-action" data-toast="${client[0]} 档案已打开">查看档案</button></div>`).join('')}</div></section>`
}

function followups() {
  return `<section class="panel full"><div class="panel-head"><div><h2>合规任务</h2><p>${dataSource === 'API 数据' ? 'API 实时合规' : '12 条待跟进任务'} · 由律师/运营确认后记录</p></div><span class="chip">全部任务　⌄</span></div><div class="follow-list">${followupTasks.map((item) => `<article><span class="task-icon ${item.status === '已完成' ? 'green' : 'coral'}">✓</span><div><strong>${item.id} · ${item.patient}</strong><p>${item.summary}</p><small>${item.dueAt} · ${dataSource === 'API 数据' ? 'API 数据' : '演示任务'}</small></div>${item.status === '已完成' ? '<button class="text-action" data-toast="该合规已经完成">查看</button>' : `<button class="text-action" data-action="complete-followup" data-followup-id="${item.id}">完成任务</button>`}</article>`).join('')}</div></section>`
}

function matterStatus(status) { return statusColors[status] || 'indigo' }

function mattersView() {
  const selected = matters.find((item) => item.id === selectedMatterId) || matters[0]
  if (!selected) return '<section class="panel full"><h2>案件协同</h2><p>暂无案件</p></section>'
  const events = matterEventsById.get(selected.id) || selected.events || []
  const nextStatus = selected.status === '已立案' ? '协同中' : selected.status === '协同中' ? '待结案' : ''
  return `<section class="matter-layout"><article class="panel matter-queue"><div class="panel-head"><div><h2>案件截止日看板</h2><p>仅展示案件别名与运营节点，不保存真实客户身份</p></div><button class="primary small" data-action="create-matter">＋ 新建案件</button></div><div class="matter-filters"><span class="chip">全部状态　⌄</span><span class="chip">负责人　⌄</span><span class="deadline-tip">${matters.length} 个案件 · ${matters.filter((item) => item.status === '待结案').length} 个待结案</span></div><div class="matter-list">${matters.map((item) => `<button class="matter-row ${item.id === selected.id ? 'active' : ''}" data-action="select-matter" data-matter-id="${item.id}"><span><strong>${item.subjectAlias}</strong><small>${item.caseType} · ${item.priority}优先级</small></span><span><b class="status ${matterStatus(item.status)}">${item.status}</b><small class="deadline">截止 ${item.deadline}</small></span></button>`).join('')}</div></article><article class="panel matter-detail"><div class="panel-head"><div><h2>${selected.subjectAlias}</h2><p>${selected.caseType} · 负责人 ${selected.assignee || '待分配'}</p></div><b class="status ${matterStatus(selected.status)}">${selected.status}</b></div><div class="matter-meta"><span><small>截止日期</small><strong>${selected.deadline}</strong></span><span><small>优先级</small><strong>${selected.priority}</strong></span><span><small>文档数</small><strong>${(selected.documents || []).length} 份</strong></span></div><div class="matter-actions"><button class="text-action" data-action="assign-matter" data-matter-id="${selected.id}">分配林律师</button>${nextStatus ? `<button class="text-action" data-action="advance-matter" data-matter-id="${selected.id}" data-next-status="${nextStatus}">推进至${nextStatus}</button>` : ''}<button class="text-action" data-action="add-matter-file" data-matter-id="${selected.id}">归档文档</button>${selected.status === '待结案' ? `<button class="primary small" data-action="close-matter" data-matter-id="${selected.id}">提交结案</button>` : ''}</div><div class="matter-section"><div class="section-title"><h3>协同任务</h3><span>${(selected.tasks || []).length} 条</span></div>${(selected.tasks || []).length ? selected.tasks.map((task) => `<div class="matter-task"><span class="task-icon indigo">✓</span><div><strong>${task.title}</strong><small>${task.assignee} · ${task.status}</small></div></div>`).join('') : '<p class="muted-copy">尚未分配负责人</p>'}</div><div class="matter-section"><div class="section-title"><h3>文档归档</h3><span>${(selected.documents || []).length} 份 · checksum 已校验</span></div>${(selected.documents || []).length ? selected.documents.map((doc) => `<div class="matter-doc"><span>▧</span><div><strong>${doc.name}</strong><small>${doc.kind} · ${doc.checksum}</small></div></div>`).join('') : '<p class="muted-copy">暂无文档元数据</p>'}</div><div class="matter-section timeline-section"><div class="section-title"><h3>事件时间线</h3><span>按发生时间</span></div>${events.map((event) => `<div class="matter-event"><i></i><div><strong>${event.action}${event.toStatus ? ` · ${event.toStatus}` : ''}</strong><small>${event.actor} · ${event.createdAt || event.time || ''}</small></div></div>`).join('')}</div></article></section>`
}

function normalizeMatter(item) { return { ...item, subjectAlias: item.subjectAlias || '演示案卷', caseType: item.caseType || '待分类', priority: item.priority || '中', deadline: item.deadline || '--', assignee: item.assignee || '', status: item.status || '待委托', documents: item.documents || [], tasks: item.tasks || [], events: item.events || [] } }

async function refreshMatterEvents(id) {
  try { const response = await api.listMatterEvents(id); matterEventsById.set(id, response?.list || []); const item = matters.find((matter) => matter.id === id); if (item) item.events = response?.list || []; render() } catch { /* demo timeline remains visible */ }
}

async function selectMatter(id) { selectedMatterId = id; render(); await refreshMatterEvents(id) }

async function createMatter() {
  try { const created = await api.createMatter({ subjectAlias: `演示案卷-${Date.now().toString().slice(-3)}`, caseType: '合同审查', priority: '中', deadline: '2026-07-30' }); matters = [normalizeMatter(created), ...matters]; selectedMatterId = created.id; dataSource = 'API 数据'; showToast('案件已创建，等待负责人分配'); await refreshMatterEvents(created.id) } catch (error) { showToast(`接口暂不可用：${error.message}`) }
}

async function matterAction(button) {
  const id = button.dataset.matterId; const matter = matters.find((item) => item.id === id); if (!matter) return
  try {
    let result
    if (button.dataset.action === 'assign-matter') result = (await api.assignMatter(id, { assignee: '林律师', actor: '许汝林' })).matter
    if (button.dataset.action === 'advance-matter') result = await api.updateMatterStatus(id, button.dataset.nextStatus, '许汝林')
    if (button.dataset.action === 'add-matter-file') { await api.addMatterFile(id, { name: '案件协同记录.pdf', kind: 'collaboration', checksum: `sha256:${id}` }); result = await api.getMatter(id) }
    if (button.dataset.action === 'close-matter') result = (await api.closeMatter(id, { result: '结案摘要已归档', actor: '许汝林' })).matter
    if (result) matters = matters.map((item) => item.id === id ? normalizeMatter({ ...item, ...result }) : item)
    dataSource = 'API 数据'; await refreshMatterEvents(id); showToast('案件协同信息已更新')
  } catch (error) { showToast(`操作未完成：${error.message}`) }
}

function mobileView() {
  return `<section class="mobile-panel"><div class="mobile-panel__hero"><span>LEGALFLOW MOBILE</span><h2>我的案件与合规</h2><p>当事人端可在同一套闭环 API 中完成立案、办理、结案和合规确认。</p><button class="primary" data-action="create-appointment">＋ 创建演示案件</button></div><div class="mobile-list"><h3>今日案件</h3>${appointments.slice(0, 4).map((appointment) => `<article class="mobile-card"><div><small>${timeLabel(appointment.scheduledAt)} · ${appointment.department}</small><strong>${appointment.patient}</strong><span>${appointment.doctor} · ${appointment.status}</span></div><b class="status ${statusColors[appointment.status] || 'indigo'}">${appointment.status}</b>${appointmentAction(appointment)}</article>`).join('')}</div><div class="mobile-list"><h3>我的合规</h3>${followupTasks.slice(0, 3).map((item) => `<article class="mobile-card"><div><small>${item.dueAt}</small><strong>${item.summary}</strong><span>${item.patient} · ${item.status}</span></div>${item.status === '已完成' ? '<b class="status green">已完成</b>' : `<button class="text-action" data-action="complete-followup" data-followup-id="${item.id}">完成合规</button>`}</article>`).join('')}</div></section>`
}

async function refreshFromApi({ quiet = false } = {}) {
  if (isSyncing) return
  isSyncing = true
  render()
  try {
    const [nextDashboard, nextAppointments, nextFollowups, nextMatters] = await Promise.all([
      api.getDashboard(),
      api.listAppointments({ page: 1, pageSize: 20 }),
      api.listFollowups({ page: 1, pageSize: 20 }),
      api.listMatters({ page: 1, pageSize: 50 }),
    ])
    dashboard = { ...demoDashboard, ...nextDashboard }
    appointments = (nextAppointments?.list || []).map(normalizeAppointment)
    followupTasks = (nextFollowups?.list || []).map(normalizeFollowup)
    matters = (nextMatters?.list || []).map(normalizeMatter)
    if (matters.length && !matters.some((item) => item.id === selectedMatterId)) selectedMatterId = matters[0].id
    dataSource = 'API 数据'
    if (!quiet) toast = '已从 LegalFlow API 刷新数据'
  } catch (error) {
    dataSource = '演示数据'
    if (!quiet) toast = `API 暂不可用，继续使用演示数据：${error.message}`
  } finally {
    isSyncing = false
    render()
  }
}

function replaceAppointment(updated) {
  appointments = appointments.map((item) => item.id === updated.id ? normalizeAppointment(updated) : item)
}

async function advanceAppointment(button) {
  const id = button.dataset.appointmentId
  const appointment = appointments.find((item) => item.id === id)
  if (!appointment) return
  const nextStatus = button.dataset.nextStatus
  try {
    const updated = button.dataset.action === 'checkin'
      ? await api.checkinAppointment(id)
      : await api.updateAppointmentStatus(id, nextStatus, '运营人员')
    replaceAppointment(updated)
    dataSource = 'API 数据'
    showToast(`${appointment.patient} 已更新为${updated.status}`)
  } catch (error) {
    dataSource = '演示数据'
    showToast(`接口暂不可用，已保留演示数据：${error.message}`)
  }
}

async function completeFollowup(button) {
  const id = button.dataset.followupId
  const task = followupTasks.find((item) => item.id === id)
  if (!task) return
  try {
    const updated = await api.completeFollowup(id)
    followupTasks = followupTasks.map((item) => item.id === id ? normalizeFollowup(updated) : item)
    dataSource = 'API 数据'
    showToast(`${task.patient} 的合规已完成`)
  } catch (error) {
    dataSource = '演示数据'
    showToast(`接口暂不可用，已保留演示任务：${error.message}`)
  }
}

async function createAppointment() {
  try {
    const created = await api.createAppointment({ patient: '移动端演示案件', patientId: 'LG-MOBILE-DEMO', department: '合同纠纷', doctor: '林律师', scheduledAt: new Date().toISOString() })
    appointments = [normalizeAppointment(created), ...appointments]
    dataSource = 'API 数据'
    showToast('案件已创建，可继续在移动端提交立案')
  } catch (error) {
    dataSource = '演示数据'
    showToast(`API 暂不可用，保留演示案件：${error.message}`)
  }
}

function bind() {
  document.querySelectorAll('[data-page]').forEach((element) => element.addEventListener('click', () => {
    page = element.dataset.page
    render()
  }))
  document.querySelectorAll('[data-toast]').forEach((element) => element.addEventListener('click', () => showToast(element.dataset.toast)))
  document.querySelectorAll('[data-refresh]').forEach((element) => element.addEventListener('click', () => refreshFromApi()))
  document.querySelectorAll('[data-action]').forEach((element) => element.addEventListener('click', () => {
    if (element.dataset.action === 'checkin' || element.dataset.action === 'status') return advanceAppointment(element)
    if (element.dataset.action === 'complete-followup') return completeFollowup(element)
    if (element.dataset.action === 'create-appointment') return createAppointment()
    if (element.dataset.action === 'create-matter') return createMatter()
    if (element.dataset.action === 'select-matter') return selectMatter(element.dataset.matterId)
    if (['assign-matter', 'advance-matter', 'add-matter-file', 'close-matter'].includes(element.dataset.action)) return matterAction(element)
    return undefined
  }))
}

render()
refreshFromApi({ quiet: true })
