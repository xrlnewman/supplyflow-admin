import './styles.css'
import { createApiClient } from './api.js'

const api = createApiClient()

const demoAppointments = [
  { id: 'PO-0716-082', patient: '杭州星河家居', department: '智能家电', doctor: '林然 · 采购专员', scheduledAt: '2026-07-16T09:30:00+08:00', status: '候诊中' },
  { id: 'PO-0716-081', patient: '苏州云杉供应商', department: '清洁电器', doctor: '沈宁 · 采购专员', scheduledAt: '2026-07-16T09:45:00+08:00', status: '已确认' },
  { id: 'PO-0716-080', patient: '上海岸线工程', department: '五金配件', doctor: '赵然 · 采购专员', scheduledAt: '2026-07-16T10:00:00+08:00', status: '已完成' },
  { id: 'PO-0716-079', patient: '南京微光零售', department: '仓储耗材', doctor: '林然 · 采购专员', scheduledAt: '2026-07-16T10:15:00+08:00', status: '待确认' },
  { id: 'PO-0716-078', patient: '成都山海餐饮', department: '冷链食材', doctor: '周宁 · 采购专员', scheduledAt: '2026-07-16T10:30:00+08:00', status: '待确认' },
]

const demoFollowups = [
  { id: 'TASK-0716-012', patient: '杭州星河家居', summary: '确认供应商交期与到货批次', dueAt: '今天 16:00', status: '待完成' },
  { id: 'TASK-0716-011', patient: '苏州云杉供应商', summary: '跟进对账单与采购合同', dueAt: '今天 17:30', status: '待完成' },
  { id: 'TASK-0716-010', patient: '上海岸线工程', summary: '补齐入库质检记录', dueAt: '明天 09:30', status: '待完成' },
  { id: 'TASK-0715-009', patient: '南京微光零售', summary: '完成本周采购归档', dueAt: '已完成', status: '已完成' },
]

