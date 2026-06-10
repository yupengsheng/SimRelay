const routes = {
  dashboard: { title: '设备监控', subtitle: '实时查看设备状态与出口 IP' },
  devices: { title: '设备管理', subtitle: '查看设备信息、编辑配置、执行 AT 指令' },
  proxy: { title: '代理管理', subtitle: '管理本地出站代理和 VoWiFi 漫游前置代理' },
  sms: { title: '短信中心', subtitle: '按联系人聚合，点击进入会话明细' },
  logs: { title: '实时日志', subtitle: '查看系统运行日志，支持过滤和搜索' },
  settings: { title: '系统设置', subtitle: '管理网关参数与运行信息' },
}

const state = {
  token: localStorage.getItem('token') || '',
  route: normalizeRoute(location.hash.replace('#/', '') || 'dashboard'),
  devices: [],
  contacts: [],
  messages: [],
  logs: [],
  selectedDevice: 'all',
  selectedContact: '',
  selectedManagedDevice: '',
  activeTab: 'overview',
  trafficRange: 'day',
  logsPaused: false,
  composingNewSMS: false,
}

const $ = (selector) => document.querySelector(selector)
const $$ = (selector) => Array.from(document.querySelectorAll(selector))

const els = {
  toast: $('#toast'),
  loginView: $('#loginView'),
  app: $('#app'),
  loginForm: $('#loginForm'),
  loginUsername: $('#loginUsername'),
  loginPassword: $('#loginPassword'),
  loginBtn: $('#loginBtn'),
  logoutBtn: $('#logoutBtn'),
  pageTitle: $('#pageTitle'),
  pageSubtitle: $('#pageSubtitle'),
  refreshBtn: $('#refreshBtn'),
  rescanBtn: $('#rescanBtn'),
  addDeviceBtn: $('#addDeviceBtn'),
  newSmsBtn: $('#newSmsBtn'),
  themeBtn: $('#themeBtn'),
  sideDot: $('#sideDot'),
  sideStatus: $('#sideStatus'),
  dashboardView: $('#dashboardView'),
  devicesView: $('#devicesView'),
  proxyView: $('#proxyView'),
  smsView: $('#smsView'),
  logsView: $('#logsView'),
  settingsView: $('#settingsView'),
  totalDevices: $('#totalDevices'),
  onlineDevices: $('#onlineDevices'),
  offlineDevices: $('#offlineDevices'),
  lastRefresh: $('#lastRefresh'),
  dashboardDeviceCards: $('#dashboardDeviceCards'),
  trafficRows: $('#trafficRows'),
  trafficRx: $('#trafficRx'),
  trafficTx: $('#trafficTx'),
  trafficTotal: $('#trafficTotal'),
  trafficRxLabel: $('#trafficRxLabel'),
  trafficTxLabel: $('#trafficTxLabel'),
  trafficTotalLabel: $('#trafficTotalLabel'),
  smsThreadListView: $('#smsThreadListView'),
  smsChatView: $('#smsChatView'),
  smsDeviceSelect: $('#smsDeviceSelect'),
  contactSearch: $('#contactSearch'),
  contactList: $('#contactList'),
  chatTitle: $('#chatTitle'),
  chatMeta: $('#chatMeta'),
  backToThreadsBtn: $('#backToThreadsBtn'),
  deleteThreadBtn: $('#deleteThreadBtn'),
  messagePane: $('#messagePane'),
  sendForm: $('#sendForm'),
  recipientInput: $('#recipientInput'),
  messageInput: $('#messageInput'),
  encodingInfo: $('#encodingInfo'),
  sendBtn: $('#sendBtn'),
  deviceSearch: $('#deviceSearch'),
  deviceList: $('#deviceList'),
  deviceName: $('#deviceName'),
  deviceId: $('#deviceId'),
  publicIP: $('#publicIP'),
  openSmsBtn: $('#openSmsBtn'),
  rotateIpBtn: $('#rotateIpBtn'),
  rebootBtn: $('#rebootBtn'),
  quickAT: $('#quickAT'),
  atCommand: $('#atCommand'),
  atTimeout: $('#atTimeout'),
  clearATBtn: $('#clearATBtn'),
  sendATBtn: $('#sendATBtn'),
  atOutput: $('#atOutput'),
  ussdCode: $('#ussdCode'),
  ussdTimeout: $('#ussdTimeout'),
  clearUSSDBtn: $('#clearUSSDBtn'),
  sendUSSDBtn: $('#sendUSSDBtn'),
  ussdOutput: $('#ussdOutput'),
  logCount: $('#logCount'),
  logLevelFilter: $('#logLevelFilter'),
  logSearch: $('#logSearch'),
  logsList: $('#logsList'),
  pauseLogsBtn: $('#pauseLogsBtn'),
  clearLogsBtn: $('#clearLogsBtn'),
  exportLogsBtn: $('#exportLogsBtn'),
  autoTailLogs: $('#autoTailLogs'),
  settingsVersion: $('#settingsVersion'),
  settingsBuildTime: $('#settingsBuildTime'),
  settingsConfigPath: $('#settingsConfigPath'),
  updateCredentialBtn: $('#updateCredentialBtn'),
  saveNotifyBtn: $('#saveNotifyBtn'),
  addProxyBtn: $('#addProxyBtn'),
  networkSelectBtn: $('#networkSelectBtn'),
  flightModeBtn: $('#flightModeBtn'),
}

