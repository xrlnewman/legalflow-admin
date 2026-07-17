import './styles.css'
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

const demoDashboard = { todayAppointments: 86, averageWaitMinutes: 12, completed: 58, checkedIn: 42, pendingFollowups: 12 }
const statusColors = { 待立案: 'coral', 已立案: 'indigo', 待办理: 'amber', 办理中: 'green', 已结案: 'green', 已撤案: 'gray' }
const nav = [
  ['overview', '运营总览', '⌂'],
  ['queue', '案件队列', '▤'],
  ['doctors', '律师排班', '◉'],
  ['patients', '当事人档案', '♧'],
  ['followups', '合规任务', '✓'],
  ['mobile', '移动端体验', '⌁'],
]

let appointments = demoAppointments.map((item) => ({ ...item }))
let followupTasks = demoFollowups.map((item) => ({ ...item }))
let dashboard = { ...demoDashboard }
let page = 'overview'
let toast = ''
let toastTimer
let dataSource = '演示数据'
let isSyncing = false

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
  if (appointment.status === '办理中') return `<button class="text-action" data-action="status" data-next-status="已完成" data-appointment-id="${appointment.id}">完成办理</button>`
  return '<button class="text-action" data-toast="该案件已完成，无需重复操作">查看详情</button>'
}

function header(title) {
  return `<header><span>工作台　/　<strong>${title}</strong></span><span class="header-tools"><span>2026 年 7 月 16 日</span><span class="data-source ${dataSource === 'API 数据' ? 'remote' : ''}">● ${isSyncing ? '同步中' : dataSource}</span><button class="refresh" data-refresh ${isSyncing ? 'disabled' : ''}>↻ 刷新</button></span></header>`
}

function render() {
  const title = nav.find((item) => item[0] === page)?.[1] || '运营总览'
  const content = page === 'overview' ? overview() : page === 'queue' ? queue() : page === 'doctors' ? doctors() : page === 'patients' ? patients() : page === 'followups' ? followups() : mobileView()
  document.querySelector('#app').innerHTML = `<div class="shell"><aside><div class="brand"><span>✚</span><div><strong>LegalFlow</strong><small>法务运营中心</small></div></div><div class="clinic">● 上海静安联合法务　⌄</div><p class="caption">临床运营</p><nav>${nav.map((item) => `<button class="${page === item[0] ? 'active' : ''}" data-page="${item[0]}"><i>${item[2]}</i>${item[1]}${item[0] === 'queue' ? '<em>8</em>' : ''}</button>`).join('')}</nav><div class="user"><b>许</b><span><strong>许汝林</strong><small>运营管理员</small></span></div></aside><main>${header(title)}<section class="heading"><div><p>THURSDAY, JUL 16 · LEGALFLOW</p><h1>${title} <i>✦</i></h1><label>让每一次案件，都有被照顾的下一步。</label></div><button class="primary" data-action="create-appointment">＋ 新建案件</button></section>${content}<footer>LegalFlow 法务案件与合规协同 · 免费开源 · 演示数据不含诊断与真实当事人信息</footer><div class="toast" ${toast ? '' : 'hidden'}>${toast}</div></main></div>`
  bind()
}