const demoDashboard = { todayAppointments: 86, averageWaitMinutes: 12, completed: 58, checkedIn: 42, pendingFollowups: 12 }
const statusColors = { 待确认: 'coral', 已确认: 'indigo', 候诊中: 'amber', 处理中: 'green', 已完成: 'green', 已取消: 'gray' }
const nav = [
  ['overview', '运营总览', '⌂'],
  ['queue', '采购单队列', '▤'],
  ['doctors', '采购专员排班', '◉'],
  ['patients', '供应商台账', '♧'],
  ['followups', '采购跟进', '✓'],
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

function displayCopy(root) {
  const rules = [['候诊', '待入库'], ['健康回访', '供应商跟进'], ['回访', '采购跟进'], ['临床', '采购'], ['科室', '采购品类'], ['人次', '单']]
  const walker = document.createTreeWalker(root, 4)
  while (walker.nextNode()) rules.forEach(([from, to]) => { walker.currentNode.nodeValue = walker.currentNode.nodeValue.replaceAll(from, to) })
}

function timeLabel(value) {
  const match = String(value ?? '').match(/T(\d{2}:\d{2})/)
  return match?.[1] || String(value ?? '').slice(0, 5) || '--:--'
}

function normalizeAppointment(item) {
  return {
    id: item.id,
    patientId: item.patientId,
    patient: item.patient || '未命名客户',
    department: item.department || '待分诊',
    doctor: item.doctor || '待安排',
    scheduledAt: item.scheduledAt || '',
    status: item.status || '待确认',
  }
}

function normalizeFollowup(item) {
  return {
    id: item.id,
    patientId: item.patientId,
    patient: item.patient || '未命名客户',
    summary: item.summary || '健康回访任务',
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
  if (appointment.status === '待确认') return `<button class="text-action" data-action="checkin" data-appointment-id="${appointment.id}">确认</button>`
  if (appointment.status === '已确认') return `<button class="text-action" data-action="status" data-next-status="候诊中" data-appointment-id="${appointment.id}">进入候诊</button>`
  if (appointment.status === '候诊中') return `<button class="text-action" data-action="status" data-next-status="处理中" data-appointment-id="${appointment.id}">开始处理</button>`
  if (appointment.status === '处理中') return `<button class="text-action" data-action="status" data-next-status="已完成" data-appointment-id="${appointment.id}">完成处理</button>`
  return '<button class="text-action" data-toast="该采购单已完成，无需重复操作">查看详情</button>'
}

function header(title) {
  return `<header><span>工作台　/　<strong>${title}</strong></span><span class="header-tools"><span>2026 年 7 月 16 日</span><span class="data-source ${dataSource === 'API 数据' ? 'remote' : ''}">● ${isSyncing ? '同步中' : dataSource}</span><button class="refresh" data-refresh ${isSyncing ? 'disabled' : ''}>↻ 刷新</button></span></header>`
}

function render() {
  const title = nav.find((item) => item[0] === page)?.[1] || '运营总览'
  const content = page === 'overview' ? overview() : page === 'queue' ? queue() : page === 'doctors' ? doctors() : page === 'patients' ? patients() : page === 'followups' ? followups() : mobileView()
  document.querySelector('#app').innerHTML = `<div class="shell"><aside><div class="brand"><span>⌁</span><div><strong>SupplyFlow</strong><small>采购供应链运营中心</small></div></div><div class="clinic">● 上海静安联合供应链中心　⌄</div><p class="caption">临床运营</p><nav>${nav.map((item) => `<button class="${page === item[0] ? 'active' : ''}" data-page="${item[0]}"><i>${item[2]}</i>${item[1]}${item[0] === 'queue' ? '<em>8</em>' : ''}</button>`).join('')}</nav><div class="user"><b>许</b><span><strong>许汝林</strong><small>运营管理员</small></span></div></aside><main>${header(title)}<section class="heading"><div><p>THURSDAY, JUL 16 · SUPPLYFLOW</p><h1>${title} <i>✦</i></h1><label>让每一次采购单，都有被照顾的下一步。</label></div><button class="primary" data-action="create-appointment">＋ 新建采购单</button></section>${content}<footer>SupplyFlow 采购供应链运营 · 免费开源 · 演示数据不含诊断与真实客户信息</footer><div class="toast" ${toast ? '' : 'hidden'}>${toast}</div></main></div>`
  const root = document.querySelector('#app')
  displayCopy(root)
  bind()
}

function overview() {
  return `<section class="metrics"><article class="metric dark"><span>今日采购单</span><strong>${dashboard.todayAppointments}</strong><small>↗ 较昨日 +14.6%</small></article><article class="metric"><span>平均候诊</span><strong>${dashboard.averageWaitMinutes}<small> 分钟</small></strong><small class="good">较上周 -3 分钟</small></article><article class="metric"><span>今日完成</span><strong>${dashboard.completed}<small> 人次</small></strong><div class="progress"><i style="width:68%"></i></div></article><article class="metric warm"><span>待回访</span><strong>${dashboard.pendingFollowups}<small> 条</small></strong><small class="coral">今日需完成</small></article></section><section class="grid"><article class="panel calendar"><div class="panel-head"><div><h2>今日采购单队列</h2><p>7 月 16 日 · 周四 · 共 ${dashboard.todayAppointments} 位客户</p></div><button class="link" data-page="queue">查看队列 →</button></div><div class="timeline">${appointments.slice(0, 4).map((appointment) => `<div class="time-row"><span>${timeLabel(appointment.scheduledAt)}</span><i class="time-dot ${statusColors[appointment.status] || 'indigo'}"></i><div><strong>${appointment.patient}</strong><small>${appointment.department} · ${appointment.status}</small></div><b class="status ${statusColors[appointment.status] || 'indigo'}">${appointment.status}</b></div>`).join('')}</div></article><article class="panel"><div class="panel-head"><div><h2>科室处理负载</h2><p>当前时段排班利用率</p></div><button class="link" data-page="doctors">排班管理 →</button></div><div class="load-list">${[['全科门诊', '32 / 40', '80%', 'indigo'], ['皮肤科', '18 / 24', '75%', 'coral'], ['康复理疗', '12 / 18', '67%', 'green'], ['营养咨询', '8 / 12', '66%', 'amber']].map((item) => `<div class="load"><div><strong>${item[0]}</strong><span>${item[1]}</span></div><div class="load-bar"><i class="${item[3]}" style="width:${item[2]}"></i></div><b>${item[2]}</b></div>`).join('')}</div></article></section><section class="grid lower"><article class="panel"><div class="panel-head"><div><h2>回访完成趋势</h2><p>近 7 日任务完成率</p></div><span class="legend">本周平均 84%</span></div><div class="spark"><i style="height:38%"></i><i style="height:58%"></i><i style="height:46%"></i><i style="height:74%"></i><i style="height:66%"></i><i style="height:88%"></i><i class="today" style="height:80%"></i></div><div class="days"><span>周五</span><span>周六</span><span>周日</span><span>周一</span><span>周二</span><span>周三</span><span>今天</span></div></article><article class="panel tasks"><div class="panel-head"><div><h2>待办提醒</h2><p>需要运营人员跟进的事项</p></div></div><div class="task"><span class="task-icon coral">!</span><div><strong>3 位客户需要改约</strong><small>采购单队列 · 10 分钟前</small></div><button data-page="queue">处理</button></div><div class="task"><span class="task-icon amber">✓</span><div><strong>${dashboard.pendingFollowups} 条回访今日到期</strong><small>健康回访 · 32 分钟前</small></div><button data-page="followups">查看</button></div></article></section>`
}

function queue() {
  return `<section class="panel full"><div class="panel-head"><div><h2>采购单队列</h2><p>${dataSource === 'API 数据' ? 'API 实时采购单' : '20 条演示采购单'} · 支持确认、候诊、处理和完成</p></div><span class="chip">今天　⌄</span></div><div class="table"><div class="th"><span>采购单编号 / 客户</span><span>科室</span><span>时间</span><span>状态</span><span>操作</span></div>${appointments.concat(dataSource === 'API 数据' ? [] : appointments.slice(0, 3)).map((appointment) => `<div class="tr"><span><strong>${appointment.id}</strong><small>${appointment.patient}</small></span><span>${appointment.department}</span><span>${timeLabel(appointment.scheduledAt)}</span><b class="status ${statusColors[appointment.status] || 'indigo'}">${appointment.status}</b><span>${appointmentAction(appointment)}</span></div>`).join('')}</div></section>`
}

function doctors() {
  return `<section class="panel full"><div class="panel-head"><div><h2>负责人排班</h2><p>8 位负责人 · 今日 42 个可采购单时段</p></div><button class="primary small" data-toast="排班编辑器已打开">编辑排班</button></div><div class="doctor-grid">${[['林负责人', '全科门诊', '32 号候诊', 'indigo'], ['沈负责人', '皮肤科', '18 号候诊', 'coral'], ['赵负责人', '康复理疗', '处理中', 'green'], ['周负责人', '营养咨询', '8 号候诊', 'amber'], ['陈负责人', '全科门诊', '午间休息', 'gray'], ['王负责人', '心理咨询', '6 号候诊', 'indigo']].map((doctor) => `<article><div class="doctor-avatar ${doctor[3]}">${doctor[0][0]}</div><div><strong>${doctor[0]}</strong><small>${doctor[1]}</small></div><span>${doctor[2]}</span><div class="schedule-line"><i style="width:78%"></i></div></article>`).join('')}</div></section>`
}

function patients() {
  return `<section class="panel full"><div class="panel-head"><div><h2>客户档案</h2><p>30 条虚构档案 · 仅用于界面演示</p></div><button class="link" data-toast="导出任务已创建">导出列表 ↓</button></div><div class="table"><div class="th"><span>客户 / 编号</span><span>最近科室</span><span>最近就诊</span><span>回访状态</span><span>操作</span></div>${[['林晓雨', 'CF-2038', '全科门诊', '07/16', '待回访'], ['沈明远', 'CF-2037', '皮肤科', '07/15', '进行中'], ['赵思涵', 'CF-2036', '康复理疗', '07/14', '已完成'], ['周子昂', 'CF-2035', '全科门诊', '07/13', '待回访'], ['许安然', 'CF-2034', '营养咨询', '07/12', '已完成']].map((patient) => `<div class="tr"><span><strong>${patient[0]}</strong><small>${patient[1]}</small></span><span>${patient[2]}</span><span>${patient[3]}</span><b class="status ${patient[4] === '已完成' ? 'green' : 'coral'}">${patient[4]}</b><button class="text-action" data-toast="${patient[0]} 档案已打开">查看档案</button></div>`).join('')}</div></section>`
}

function followups() {
  return `<section class="panel full"><div class="panel-head"><div><h2>回访任务</h2><p>${dataSource === 'API 数据' ? 'API 实时回访' : '12 条待跟进任务'} · 由负责人/护士确认后记录</p></div><span class="chip">全部任务　⌄</span></div><div class="follow-list">${followupTasks.map((item) => `<article><span class="task-icon ${item.status === '已完成' ? 'green' : 'coral'}">✓</span><div><strong>${item.id} · ${item.patient}</strong><p>${item.summary}</p><small>${item.dueAt} · ${dataSource === 'API 数据' ? 'API 数据' : '演示任务'}</small></div>${item.status === '已完成' ? '<button class="text-action" data-toast="该回访已经完成">查看</button>' : `<button class="text-action" data-action="complete-followup" data-followup-id="${item.id}">完成任务</button>`}</article>`).join('')}</div></section>`
}

function mobileView() {
  return `<section class="mobile-panel"><div class="mobile-panel__hero"><span>SUPPLYFLOW MOBILE</span><h2>我的就诊与回访</h2><p>客户端可在同一套闭环 API 中完成确认、候诊、处理和回访确认。</p><button class="primary" data-action="create-appointment">＋ 创建演示采购单</button></div><div class="mobile-list"><h3>今日采购单</h3>${appointments.slice(0, 4).map((appointment) => `<article class="mobile-card"><div><small>${timeLabel(appointment.scheduledAt)} · ${appointment.department}</small><strong>${appointment.patient}</strong><span>${appointment.doctor} · ${appointment.status}</span></div><b class="status ${statusColors[appointment.status] || 'indigo'}">${appointment.status}</b>${appointmentAction(appointment)}</article>`).join('')}</div><div class="mobile-list"><h3>我的回访</h3>${followupTasks.slice(0, 3).map((item) => `<article class="mobile-card"><div><small>${item.dueAt}</small><strong>${item.summary}</strong><span>${item.patient} · ${item.status}</span></div>${item.status === '已完成' ? '<b class="status green">已完成</b>' : `<button class="text-action" data-action="complete-followup" data-followup-id="${item.id}">完成回访</button>`}</article>`).join('')}</div></section>`
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
    if (!quiet) toast = '已从 SupplyFlow API 刷新数据'
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
    showToast(`${task.patient} 的回访已完成`)
  } catch (error) {
    dataSource = '演示数据'
    showToast(`接口暂不可用，已保留演示任务：${error.message}`)
  }
}

async function createAppointment() {
  try {
    const created = await api.createAppointment({ patient: '移动端演示客户', patientId: 'PT-MOBILE-DEMO', department: '全科门诊', doctor: '林负责人', scheduledAt: new Date().toISOString() })
    appointments = [normalizeAppointment(created), ...appointments]
    dataSource = 'API 数据'
    showToast('采购单已创建，可继续在移动端完成确认')
  } catch (error) {
    dataSource = '演示数据'
    showToast(`API 暂不可用，保留演示采购单：${error.message}`)
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