function normalizeRoute(route) {
  return Object.hasOwn(routes, route) ? route : 'dashboard'
}

function showToast(message, kind = 'error') {
  els.toast.textContent = message
  els.toast.className = `toast ${kind}`
  window.clearTimeout(showToast.timer)
  showToast.timer = window.setTimeout(() => els.toast.classList.add('hidden'), 3600)
}

async function api(path, options = {}) {
  const headers = { ...(options.headers || {}) }
  if (state.token) headers.Authorization = `Bearer ${state.token}`
  if (options.body && !headers['Content-Type']) headers['Content-Type'] = 'application/json'
  const res = await fetch(`/api${path}`, { ...options, headers })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    if (res.status === 401) logout()
    throw new Error(data.message || data.error || `请求失败：${res.status}`)
  }
  return data
}

function text(value, fallback = '--') {
  return value === undefined || value === null || value === '' ? fallback : String(value)
}

function setOnline(ok) {
  els.sideDot.classList.toggle('ok', ok)
  els.sideDot.classList.toggle('bad', !ok)
  els.sideStatus.textContent = ok ? '在线' : '异常'
}

function showApp() {
  els.loginView.classList.toggle('hidden', !!state.token)
  els.app.classList.toggle('hidden', !state.token)
  if (state.token) {
    navigate(state.route, { refresh: false })
    refreshAll()
  }
}

async function login(event) {
  event.preventDefault()
  els.loginBtn.disabled = true
  els.loginBtn.textContent = '登录中'
  try {
    const data = await api('/auth/login', {
      method: 'POST',
      body: JSON.stringify({
        username: els.loginUsername.value.trim(),
        password: els.loginPassword.value,
      }),
    })
    state.token = data.token || ''
    localStorage.setItem('token', state.token)
    showToast('登录成功', 'success')
    showApp()
  } catch (err) {
    showToast(err.message)
  } finally {
    els.loginBtn.disabled = false
    els.loginBtn.textContent = '登录'
  }
}

function logout() {
  state.token = ''
  localStorage.removeItem('token')
  els.app.classList.add('hidden')
  els.loginView.classList.remove('hidden')
}

function navigate(route, options = { refresh: true }) {
  state.route = normalizeRoute(route)
  if (location.hash !== `#/${state.route}`) location.hash = `#/${state.route}`

  $$('.nav-item').forEach((item) => item.classList.toggle('active', item.dataset.route === state.route))
  for (const routeName of Object.keys(routes)) {
    const view = els[`${routeName}View`]
    if (view) view.classList.toggle('hidden', routeName !== state.route)
  }

  els.pageTitle.textContent = routes[state.route].title
  els.pageSubtitle.textContent = routes[state.route].subtitle
  setRouteActions()
  if (options.refresh) refreshAll()
}

function setRouteActions() {
  els.refreshBtn.classList.remove('hidden')
  els.rescanBtn.classList.toggle('hidden', state.route !== 'devices')
  els.addDeviceBtn.classList.toggle('hidden', state.route !== 'devices')
  els.newSmsBtn.classList.toggle('hidden', state.route !== 'sms')
}