function overview() {
  return `<section class="metrics"><article class="metric dark"><span>今日案件</span><strong>${dashboard.todayAppointments}</strong><small>↗ 较昨日 +14.6%</small></article><article class="metric"><span>平均办理周期</span><strong>${dashboard.averageWaitMinutes}<small> 天</small></strong><small class="good">较上周 -3 天</small></article><article class="metric"><span>今日结案</span><strong>${dashboard.completed}<small> 件</small></strong><div class="progress"><i style="width:68%"></i></div></article><article class="metric warm"><span>待合规</span><strong>${dashboard.pendingFollowups}<small> 条</small></strong><small class="coral">今日需完成</small></article></section><section class="grid"><article class="panel calendar"><div class="panel-head"><div><h2>今日案件队列</h2><p>7 月 16 日 · 周四 · 共 ${dashboard.todayAppointments} 个案件</p></div><button class="link" data-page="queue">查看队列 →</button></div><div class="timeline">${appointments.slice(0, 4).map((appointment) => `<div class="time-row"><span>${timeLabel(appointment.scheduledAt)}</span><i class="time-dot ${statusColors[appointment.status] || 'indigo'}"></i><div><strong>${appointment.patient}</strong><small>${appointment.department} · ${appointment.status}</small></div><b class="status ${statusColors[appointment.status] || 'indigo'}">${appointment.status}</b></div>`).join('')}</div></article><article class="panel"><div class="panel-head"><div><h2>业务领域负载</h2><p>当前时段律师利用率</p></div><button class="link" data-page="doctors">排班管理 →</button></div><div class="load-list">${[['合同纠纷', '32 / 40', '80%', 'indigo'], ['劳动争议', '18 / 24', '75%', 'coral'], ['知识产权', '12 / 18', '67%', 'green'], ['公司治理', '8 / 12', '66%', 'amber']].map((item) => `<div class="load"><div><strong>${item[0]}</strong><span>${item[1]}</span></div><div class="load-bar"><i class="${item[3]}" style="width:${item[2]}"></i></div><b>${item[2]}</b></div>`).join('')}</div></article></section><section class="grid lower"><article class="panel"><div class="panel-head"><div><h2>合规完成趋势</h2><p>近 7 日任务完成率</p></div><span class="legend">本周平均 84%</span></div><div class="spark"><i style="height:38%"></i><i style="height:58%"></i><i style="height:46%"></i><i style="height:74%"></i><i style="height:66%"></i><i style="height:88%"></i><i class="today" style="height:80%"></i></div><div class="days"><span>周五</span><span>周六</span><span>周日</span><span>周一</span><span>周二</span><span>周三</span><span>今天</span></div></article><article class="panel tasks"><div class="panel-head"><div><h2>待办提醒</h2><p>需要运营人员跟进的事项</p></div></div><div class="task"><span class="task-icon coral">!</span><div><strong>3 个案件需要补充材料</strong><small>案件队列 · 10 分钟前</small></div><button data-page="queue">处理</button></div><div class="task"><span class="task-icon amber">✓</span><div><strong>${dashboard.pendingFollowups} 条合规今日到期</strong><small>法务合规 · 32 分钟前</small></div><button data-page="followups">查看</button></div></article></section>`
}

function queue() {
  return `<section class="panel full"><div class="panel-head"><div><h2>案件队列</h2><p>${dataSource === 'API 数据' ? 'API 实时案件' : '20 条演示案件'} · 支持立案、办理、结案和归档</p></div><span class="chip">今天　⌄</span></div><div class="table"><div class="th"><span>案件编号 / 当事人</span><span>业务领域</span><span>时间</span><span>状态</span><span>操作</span></div>${appointments.concat(dataSource === 'API 数据' ? [] : appointments.slice(0, 3)).map((appointment) => `<div class="tr"><span><strong>${appointment.id}</strong><small>${appointment.patient}</small></span><span>${appointment.department}</span><span>${timeLabel(appointment.scheduledAt)}</span><b class="status ${statusColors[appointment.status] || 'indigo'}">${appointment.status}</b><span>${appointmentAction(appointment)}</span></div>`).join('')}</div></section>`
}

function doctors() {
  return `<section class="panel full"><div class="panel-head"><div><h2>律师排班</h2><p>8 位律师 · 今日 42 个可案件时段</p></div><button class="primary small" data-toast="排班编辑器已打开">编辑排班</button></div><div class="doctor-grid">${[['林律师', '全科门诊', '32 号候诊', 'indigo'], ['沈律师', '皮肤科', '18 号候诊', 'coral'], ['赵律师', '康复理疗', '办理中', 'green'], ['周律师', '营养咨询', '8 号候诊', 'amber'], ['陈律师', '全科门诊', '午间休息', 'gray'], ['王律师', '心理咨询', '6 号候诊', 'indigo']].map((doctor) => `<article><div class="doctor-avatar ${doctor[3]}">${doctor[0][0]}</div><div><strong>${doctor[0]}</strong><small>${doctor[1]}</small></div><span>${doctor[2]}</span><div class="schedule-line"><i style="width:78%"></i></div></article>`).join('')}</div></section>`
}

