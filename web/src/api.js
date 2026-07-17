const DEFAULT_API_BASE = '/api/v1'

function resolveBaseUrl(baseUrl) {
  const configured = String(baseUrl ?? '').trim()
  if (!configured) return DEFAULT_API_BASE
  return configured.replace(/\/+$/, '').endsWith('/api/v1')
    ? configured.replace(/\/+$/, '')
    : `${configured.replace(/\/+$/, '')}/api/v1`
}

function createIdempotencyKey() {
  const uuid = globalThis.crypto?.randomUUID?.()
  return `cf-${uuid ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`}`
}

function withQuery(path, query = {}) {
  const params = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== '') params.set(key, String(value))
  })
  const encoded = params.toString()
  return encoded ? `${path}?${encoded}` : path
}

export function createApiClient({ baseUrl, fetchImpl = globalThis.fetch } = {}) {
  if (typeof fetchImpl !== 'function') throw new Error('当前环境不支持 fetch')
  const base = resolveBaseUrl(baseUrl ?? import.meta.env?.VITE_API_BASE_URL)

  async function request(path, { method = 'GET', body, idempotencyKey, headers = {} } = {}) {
    const isWrite = method !== 'GET' && method !== 'HEAD'
    const requestHeaders = { Accept: 'application/json', ...headers }
    if (body !== undefined) requestHeaders['Content-Type'] = 'application/json'
    if (isWrite) requestHeaders['Idempotency-Key'] = idempotencyKey || createIdempotencyKey()
    const response = await fetchImpl(`${base}${path}`, {
      method,
      headers: requestHeaders,
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    let envelope
    try {
      envelope = await response.json()
    } catch {
      throw new Error(`API 返回了无效响应（${response.status}）`)
    }
    if (!response.ok || envelope?.code !== 0) {
      throw new Error(envelope?.message || `API 请求失败（${response.status}）`)
    }
    return envelope.data
  }

  return {
    baseUrl: base,
    getDashboard: () => request('/dashboard'),
    listDepartments: () => request('/departments'),
    listDoctors: () => request('/doctors'),
    listPatients: (query) => request(withQuery('/patients', query)),
    listAppointments: (query) => request(withQuery('/appointments', query)),
    getAppointment: (id) => request(`/appointments/${encodeURIComponent(id)}`),
    createAppointment: (input, idempotencyKey) => request('/appointments', { method: 'POST', body: input, idempotencyKey }),
    checkinAppointment: (id, idempotencyKey) => request(`/appointments/${encodeURIComponent(id)}/checkin`, { method: 'POST', idempotencyKey }),
    updateAppointmentStatus: (id, status, actor, idempotencyKey) => request(`/appointments/${encodeURIComponent(id)}/status`, {
      method: 'POST',
      body: { status, ...(actor ? { actor } : {}) },
      idempotencyKey,
    }),
    listFollowups: (query) => request(withQuery('/followups', query)),
    createFollowup: (input, idempotencyKey) => request('/followups', { method: 'POST', body: input, idempotencyKey }),
    completeFollowup: (id, idempotencyKey) => request(`/followups/${encodeURIComponent(id)}/complete`, { method: 'POST', idempotencyKey }),
  }
}