async function refreshAll() {
  els.refreshBtn.disabled = true
  els.refreshBtn.textContent = '刷新中'
  try {
    if (state.route === 'dashboard') {
      await loadDashboard()
    } else if (state.route === 'devices') {
      await loadDevices()
      await loadContacts({ keepSelection: true })
      renderManagedDevices()
      renderDeviceDetail()
    } else if (state.route === 'sms') {
      await loadDevices()
      await loadContacts({ keepSelection: true })
      if (state.selectedContact || state.composingNewSMS) await loadThread()
      renderSMSMode()
    } else if (state.route === 'logs') {
      await loadLogs()
    } else if (state.route === 'settings') {
      await loadSettings()
    } else if (state.route === 'proxy') {
      await loadProxy()
    }
    setOnline(true)
  } catch (err) {
    setOnline(false)
    showToast(err.message)
  } finally {
    els.refreshBtn.disabled = false
    els.refreshBtn.textContent = '刷新'
  }
}

async function loadDevices(endpoint = '/devices') {
  const data = await api(endpoint)
  state.devices = Array.isArray(data.devices) ? data.devices : []
  if (!state.selectedManagedDevice && state.devices[0]) state.selectedManagedDevice = state.devices[0].id
  if (state.selectedManagedDevice && !state.devices.some((device) => device.id === state.selectedManagedDevice) && state.devices[0]) {
    state.selectedManagedDevice = state.devices[0].id
  }
  renderSMSDevices()
  return state.devices
}

async function loadDashboard() {
  await loadDevices('/dashboard/devices')
  renderDashboard()
  await loadTraffic(state.trafficRange)
}

function renderDashboard() {
  const total = state.devices.length
  const online = state.devices.filter(isDeviceOnline).length
  els.totalDevices.textContent = String(total)
  els.onlineDevices.textContent = String(online)
  els.offlineDevices.textContent = String(total - online)
  els.lastRefresh.textContent = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })

  if (!state.devices.length) {
    els.dashboardDeviceCards.innerHTML = '<div class="panel empty-module">暂无设备</div>'
    return
  }
  els.dashboardDeviceCards.innerHTML = state.devices
    .map((device) => {
      const modem = device.modem || {}
      const onlineText = isDeviceOnline(device) ? '在线' : '离线'
      return `
        <button class="dashboard-device-card" type="button" data-dashboard-device="${escapeHTML(device.id)}">
          <span class="status-dot ${isDeviceOnline(device) ? 'ok' : 'bad'}"></span>
          <strong>${escapeHTML(device.name || device.id)}</strong>
          <span>${onlineText}</span>
          <span>${escapeHTML(modem.operator || modem.native_spn || '运营商未知')}</span>
          <span>公网 IP ${escapeHTML(device.public_ip || '---')}</span>
        </button>
      `
    })
    .join('')
}

async function loadTraffic(range = 'day') {
  state.trafficRange = range
  const data = await api(`/traffic/analysis?range=${encodeURIComponent(range)}`)
  const buckets = Array.isArray(data.buckets) ? data.buckets : []
  const totals = buckets.reduce(
    (acc, bucket) => {
      acc.rx += Number(bucket.rx_bytes || bucket.rx || 0)
      acc.tx += Number(bucket.tx_bytes || bucket.tx || 0)
      acc.total += Number(bucket.total_bytes || bucket.total || 0)
      return acc
    },
    { rx: 0, tx: 0, total: 0 },
  )
  const label = range === 'week' ? '本周' : range === 'month' ? '本月' : '本日'
  els.trafficRxLabel.textContent = `${label}下载`
  els.trafficTxLabel.textContent = `${label}上传`
  els.trafficTotalLabel.textContent = `${label}合计`
  els.trafficRx.textContent = formatBytes(totals.rx)
  els.trafficTx.textContent = formatBytes(totals.tx)
  els.trafficTotal.textContent = formatBytes(totals.total || totals.rx + totals.tx)
  els.trafficRows.innerHTML = buckets
    .map(
      (bucket) => `
        <tr>
          <td>${escapeHTML(bucket.bucket || bucket.time || '--')}</td>
          <td>${formatBytes(bucket.rx_bytes || bucket.rx || 0)}</td>
          <td>${formatBytes(bucket.tx_bytes || bucket.tx || 0)}</td>
          <td>${formatBytes(bucket.total_bytes || bucket.total || 0)}</td>
        </tr>
      `,
    )
    .join('')
}

async function loadContacts(options = {}) {
  const params = new URLSearchParams({ limit: '200' })
  if (state.selectedDevice !== 'all') params.set('device_id', state.selectedDevice)
  const contacts = await api(`/sms/contacts?${params}`)
  state.contacts = Array.isArray(contacts) ? contacts : []
  if (!options.keepSelection && state.contacts[0]) state.selectedContact = contactKey(state.contacts[0])
  if (state.selectedContact && !state.contacts.some((contact) => contactKey(contact) === state.selectedContact)) {
    state.selectedContact = ''
  }
  renderContacts()
}

