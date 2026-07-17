import test from 'node:test'
import assert from 'node:assert/strict'

import { createApiClient } from '../src/api.js'

function response(data, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    async json() {
      return { code: 0, message: 'ok', data }
    },
  }
}

test('defaults to /api/v1 and adds an idempotency key to writes', async () => {
  const requests = []
  const client = createApiClient({
    fetchImpl: async (url, init) => {
      requests.push({ url, init })
      return response({ id: 'AP-1', status: '已确认' })
    },
  })

  const appointment = await client.checkinAppointment('AP-1')

  assert.equal(appointment.id, 'AP-1')
  assert.equal(requests[0].url, '/api/v1/appointments/AP-1/checkin')
  assert.equal(requests[0].init.method, 'POST')
  assert.match(requests[0].init.headers['Idempotency-Key'], /^cf-/)
})

test('uses a configured API origin without duplicating the API path', async () => {
  const requests = []
  const client = createApiClient({
    baseUrl: 'http://localhost:8080/api/v1/',
    fetchImpl: async (url) => {
      requests.push(url)
      return response({ list: [], total: 0 })
    },
  })

  await client.listAppointments({ page: 1, pageSize: 20 })

  assert.equal(requests[0], 'http://localhost:8080/api/v1/appointments?page=1&pageSize=20')
})

test('rejects non-zero API envelopes so callers can keep demo data', async () => {
  const client = createApiClient({
    fetchImpl: async () => ({
      ok: false,
      status: 409,
      async json() {
        return { code: 409, message: '状态不可推进', data: null }
      },
    }),
  })

  await assert.rejects(() => client.updateAppointmentStatus('AP-1', '候诊中'), /状态不可推进/)
})

test('采购闭环客户端覆盖列表、比价、审批、收货质检与核销', async () => {
  const calls = []
  const client = createApiClient({ fetchImpl: async (url, init = {}) => { calls.push({ url, init }); return response({ id: 'PR-1', status: '询价中' }) } })
  await client.listPurchaseRequests({ keyword: '滤芯', status: '询价中', page: 1 })
  await client.getPurchaseRequest('PR-1')
  await client.addSupplierQuote('PR-1', { supplier: '星链', amount: '99.90' })
  await client.approvePurchaseRequest('PR-1')
  await client.orderPurchaseRequest('PR-1')
  await client.receivePurchaseRequest('PR-1', { quantity: 2 })
  await client.reconcilePurchaseRequest('PR-1', { passed: true })
  assert.deepEqual(calls.map(({ url }) => url), ['/api/v1/purchase-requests?keyword=%E6%BB%A4%E8%8A%AF&status=%E8%AF%A2%E4%BB%B7%E4%B8%AD&page=1', '/api/v1/purchase-requests/PR-1', '/api/v1/purchase-requests/PR-1/quotes', '/api/v1/purchase-requests/PR-1/approve', '/api/v1/purchase-requests/PR-1/order', '/api/v1/purchase-requests/PR-1/receipt', '/api/v1/purchase-requests/PR-1/reconcile'])
})

test('exposes mobile lifecycle and follow-up operations through the same client', async () => {
  const paths = []
  const client = createApiClient({
    fetchImpl: async (url) => {
      paths.push(url)
      return response({ id: 'ok' })
    },
  })

  await client.createAppointment({ patient: '演示客户', department: '全科门诊' })
  await client.checkinAppointment('AP-1')
  await client.updateAppointmentStatus('AP-1', '候诊中')
  await client.updateAppointmentStatus('AP-1', '处理中')
  await client.updateAppointmentStatus('AP-1', '已完成')
  await client.completeFollowup('FW-1')

  assert.deepEqual(paths, [
    '/api/v1/appointments',
    '/api/v1/appointments/AP-1/checkin',
    '/api/v1/appointments/AP-1/status',
    '/api/v1/appointments/AP-1/status',
    '/api/v1/appointments/AP-1/status',
    '/api/v1/followups/FW-1/complete',
  ])
})
