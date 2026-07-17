import test from 'node:test'; import assert from 'node:assert/strict'; import { readFile } from 'node:fs/promises'
test('LegalFlow has case queue, schedule and compliance data', async()=>{const source=await readFile(new URL('../src/main.js',import.meta.url),'utf8'); assert.match(source,/今日案件队列/); assert.match(source,/律师排班/); assert.match(source,/合规任务/); assert.match(source,/LG-0716-082/)})

test('LegalFlow binds real API actions while keeping a demo fallback', async()=>{const source=await readFile(new URL('../src/main.js',import.meta.url),'utf8'); assert.match(source,/createApiClient/); assert.match(source,/data-action="checkin"/); assert.match(source,/data-action="status"/); assert.match(source,/data-action="complete-followup"/); assert.match(source,/refreshFromApi/); assert.match(source,/演示数据/)})

test('Vite proxies the default API path to the local Go service', async()=>{const source=await readFile(new URL('../vite.config.js',import.meta.url),'utf8'); assert.match(source,/server/); assert.match(source,/proxy/); assert.match(source,/localhost:8080/)})

test('LegalFlow exposes a matter collaboration workspace with deadline, document and closure controls', async()=>{const source=await readFile(new URL('../src/main.js',import.meta.url),'utf8'); assert.match(source,/案件协同/); assert.match(source,/截止日期/); assert.match(source,/归档文档/); assert.match(source,/提交结案/); assert.match(source,/listMatters/); assert.match(source,/listMatterEvents/)})