async function loadThread() {
  const contact = selectedContact()
  if (!contact) {
    state.messages = []
    renderMessages()
    return
  }
  const params = new URLSearchParams({ peer: contact.peer, limit: '80' })
  if (state.selectedDevice === 'all') {
    params.set('device_id', 'all')
    params.set('imsi', contact.imsi || '')
  } else {
    params.set('device_id', state.selectedDevice)
  }
  const messages = await api(`/sms/thread?${params}`)
  state.messages = Array.isArray(messages) ? messages : []
  renderMessages()
}

function contactKey(contact) {
  return `${contact.device_id || ''}|${contact.imsi || ''}|${contact.peer || ''}`
}

function selectedContact() {
  return state.contacts.find((contact) => contactKey(contact) === state.selectedContact) || null
}

function renderSMSDevices() {
  if (!els.smsDeviceSelect) return
  const choices = [{ id: 'all', name: '全部设备', healthy: true }, ...state.devices]
  els.smsDeviceSelect.innerHTML = ''
  for (const device of choices) {
    const option = document.createElement('option')
    option.value = device.id
    option.textContent = device.name || device.id
    option.selected = option.value === state.selectedDevice
    els.smsDeviceSelect.append(option)
  }
}

function renderContacts() {
  const query = els.contactSearch.value.trim().toLowerCase()
  const contacts = query
    ? state.contacts.filter((contact) => `${contact.peer || ''} ${contact.last_content || ''}`.toLowerCase().includes(query))
    : state.contacts

  if (!contacts.length) {
    els.contactList.innerHTML = '<div class="empty-state">暂无短信会话</div>'
    return
  }
  els.contactList.innerHTML = contacts
    .map((contact) => {
      const key = contactKey(contact)
      return `
        <button class="contact-item ${key === state.selectedContact ? 'active' : ''}" data-contact="${escapeHTML(key)}">
          <div class="contact-avatar">${escapeHTML(String(contact.peer || '?').slice(-2))}</div>
          <div class="contact-body">
            <div class="contact-top">
              <strong>${escapeHTML(contact.peer || '未知号码')}</strong>
              <span>${formatTime(contact.last_timestamp)}</span>
            </div>
            <p>${escapeHTML(contact.last_content || '(空短信)')}</p>
            <div class="contact-meta">
              <span>${escapeHTML(contact.device_name || contact.device_id || 'EC20')}</span>
              ${contact.unread_count ? `<b>${contact.unread_count}</b>` : ''}
            </div>
          </div>
        </button>
      `
    })
    .join('')
}

function renderSMSMode() {
  const showChat = Boolean(state.selectedContact || state.composingNewSMS)
  els.smsThreadListView.classList.toggle('hidden', showChat)
  els.smsChatView.classList.toggle('hidden', !showChat)
  if (!showChat) {
    state.messages = []
    renderContacts()
  } else {
    renderMessages()
  }
}

function renderMessages() {
  const contact = selectedContact()
  const inComposeMode = state.composingNewSMS && !contact
  els.deleteThreadBtn.classList.toggle('hidden', !contact)
  if (contact) els.recipientInput.value = contact.peer
  if (inComposeMode && !els.recipientInput.value) els.recipientInput.value = ''

  els.chatTitle.textContent = contact ? contact.peer : inComposeMode ? '新建短信' : '请选择会话'
  els.chatMeta.textContent = contact
    ? `${contact.device_name || contact.device_id || 'EC20'} · ${state.messages.length} 条短信`
    : inComposeMode
      ? '填写号码和内容后发送'
      : '短信内容会显示在这里'

  if (inComposeMode) {
    els.messagePane.className = 'message-pane empty'
    els.messagePane.textContent = '新短信'
    return
  }
  if (!contact) {
    els.messagePane.className = 'message-pane empty'
    els.messagePane.textContent = '暂无会话'
    return
  }
  if (!state.messages.length) {
    els.messagePane.className = 'message-pane empty'
    els.messagePane.textContent = '没有短信'
    return
  }

  els.messagePane.className = 'message-pane'
  let lastDate = ''
  els.messagePane.innerHTML = state.messages
    .map((message) => {
      const date = formatDate(message.timestamp)
      const dateHTML = date !== lastDate ? `<div class="date-chip">${escapeHTML(date)}</div>` : ''
      lastDate = date
      const outgoing = Number(message.type) === 2 || String(message.status || '').includes('STO')
      return `
        ${dateHTML}
        <div class="message-row ${outgoing ? 'outgoing' : 'incoming'}">
          <div class="message-bubble">
            <div class="message-content">${escapeHTML(message.content || '')}</div>
            <div class="message-foot">
              <span>${formatTime(message.timestamp)}</span>
              <button class="delete-message" data-message-id="${message.id}" title="删除短信">删除</button>
            </div>
          </div>
        </div>
      `
    })
    .join('')
  els.messagePane.scrollTop = els.messagePane.scrollHeight
}

