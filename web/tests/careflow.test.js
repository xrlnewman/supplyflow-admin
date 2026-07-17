import test from 'node:test'; import assert from 'node:assert/strict'; import { readFile } from 'node:fs/promises'
test('SupplyFlow has purchase-order, buyer schedule and supplier follow-up data', async()=>{const source=await readFile(new URL('../src/main.js',import.meta.url),'utf8'); assert.match(source,/今日采购单队列/); assert.match(source,/采购专员排班/); assert.match(source,/采购跟进/); assert.match(source,/PO-0716-082/)})

test('SupplyFlow binds real API actions while keeping a demo fallback', async()=>{const source=await readFile(new URL('../src/main.js',import.meta.url),'utf8'); assert.match(source,/createApiClient/); assert.match(source,/data-action="checkin"/); assert.match(source,/data-action="status"/); assert.match(source,/data-action="complete-followup"/); assert.match(source,/refreshFromApi/); assert.match(source,/演示数据/)})

test('Vite proxies the default API path to the local Go service', async()=>{const source=await readFile(new URL('../vite.config.js',import.meta.url),'utf8'); assert.match(source,/server/); assert.match(source,/proxy/); assert.match(source,/localhost:8080/)})
