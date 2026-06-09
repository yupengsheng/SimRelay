const state = {
  device: null,
  messages: [],
  selectedIndex: null,
  activePanel: 'messages',
}

const els = {
  alert: document.querySelector('#alert'),
  refreshBtn: document.querySelector('#refreshBtn'),
  themeBtn: document.querySelector('#themeBtn'),
  boxSelect: document.querySelector('#boxSelect'),
  messageList: document.querySelector('#messageList'),
  messageDetail: document.querySelector('#messageDetail'),
  detailMeta: document.querySelector('#detailMeta'),
  sendForm: document.querySelector('#sendForm'),
  sendBtn: document.querySelector('#sendBtn'),
  toInput: document.querySelector('#toInput'),
  textInput: document.querySelector('#textInput'),
  textCounter: document.querySelector('#textCounter'),
  sideDot: document.querySelector('#sideDot'),
  sideStatus: document.querySelector('#sideStatus'),
  messagesPanel: document.querySelector('#messagesPanel'),
  devicePanel: document.querySelector('#devicePanel'),
}

function text(value) {
  return value === undefined || value === null || value === '' ? '--' : String(value)
}

function setAlert(message, kind = 'error') {
  if (!message) {
    els.alert.classList.add('hidden')
    els.alert.textContent = ''
    return
  }
  els.alert.textContent = message
  els.alert.classList.remove('hidden')
  els.alert.style.color = kind === 'success' ? 'var(--success)' : 'var(--danger)'
}

async function fetchJSON(url, options) {
  const res = await fetch(url, options)
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error(data.error || `请求失败：${res.status}`)
  }
  return data
}

async function refreshAll() {
  setAlert('')
  els.refreshBtn.disabled = true
  els.refreshBtn.textContent = '刷新中'
  try {
    const [device, messages] = await Promise.all([
      fetchJSON('/api/v1/device'),
      fetchJSON(`/api/v1/messages?box=${encodeURIComponent(els.boxSelect.value)}`),
    ])
    state.device = device
    state.messages = Array.isArray(messages) ? messages : []
    renderDevice()
    renderMessages()
    setOnline(true)
  } catch (err) {
    setOnline(false)
    setAlert(err.message)
  } finally {
    els.refreshBtn.disabled = false
    els.refreshBtn.textContent = '刷新'
  }
}

function setOnline(ok) {
  els.sideDot.classList.toggle('ok', ok)
  els.sideDot.classList.toggle('bad', !ok)
  els.sideStatus.textContent = ok ? '模组在线' : '模组异常'
}

function renderDevice() {
  const d = state.device || {}
  document.querySelector('#simValue').textContent = text(d.sim)
  document.querySelector('#networkValue').textContent = d.registered ? '已注册' : '未注册'
  document.querySelector('#signalValue').textContent = text(d.signal_rssi)
  document.querySelector('#messageCount').textContent = String(state.messages.length)
  document.querySelector('#manufacturerValue').textContent = text(d.manufacturer)
  document.querySelector('#modelValue').textContent = text(d.model)
  document.querySelector('#imeiValue').textContent = text(d.imei)
  document.querySelector('#berValue').textContent = text(d.signal_ber)
}

function renderMessages() {
  document.querySelector('#messageCount').textContent = String(state.messages.length)
  document.querySelector('#listHint').textContent = `${els.boxSelect.selectedOptions[0].textContent}短信`
  els.messageList.innerHTML = ''

  if (!state.messages.length) {
    const empty = document.createElement('div')
    empty.className = 'message-detail empty'
    empty.textContent = '没有短信'
    els.messageList.append(empty)
    renderDetail(null)
    return
  }

  const selectedExists = state.messages.some((msg) => msg.index === state.selectedIndex)
  if (!selectedExists) {
    state.selectedIndex = state.messages[0].index
  }

  for (const msg of state.messages) {
    const row = document.createElement('button')
    row.type = 'button'
    row.className = `message-row${msg.index === state.selectedIndex ? ' active' : ''}`
    row.innerHTML = `
      <div class="message-row-top">
        <span class="message-from"></span>
        <span class="message-status"></span>
      </div>
      <div class="message-preview"></div>
      <div class="message-time"></div>
    `
    row.querySelector('.message-from').textContent = msg.from || '未知号码'
    row.querySelector('.message-status').textContent = msg.status || 'UNKNOWN'
    row.querySelector('.message-preview').textContent = msg.text || '(空短信)'
    row.querySelector('.message-time').textContent = msg.timestamp || `索引 ${msg.index}`
    row.addEventListener('click', () => {
      state.selectedIndex = msg.index
      renderMessages()
    })
    els.messageList.append(row)
  }

  renderDetail(state.messages.find((msg) => msg.index === state.selectedIndex))
}

function renderDetail(msg) {
  if (!msg) {
    els.detailMeta.textContent = '选择一条短信查看内容'
    els.messageDetail.className = 'message-detail empty'
    els.messageDetail.textContent = '暂无选中短信'
    return
  }
  els.detailMeta.textContent = `${msg.from || '未知号码'} · ${msg.timestamp || `索引 ${msg.index}`}`
  els.messageDetail.className = 'message-detail'
  els.messageDetail.textContent = msg.text || '(空短信)'
}

async function sendMessage(event) {
  event.preventDefault()
  const to = els.toInput.value.trim()
  const body = els.textInput.value
  if (!to || !body) {
    setAlert('收件号码和短信内容不能为空')
    return
  }

  els.sendBtn.disabled = true
  els.sendBtn.textContent = '发送中'
  try {
    const result = await fetchJSON('/api/v1/messages', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ to, text: body }),
    })
    els.textInput.value = ''
    updateCounter()
    setAlert(`发送成功，引用号 ${result.reference}`, 'success')
    await refreshAll()
  } catch (err) {
    setAlert(err.message)
  } finally {
    els.sendBtn.disabled = false
    els.sendBtn.textContent = '发送'
  }
}

function updateCounter() {
  els.textCounter.textContent = `${els.textInput.value.length} / ${els.textInput.maxLength}`
}

function switchPanel(panel) {
  state.activePanel = panel
  document.querySelectorAll('.nav-item').forEach((item) => {
    item.classList.toggle('active', item.dataset.panel === panel)
  })
  els.messagesPanel.classList.toggle('hidden', panel !== 'messages')
  els.devicePanel.classList.toggle('hidden', panel !== 'device')
}

els.refreshBtn.addEventListener('click', refreshAll)
els.boxSelect.addEventListener('change', refreshAll)
els.sendForm.addEventListener('submit', sendMessage)
els.textInput.addEventListener('input', updateCounter)
els.themeBtn.addEventListener('click', () => {
  document.documentElement.classList.toggle('dark')
  localStorage.setItem('simrelay-theme', document.documentElement.classList.contains('dark') ? 'dark' : 'light')
})
document.querySelectorAll('.nav-item').forEach((item) => {
  item.addEventListener('click', () => switchPanel(item.dataset.panel))
})

updateCounter()
refreshAll()