function renderManagedDevices() {
  const query = els.deviceSearch.value.trim().toLowerCase()
  const devices = query
    ? state.devices.filter((device) => `${device.id} ${device.name} ${device.interface} ${device.modem?.imei || ''} ${device.modem?.iccid || ''}`.toLowerCase().includes(query))
    : state.devices
  if (!devices.length) {
    els.deviceList.innerHTML = '<div class="empty-state">暂无设备</div>'
    return
  }
  els.deviceList.innerHTML = devices
    .map((device) => {
      const modem = device.modem || {}
      const online = isDeviceOnline(device)
      return `
        <button class="managed-device ${device.id === state.selectedManagedDevice ? 'active' : ''}" data-managed-device="${escapeHTML(device.id)}">
          <div>
            <strong>${escapeHTML(device.name || device.id)}</strong>
            <span>${escapeHTML(device.interface || device.at_port || '--')} · ${escapeHTML(device.id)}</span>
            <small>${escapeHTML(modem.operator || '运营商未知')} · ${escapeHTML(modem.network_mode || 'LTE')} · ${device.network_enabled ? '数据已开启' : '数据未开启'}</small>
          </div>
          <b class="${online ? 'ok' : 'bad'}">${online ? '在线' : '离线'}</b>
        </button>
      `
    })
    .join('')
}

function selectedDeviceDetail() {
  return state.devices.find((device) => device.id === state.selectedManagedDevice) || state.devices[0] || null
}

function renderDeviceDetail() {
  const device = selectedDeviceDetail()
  if (!device) return
  state.selectedManagedDevice = device.id
  const modem = device.modem || {}
  const operator = modem.operator || modem.native_spn || '运营商未知'
  const registered = device.radio_registered || modem.reg_status === 1
  els.deviceName.textContent = device.name || device.id
  els.deviceId.textContent = device.interface || device.id
  els.publicIP.textContent = device.public_ip || '---'
  $('#operatorValue').textContent = operator
  $('#operatorSubValue').textContent = modem.network_mode ? `· ${modem.network_mode}` : '· LTE'
  $('#regValue').textContent = registered ? '已注册' : '未注册'
  $('#simValue').textContent = `SIM ${text(modem.sim || (device.healthy ? 'READY' : 'UNKNOWN'))}`
  $('#signalValue').textContent = modem.signal_dbm ? `${modem.signal_dbm} dBm` : text(modem.signal_rssi)
  $('#signalSubValue').textContent = `RSSI ${text(modem.signal_rssi)} · BER ${text(modem.signal_ber)} · SINR ${text(modem.signal_sinr, '0')}`
  $('#networkModeValue').textContent = text(modem.network_mode, 'LTE')
  $('#bandValue').textContent = text(modem.radio_band)
  $('#channelValue').textContent = text(modem.radio_channel)
  $('#manufacturerValue').textContent = text(`${text(modem.manufacturer, 'Quectel')} ${text(modem.model || device.name, '')}`.trim())
  $('#imeiValue').textContent = text(modem.imei)
  $('#iccidValue').textContent = text(modem.iccid)
  $('#imsiValue').textContent = text(modem.imsi)
  $('#localNumberValue').textContent = text(modem.local_phone)
  $('#homeOperatorValue').textContent = text(modem.home_operator || modem.operator)
  $('#firmwareValue').textContent = text(modem.firmware_revision || modem.firmware)
  $('#backendModeValue').textContent = text((device.backend_mode || device.device_backend || 'AT').toUpperCase())
  $('#dataStateValue').textContent = device.network_enabled ? (device.network_connected ? '数据已连接' : '数据未连接') : '数据未开启'
  renderConfig(device)
}