function patients() {
  return `<section class="panel full"><div class="panel-head"><div><h2>当事人档案</h2><p>30 条虚构档案 · 仅用于界面演示</p></div><button class="link" data-toast="导出任务已创建">导出列表 ↓</button></div><div class="table"><div class="th"><span>当事人 / 编号</span><span>最近科室</span><span>最近案件</span><span>合规状态</span><span>操作</span></div>${[['林晓雨', 'CF-2038', '全科门诊', '07/16', '待合规'], ['沈明远', 'CF-2037', '皮肤科', '07/15', '进行中'], ['赵思涵', 'CF-2036', '康复理疗', '07/14', '已完成'], ['周子昂', 'CF-2035', '全科门诊', '07/13', '待合规'], ['许安然', 'CF-2034', '营养咨询', '07/12', '已完成']].map((patient) => `<div class="tr"><span><strong>${patient[0]}</strong><small>${patient[1]}</small></span><span>${patient[2]}</span><span>${patient[3]}</span><b class="status ${patient[4] === '已完成' ? 'green' : 'coral'}">${patient[4]}</b><button class="text-action" data-toast="${patient[0]} 档案已打开">查看档案</button></div>`).join('')}</div></section>`
}

function followups() {
  return `<section class="panel full"><div class="panel-head"><div><h2>合规任务</h2><p>${dataSource === 'API 数据' ? 'API 实时合规' : '12 条待跟进任务'} · 由律师/护士确认后记录</p></div><span class="chip">全部任务　⌄</span></div><div class="follow-list">${followupTasks.map((item) => `<article><span class="task-icon ${item.status === '已完成' ? 'green' : 'coral'}">✓</span><div><strong>${item.id} · ${item.patient}</strong><p>${item.summary}</p><small>${item.dueAt} · ${dataSource === 'API 数据' ? 'API 数据' : '演示任务'}</small></div>${item.status === '已完成' ? '<button class="text-action" data-toast="该合规已经完成">查看</button>' : `<button class="text-action" data-action="complete-followup" data-followup-id="${item.id}">完成任务</button>`}</article>`).join('')}</div></section>`
}

function mobileView() {
  return `<section class="mobile-panel"><div class="mobile-panel__hero"><span>LEGALFLOW MOBILE</span><h2>我的案件与合规</h2><p>当事人端可在同一套闭环 API 中完成签到、候诊、办理和合规确认。</p><button class="primary" data-action="create-appointment">＋ 创建演示案件</button></div><div class="mobile-list"><h3>今日案件</h3>${appointments.slice(0, 4).map((appointment) => `<article class="mobile-card"><div><small>${timeLabel(appointment.scheduledAt)} · ${appointment.department}</small><strong>${appointment.patient}</strong><span>${appointment.doctor} · ${appointment.status}</span></div><b class="status ${statusColors[appointment.status] || 'indigo'}">${appointment.status}</b>${appointmentAction(appointment)}</article>`).join('')}</div><div class="mobile-list"><h3>我的合规</h3>${followupTasks.slice(0, 3).map((item) => `<article class="mobile-card"><div><small>${item.dueAt}</small><strong>${item.summary}</strong><span>${item.patient} · ${item.status}</span></div>${item.status === '已完成' ? '<b class="status green">已完成</b>' : `<button class="text-action" data-action="complete-followup" data-followup-id="${item.id}">完成合规</button>`}</article>`).join('')}</div></section>`
}

async function refreshFromApi({ quiet = false } = {}) {
  if (isSyncing) return
  isSyncing = true
  render()
  try {
    const [nextDashboard, nextAppointments, nextFollowups] = await Promise.all([
      api.getDashboard(),
      api.listAppointments({ page: 1, pageSize: 20 }),
      api.listFollowups({ page: 1, pageSize: 20 }),
    ])
    dashboard = { ...demoDashboard, ...nextDashboard }
    appointments = (nextAppointments?.list || []).map(normalizeAppointment)
    followupTasks = (nextFollowups?.list || []).map(normalizeFollowup)
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
    showToast('案件已创建，可继续在移动端完成签到')
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
    return undefined
  }))
}

render()
refreshFromApi({ quiet: true })
