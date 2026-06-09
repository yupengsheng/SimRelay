const state = {
  token: localStorage.getItem('token') || '',
  route: location.hash.replace('#/', '') || 'sms',
  devices: [],
  contacts: [],
  messages: [],
  selectedDevice: 'all',
  selectedContact: '',
  selectedManagedDevice: '',
  activeTab: 'overview',
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
  themeBtn: $('#themeBtn'),
  sideDot: $('#sideDot'),
  sideStatus: $('#sideStatus'),
  smsView: $('#smsView'),
  devicesView: $('#devicesView'),
  smsDeviceSelect: $('#smsDeviceSelect'),
  smsDeviceList: $('#smsDeviceList'),
  contactSearch: $('#contactSearch'),
  contactList: $('#contactList'),
  contactCount: $('#contactCount'),
  chatTitle: $('#chatTitle'),
  chatMeta: $('#chatMeta'),
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
  sendATBtn: $('#sendATBtn'),
  atOutput: $('#atOutput'),
  ussdCode: $('#ussdCode'),
  sendUSSDBtn: $('#sendUSSDBtn'),
  ussdOutput: $('#ussdOutput'),
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
  els.sideStatus.textContent = ok ? '控制面在线' : '控制面异常'
}

function showApp() {
  els.loginView.classList.toggle('hidden', !!state.token)
  els.app.classList.toggle('hidden', !state.token)
  if (state.token) {
    navigate(state.route)
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

function navigate(route) {
  state.route = route === 'devices' ? 'devices' : 'sms'
  location.hash = `#/${state.route}`
  $$('.nav-item').forEach((item) => item.classList.toggle('active', item.dataset.route === state.route))
  els.smsView.classList.toggle('hidden', state.route !== 'sms')
  els.devicesView.classList.toggle('hidden', state.route !== 'devices')
  els.pageTitle.textContent = state.route === 'sms' ? '短信' : '设备管理'
  els.pageSubtitle.textContent =
    state.route === 'sms'
      ? '按联系人查看短信会话，发送中文短信。'
      : '查看设备信息、编辑配置、执行 AT 指令'
}

async function refreshAll() {
  els.refreshBtn.disabled = true
  els.refreshBtn.textContent = '刷新中'
  try {
    await loadDevices()
    if (state.route === 'sms') {
      await loadContacts()
      await loadThread()
    } else {
      renderManagedDevices()
      renderDeviceDetail()
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

async function loadDevices() {
  const data = await api('/devices')
  state.devices = Array.isArray(data.devices) ? data.devices : []
  if (!state.selectedManagedDevice && state.devices[0]) state.selectedManagedDevice = state.devices[0].id
  renderSMSDevices()
}

async function loadContacts() {
  const params = new URLSearchParams({ limit: '200' })
  if (state.selectedDevice !== 'all') params.set('device_id', state.selectedDevice)
  state.contacts = await api(`/sms/contacts?${params}`)
  if (!Array.isArray(state.contacts)) state.contacts = []
  if (!state.contacts.some((contact) => contactKey(contact) === state.selectedContact)) {
    state.selectedContact = state.contacts[0] ? contactKey(state.contacts[0]) : ''
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
  state.messages = await api(`/sms/thread?${params}`)
  if (!Array.isArray(state.messages)) state.messages = []
  renderMessages()
}

function contactKey(contact) {
  return `${contact.device_id || ''}|${contact.imsi || ''}|${contact.peer || ''}`
}

function selectedContact() {
  return state.contacts.find((contact) => contactKey(contact) === state.selectedContact) || null
}

function renderSMSDevices() {
  const choices = [{ id: 'all', name: '全部设备', healthy: true }, ...state.devices]
  els.smsDeviceSelect.innerHTML = ''
  for (const device of choices) {
    const option = document.createElement('option')
    option.value = device.id
    option.textContent = device.name || device.id
    option.selected = option.value === state.selectedDevice
    els.smsDeviceSelect.append(option)
  }

  els.smsDeviceList.innerHTML = choices
    .map(
      (device) => `
      <button class="device-mini ${device.id === state.selectedDevice ? 'active' : ''}" data-device="${escapeHTML(device.id)}">
        <span class="status-dot ${device.healthy === false ? 'bad' : 'ok'}"></span>
        <span>${escapeHTML(device.name || device.id)}</span>
      </button>
    `,
    )
    .join('')
}

function renderContacts() {
  const query = els.contactSearch.value.trim().toLowerCase()
  const contacts = query
    ? state.contacts.filter((contact) => {
        return `${contact.peer || ''} ${contact.last_content || ''}`.toLowerCase().includes(query)
      })
    : state.contacts

  els.contactCount.textContent = `${contacts.length} 个会话`
  if (!contacts.length) {
    els.contactList.innerHTML = '<div class="empty-state">暂无短信会话</div>'
    return
  }
  els.contactList.innerHTML = contacts
    .map((contact) => {
      const key = contactKey(contact)
      return `
        <button class="contact-item ${key === state.selectedContact ? 'active' : ''}" data-contact="${escapeHTML(key)}">
          <div class="contact-top">
            <strong>${escapeHTML(contact.peer || '未知号码')}</strong>
            <span>${formatTime(contact.last_timestamp)}</span>
          </div>
          <p>${escapeHTML(contact.last_content || '(空短信)')}</p>
          <div class="contact-meta">
            <span>${escapeHTML(contact.device_name || contact.device_id || 'EC20')}</span>
            ${contact.unread_count ? `<b>${contact.unread_count}</b>` : ''}
          </div>
        </button>
      `
    })
    .join('')
}

function renderMessages() {
  const contact = selectedContact()
  els.deleteThreadBtn.classList.toggle('hidden', !contact)
  els.recipientInput.value = contact ? contact.peer : els.recipientInput.value
  els.chatTitle.textContent = contact ? contact.peer : '请选择会话'
  els.chatMeta.textContent = contact ? `${contact.device_name || contact.device_id || 'EC20'} · ${state.messages.length} 条短信` : '短信内容会显示在这里'

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
    ? state.devices.filter((device) => `${device.id} ${device.name} ${device.interface} ${device.modem?.imei || ''}`.toLowerCase().includes(query))
    : state.devices
  if (!devices.length) {
    els.deviceList.innerHTML = '<div class="empty-state">暂无设备</div>'
    return
  }
  els.deviceList.innerHTML = devices
    .map((device) => {
      const online = device.running && (device.control_online ?? device.healthy)
      return `
        <button class="managed-device ${device.id === state.selectedManagedDevice ? 'active' : ''}" data-managed-device="${escapeHTML(device.id)}">
          <div>
            <strong>${escapeHTML(device.name || device.id)}</strong>
            <span>${escapeHTML(device.id)} · ${escapeHTML(device.interface || '--')}</span>
            <small>${escapeHTML(device.modem?.operator || 'EC20')} · ${escapeHTML(device.modem?.network_mode || 'LTE')}</small>
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
  els.deviceName.textContent = device.name || device.id
  els.deviceId.textContent = device.id
  els.publicIP.textContent = device.public_ip || '---'
  $('#simValue').textContent = text(modem.sim || (device.healthy ? 'READY' : 'UNKNOWN'))
  $('#regValue').textContent = device.radio_registered ? '已注册' : '未注册'
  $('#signalValue').textContent = modem.signal_dbm ? `${modem.signal_dbm} dBm` : text(modem.signal_rssi)
  $('#smsCountValue').textContent = String(state.contacts.length)
  $('#manufacturerValue').textContent = text(modem.manufacturer || 'Quectel')
  $('#modelValue').textContent = text(modem.model || device.name)
  $('#imeiValue').textContent = text(modem.imei)
  $('#portValue').textContent = text(device.at_port || device.interface)
  $('#networkModeValue').textContent = text(modem.network_mode, 'LTE')
  $('#berValue').textContent = text(modem.signal_ber)
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
    showToast(`发送成功${data.reference ? `，引用号 ${data.reference}` : ''}`, 'success')
    await loadContacts()
    await loadThread()
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
  await loadContacts()
  await loadThread()
}

async function deleteThread() {
  const contact = selectedContact()
  if (!contact || !confirm(`确定删除与 ${contact.peer} 的会话？`)) return
  await api(`/sms/thread?peer=${encodeURIComponent(contact.peer)}&device_id=${encodeURIComponent(contact.device_id || '')}&imsi=${encodeURIComponent(contact.imsi || '')}`, {
    method: 'DELETE',
  })
  state.selectedContact = ''
  showToast('会话已删除', 'success')
  await loadContacts()
  await loadThread()
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
      body: JSON.stringify({ command }),
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
      body: JSON.stringify({ code }),
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

function updateEncoding() {
  const value = els.messageInput.value || ''
  const length = Array.from(value).length
  const gsm = /^[\u000A\u000D\u0020-\u007E]*$/.test(value)
  const parts = gsm ? (length <= 160 ? 1 : Math.ceil(length / 153)) : length <= 70 ? 1 : Math.ceil(length / 67)
  els.encodingInfo.textContent = `${gsm ? 'GSM7' : 'UCS2'} · ${length} 字符 · ${parts} 段`
}

function switchTab(tab) {
  state.activeTab = tab
  $$('.tab').forEach((item) => item.classList.toggle('active', item.dataset.tab === tab))
  for (const panel of ['overview', 'at', 'ussd', 'config']) {
    $(`#${panel}Tab`).classList.toggle('hidden', panel !== tab)
  }
}

function formatTime(value) {
  const date = parseDate(value)
  return date ? date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : ''
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

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char])
}

els.loginForm.addEventListener('submit', login)
els.logoutBtn.addEventListener('click', logout)
els.refreshBtn.addEventListener('click', refreshAll)
els.themeBtn.addEventListener('click', () => {
  document.documentElement.classList.toggle('dark')
  localStorage.setItem('theme', document.documentElement.classList.contains('dark') ? 'dark' : 'light')
})
$$('.nav-item').forEach((item) => {
  item.addEventListener('click', () => {
    if (item.classList.contains('disabled')) {
      showToast('SimRelay 当前未实现该模块')
      return
    }
    navigate(item.dataset.route)
    refreshAll()
  })
})
els.smsDeviceSelect.addEventListener('change', async () => {
  state.selectedDevice = els.smsDeviceSelect.value || 'all'
  await loadContacts()
  await loadThread()
  renderSMSDevices()
})
els.smsDeviceList.addEventListener('click', async (event) => {
  const button = event.target.closest('[data-device]')
  if (!button) return
  state.selectedDevice = button.dataset.device
  await loadContacts()
  await loadThread()
  renderSMSDevices()
})
els.contactSearch.addEventListener('input', renderContacts)
els.contactList.addEventListener('click', async (event) => {
  const button = event.target.closest('[data-contact]')
  if (!button) return
  state.selectedContact = button.dataset.contact
  renderContacts()
  await loadThread()
})
els.messagePane.addEventListener('click', async (event) => {
  const button = event.target.closest('[data-message-id]')
  if (button) await deleteMessage(button.dataset.messageId)
})
els.sendForm.addEventListener('submit', sendMessage)
els.deleteThreadBtn.addEventListener('click', deleteThread)
els.messageInput.addEventListener('input', updateEncoding)
els.deviceSearch.addEventListener('input', renderManagedDevices)
els.deviceList.addEventListener('click', (event) => {
  const button = event.target.closest('[data-managed-device]')
  if (!button) return
  state.selectedManagedDevice = button.dataset.managedDevice
  renderManagedDevices()
  renderDeviceDetail()
})
els.openSmsBtn.addEventListener('click', () => navigate('sms'))
els.rotateIpBtn.addEventListener('click', () => showToast('SimRelay 单设备短信版本暂不接管数据网络'))
els.rebootBtn.addEventListener('click', rebootModem)
els.quickAT.addEventListener('change', () => {
  els.atCommand.value = els.quickAT.value
})
els.sendATBtn.addEventListener('click', sendAT)
els.sendUSSDBtn.addEventListener('click', sendUSSD)
$$('.tab').forEach((tab) => tab.addEventListener('click', () => switchTab(tab.dataset.tab)))
window.addEventListener('hashchange', () => navigate(location.hash.replace('#/', '') || 'sms'))

updateEncoding()
showApp()