function renderConfig(device) {
  const modem = device.modem || {}
  setText('#configId', device.id)
  setText('#configName', device.name || device.id)
  setText('#configIMEI', modem.imei)
  setText('#configUSBPath', device.usb_path)
  setText('#configInterface', device.interface)
  setText('#configATPort', device.at_port || device.interface)
  setText('#configControlDevice', device.control_device)
  setText('#configAPN', device.apn)
}

async function loadLogs() {
  const data = await api('/logs')
  state.logs = Array.isArray(data.logs) ? data.logs : []
  renderLogs()
}

function renderLogs() {
  const level = els.logLevelFilter.value || 'all'
  const query = els.logSearch.value.trim().toLowerCase()
  const rows = state.logs.filter((item) => {
    const levelMatches = level === 'all' || String(item.level || '').toUpperCase() === level
    const queryMatches = !query || `${item.ts || ''} ${item.level || ''} ${item.source || ''} ${item.message || ''}`.toLowerCase().includes(query)
    return levelMatches && queryMatches
  })
  els.logCount.textContent = `${rows.length} 条日志`
  if (!rows.length) {
    els.logsList.innerHTML = '<div class="empty-state">暂无日志</div>'
    return
  }
  els.logsList.innerHTML = rows
    .map(
      (item) => `
        <div class="log-row">
          <span class="log-time">${escapeHTML(item.ts || '--')}</span>
          <b class="log-level ${escapeHTML(String(item.level || 'INFO').toLowerCase())}">${escapeHTML(item.level || 'INFO')}</b>
          <span class="log-source">${escapeHTML(item.source || 'simrelay')}</span>
          <span class="log-message">${escapeHTML(item.message || '')}</span>
        </div>
      `,
    )
    .join('')
  if (els.autoTailLogs.checked) els.logsList.scrollTop = els.logsList.scrollHeight
}

async function loadSettings() {
  const data = await api('/settings')
  els.settingsVersion.textContent = text(data.version, 'simrelay')
  els.settingsBuildTime.textContent = text(data.build_time)
  els.settingsConfigPath.textContent = text(data.config_path, 'environment')
}

async function loadProxy() {
  await api('/proxies')
}

async function sendMessage(event) {
  event.preventDefault()
  const phone = els.recipientInput.value.trim()
  const message = els.messageInput.value
  if (!phone || !message) {
    showToast('收件号码和短信内容不能为空')
    return
  }
  els.sendBtn.disabled = true
  els.sendBtn.textContent = '发送中'
  try {
    const data = await api('/sms/send', {
      method: 'POST',
      body: JSON.stringify({ device_id: state.selectedDevice === 'all' ? '' : state.selectedDevice, phone, message }),
    })
    els.messageInput.value = ''
    updateEncoding()
    state.composingNewSMS = false
    showToast(`发送成功${data.reference ? `，引用号 ${data.reference}` : ''}`, 'success')
    await loadContacts({ keepSelection: true })
    if (state.selectedContact) await loadThread()
    renderSMSMode()
  } catch (err) {
    showToast(err.message)
  } finally {
    els.sendBtn.disabled = false
    els.sendBtn.textContent = '发送'
  }
}

async function deleteMessage(id) {
  if (!confirm('确定删除这条短信？')) return
  await api(`/sms/messages/${id}`, { method: 'DELETE' })
  showToast('短信已删除', 'success')
  await loadContacts({ keepSelection: true })
  await loadThread()
}

async function deleteThread() {
  const contact = selectedContact()
  if (!contact || !confirm(`确定删除与 ${contact.peer} 的会话？`)) return
  await api(`/sms/thread?peer=${encodeURIComponent(contact.peer)}&device_id=${encodeURIComponent(contact.device_id || '')}&imsi=${encodeURIComponent(contact.imsi || '')}`, {
    method: 'DELETE',
  })
  state.selectedContact = ''
  state.composingNewSMS = false
  showToast('会话已删除', 'success')
  await loadContacts({ keepSelection: true })
  renderSMSMode()
}

async function sendAT() {
  const device = selectedDeviceDetail()
  if (!device) return
  const command = els.atCommand.value.trim()
  if (!command) return
  els.sendATBtn.disabled = true
  els.atOutput.textContent = '执行中...'
  try {
    const data = await api(`/devices/${encodeURIComponent(device.id)}/actions/at`, {
      method: 'POST',
      body: JSON.stringify({ command, timeout: Number(els.atTimeout.value || 10) }),
    })
    els.atOutput.textContent = data.response || '(无响应)'
  } catch (err) {
    els.atOutput.textContent = err.message
  } finally {
    els.sendATBtn.disabled = false
  }
}

