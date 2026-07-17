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
      return response({ id: 'LG-1', status: '已立案' })
    },
  })

  const appointment = await client.checkinAppointment('LG-1')

  assert.equal(appointment.id, 'LG-1')
  assert.equal(requests[0].url, '/api/v1/appointments/LG-1/checkin')
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

  await assert.rejects(() => client.updateAppointmentStatus('LG-1', '待办理'), /状态不可推进/)
})

test('exposes mobile lifecycle and follow-up operations through the same client', async () => {
  const paths = []
  const client = createApiClient({
    fetchImpl: async (url) => {
      paths.push(url)
      return response({ id: 'ok' })
    },
  })

  await client.createAppointment({ patient: '案卷 C001', department: '合同纠纷' })
  await client.checkinAppointment('LG-1')
  await client.updateAppointmentStatus('LG-1', '待办理')
  await client.updateAppointmentStatus('LG-1', '办理中')
  await client.updateAppointmentStatus('LG-1', '已结案')
  await client.completeFollowup('FW-1')

  assert.deepEqual(paths, [
    '/api/v1/appointments',
    '/api/v1/appointments/LG-1/checkin',
    '/api/v1/appointments/LG-1/status',
    '/api/v1/appointments/LG-1/status',
    '/api/v1/appointments/LG-1/status',
    '/api/v1/followups/FW-1/complete',
  ])
})

test('exposes matter collaboration queue, assignment, documents and closure APIs', async () => {
  const requests = []
  const client = createApiClient({ fetchImpl: async (url, init) => { requests.push({ url, init }); return response({ id: 'LF-1', status: '协同中' }) } })
  await client.listMatters({ status: '协同中', assignee: '林律师' })
  await client.getMatter('LF-1')
  await client.listMatterEvents('LF-1')
  await client.createMatter({ subjectAlias: '演示案卷-1', caseType: '合同审查', priority: '高', deadline: '2026-07-30' })
  await client.assignMatter('LF-1', { assignee: '林律师', actor: '许汝林' })
  await client.updateMatterStatus('LF-1', '待结案', '林律师')
  await client.addMatterFile('LF-1', { name: '证据.pdf', kind: 'evidence', checksum: 'sha256:1' })
  await client.closeMatter('LF-1', { result: '材料已归档', actor: '林律师' })
  assert.deepEqual(requests.map(({ url }) => url), [
    '/api/v1/matters?status=%E5%8D%8F%E5%90%8C%E4%B8%AD&assignee=%E6%9E%97%E5%BE%8B%E5%B8%88',
    '/api/v1/matters/LF-1',
    '/api/v1/matters/LF-1/events',
    '/api/v1/matters',
    '/api/v1/matters/LF-1/assign',
    '/api/v1/matters/LF-1/status',
    '/api/v1/matters/LF-1/file',
    '/api/v1/matters/LF-1/close',
  ])
})
