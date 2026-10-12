// Offline only: no banking, credentials, external pages or debug listener.
import assert from 'node:assert/strict'
import {mkdtemp,rm,readdir} from 'node:fs/promises'
import {tmpdir} from 'node:os'
import {join} from 'node:path'
import {spawnSync} from 'node:child_process'
import puppeteer from 'puppeteer-core'
import {launchBrowser} from './browser-runtime.mjs'

const profile=await mkdtemp(join(tmpdir(),'finance-fnb-runtime-check-'))
let browser,context
try {
 assert.notEqual(process.getuid(),0)
 // Exercise the backend's configured worker path with invalid, credential-free
 // input. Validation must stop before any browser or bank navigation.
 const worker=spawnSync(process.env.FNB_NODE_EXECUTABLE,[process.env.FNB_RUNNER_PATH],{
  input:'{}\n',encoding:'utf8',timeout:10000,maxBuffer:16384
 })
 assert.equal(worker.status,0,'Configured worker could not start')
 const report=JSON.parse(worker.stdout)
 assert.equal(report.diagnostics.refresh_phase,1)
 assert.deepEqual(report.accounts,[])

 browser=await launchBrowser(puppeteer,{profile})
 const args=browser.process().spawnargs
 assert(!args.some(arg=>/^--(?:no-sandbox|disable-setuid-sandbox|disable-seccomp-filter-sandbox|remote-debugging-port)(?:=|$)/.test(arg)))
 assert(args.includes('--remote-debugging-pipe'))
 context=await browser.createBrowserContext()
 const page=await context.newPage()
 await page.setContent('<title>Offline FNB runtime check</title><p id="result">Synthetic only</p>')
 assert.equal(await page.$eval('#result',node=>node.textContent),'Synthetic only')
 await page.goto('chrome://sandbox')
 const status=await page.evaluate(()=>document.body.innerText)
 for(const name of ['PID namespaces','Network namespaces','Seccomp-BPF sandbox']) {
  assert(new RegExp(name+'\\s+Yes').test(status),'Chromium sandbox check failed: '+name)
 }
 console.log('PASS offline FNB browser: non-root, namespace/seccomp sandbox, private pipe, synthetic page')
} finally {
 await context?.close().catch(()=>{})
 await browser?.close().catch(()=>{})
 await rm(profile,{recursive:true,force:true})
 assert(!(await readdir(tmpdir())).includes(profile.split('/').at(-1)))
}