async function sendUSSD() {
  const device = selectedDeviceDetail()
  if (!device) return
  const code = els.ussdCode.value.trim()
  if (!code) return
  els.sendUSSDBtn.disabled = true
  els.ussdOutput.textContent = '发送中...'
  try {
    const data = await api(`/devices/${encodeURIComponent(device.id)}/actions/ussd`, {
      method: 'POST',
      body: JSON.stringify({ code, timeout: Number(els.ussdTimeout.value || 30) }),
    })
    els.ussdOutput.textContent = data.result?.text || data.result?.raw_text || JSON.stringify(data.result || data, null, 2)
  } catch (err) {
    els.ussdOutput.textContent = err.message
  } finally {
    els.sendUSSDBtn.disabled = false
  }
}

async function rebootModem() {
  const device = selectedDeviceDetail()
  if (!device || !confirm(`确定重启设备 ${device.id}？`)) return
  await api(`/devices/${encodeURIComponent(device.id)}/actions/reboot`, { method: 'POST' })
  showToast('重启指令已送达', 'success')
}

async function rescanDevices() {
  await api('/devices/actions/rescan', { method: 'POST' })
  showToast('已触发重新扫描', 'success')
  await refreshAll()
}

function updateEncoding() {
  const value = els.messageInput.value || ''
  const length = Array.from(value).length
  const gsm = /^[\u000A\u000D\u0020-\u007E]*$/.test(value)
  const parts = gsm ? (length <= 160 ? 1 : Math.ceil(length / 153)) : length <= 70 ? 1 : Math.ceil(length / 67)
  els.encodingInfo.textContent = `${gsm ? 'GSM7' : 'UCS2'} · ${length} 字符 · ${parts} 段`
}

function switchTab(tab) {
  if (!tab) return
  state.activeTab = tab
  $$('.device-detail .tab').forEach((item) => item.classList.toggle('active', item.dataset.tab === tab))
  for (const panel of ['overview', 'esim', 'at', 'ussd', 'config']) {
    $(`#${panel}Tab`).classList.toggle('hidden', panel !== tab)
  }
}

function backToThreads() {
  state.selectedContact = ''
  state.composingNewSMS = false
  state.messages = []
  els.recipientInput.value = ''
  els.messageInput.value = ''
  updateEncoding()
  renderSMSMode()
}

function newSMS() {
  state.selectedContact = ''
  state.composingNewSMS = true
  state.messages = []
  els.recipientInput.value = ''
  els.messageInput.value = ''
  updateEncoding()
  renderSMSMode()
  setTimeout(() => els.recipientInput.focus(), 0)
}

function exportLogs() {
  const content = state.logs.map((item) => `${item.ts || ''}\t${item.level || ''}\t${item.source || ''}\t${item.message || ''}`).join('\n')
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `simrelay-logs-${Date.now()}.txt`
  link.click()
  URL.revokeObjectURL(url)
}

function isDeviceOnline(device) {
  return Boolean(device.running && (device.control_online ?? device.healthy))
}

function formatTime(value) {
  const date = parseDate(value)
  return date ? date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }) : ''
}

function formatDate(value) {
  const date = parseDate(value)
  return date ? date.toLocaleDateString() : '未知日期'
}

function parseDate(value) {
  if (!value) return null
  if (/^\d{2}\/\d{2}\/\d{2},/.test(value)) {
    const match = value.match(/^(\d{2})\/(\d{2})\/(\d{2}),(\d{2}):(\d{2}):(\d{2})/)
    if (match) return new Date(2000 + Number(match[1]), Number(match[2]) - 1, Number(match[3]), Number(match[4]), Number(match[5]), Number(match[6]))
  }
  const date = new Date(value)
  return Number.isFinite(date.getTime()) ? date : null
}

function formatBytes(value) {
  const bytes = Number(value || 0)
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char])
}

function setText(selector, value, fallback = '--') {
  const node = $(selector)
  if (node) node.textContent = text(value, fallback)
}

els.loginForm.addEventListener('submit', login)
els.logoutBtn.addEventListener('click', logout)
els.refreshBtn.addEventListener('click', refreshAll)
els.rescanBtn.addEventListener('click', rescanDevices)
els.addDeviceBtn.addEventListener('click', () => showToast('SimRelay 当前通过环境变量绑定单个 EC20 设备'))
els.newSmsBtn.addEventListener('click', newSMS)
els.themeBtn.addEventListener('click', () => {
  document.documentElement.classList.toggle('dark')
  localStorage.setItem('theme', document.documentElement.classList.contains('dark') ? 'dark' : 'light')
})
$$('.nav-item').forEach((item) => item.addEventListener('click', () => navigate(item.dataset.route)))
$$('[data-range]').forEach((button) => {
  button.addEventListener('click', async () => {
    $$('.segment[data-range]').forEach((item) => item.classList.toggle('active', item === button))
    await loadTraffic(button.dataset.range)
  })
})
els.dashboardDeviceCards.addEventListener('click', (event) => {
  const button = event.target.closest('[data-dashboard-device]')
  if (!button) return
  state.selectedManagedDevice = button.dataset.dashboardDevice
  navigate('devices')
})
els.smsDeviceSelect.addEventListener('change', async () => {
  state.selectedDevice = els.smsDeviceSelect.value || 'all'
  await loadContacts({ keepSelection: true })
  if (state.selectedContact) await loadThread()
  renderSMSMode()
})
els.contactSearch.addEventListener('input', renderContacts)
els.contactList.addEventListener('click', async (event) => {
  const button = event.target.closest('[data-contact]')
  if (!button) return
  state.selectedContact = button.dataset.contact
  state.composingNewSMS = false
  renderContacts()
  await loadThread()
  renderSMSMode()
})
els.backToThreadsBtn.addEventListener('click', backToThreads)
els.messagePane.addEventListener('click', async (event) => {
  const button = event.target.closest('[data-message-id]')
  if (button) await deleteMessage(button.dataset.messageId)
})
els.sendForm.addEventListener('submit', sendMessage)
els.deleteThreadBtn.addEventListener('click', deleteThread)
els.messageInput.addEventListener('input', updateEncoding)
els.messageInput.addEventListener('keydown', (event) => {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    els.sendForm.requestSubmit()
  }
})
els.deviceSearch.addEventListener('input', renderManagedDevices)
els.deviceList.addEventListener('click', (event) => {
  const button = event.target.closest('[data-managed-device]')
  if (!button) return
  state.selectedManagedDevice = button.dataset.managedDevice
  renderManagedDevices()
  renderDeviceDetail()
})
els.openSmsBtn.addEventListener('click', () => navigate('sms'))
els.rotateIpBtn.addEventListener('click', () => showToast('SimRelay 当前未接管数据网络，无法切换 IP'))
els.rebootBtn.addEventListener('click', rebootModem)
els.networkSelectBtn.addEventListener('click', () => showToast('SimRelay 当前未接管网络选择'))
els.flightModeBtn.addEventListener('click', () => showToast('SimRelay 当前未接管飞行模式'))
els.quickAT.addEventListener('change', () => {
  els.atCommand.value = els.quickAT.value
})
els.clearATBtn.addEventListener('click', () => {
  els.atOutput.textContent = '等待执行 AT 命令'
})
els.sendATBtn.addEventListener('click', sendAT)
els.clearUSSDBtn.addEventListener('click', () => {
  els.ussdOutput.textContent = '等待发送 USSD'
})
els.sendUSSDBtn.addEventListener('click', sendUSSD)
$$('.device-detail .tab').forEach((tab) => tab.addEventListener('click', () => switchTab(tab.dataset.tab)))
els.logLevelFilter.addEventListener('change', renderLogs)
els.logSearch.addEventListener('input', renderLogs)
els.pauseLogsBtn.addEventListener('click', () => {
  state.logsPaused = !state.logsPaused
  els.pauseLogsBtn.textContent = state.logsPaused ? '继续' : '暂停'
})
els.clearLogsBtn.addEventListener('click', () => {
  state.logs = []
  renderLogs()
})
els.exportLogsBtn.addEventListener('click', exportLogs)
els.updateCredentialBtn.addEventListener('click', () => showToast('SimRelay 当前通过环境变量配置登录凭证'))
els.saveNotifyBtn.addEventListener('click', async () => {
  const data = await api('/settings', { method: 'PUT', body: JSON.stringify({ notify: {} }) })
  showToast(data.message || '保存成功', 'success')
})
els.addProxyBtn.addEventListener('click', () => showToast('SimRelay 当前未接管 VoWiFi 代理'))
window.addEventListener('hashchange', () => {
  const route = normalizeRoute(location.hash.replace('#/', '') || 'dashboard')
  if (route !== state.route) navigate(route)
})

updateEncoding()
showApp()
