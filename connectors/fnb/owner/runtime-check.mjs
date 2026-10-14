// Offline only: no banking, credentials, external pages or debug listener.
import assert from 'node:assert/strict'
import {createServer} from 'node:http'
import {mkdtemp,rm,readdir,stat} from 'node:fs/promises'
import {tmpdir} from 'node:os'
import {join} from 'node:path'
import {spawnSync} from 'node:child_process'
import puppeteer from 'puppeteer-core'
import {launchBrowser} from './browser-runtime.mjs'
import {loginAndAccounts} from './refresh.mjs'

const profile=await mkdtemp(join(tmpdir(),'finance-fnb-runtime-check-'))
let browser,context,server
let sandboxViolation=false,stderrTail=''
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

 browser=await launchBrowser(puppeteer,{profile,visible:process.argv.includes('--visible')})
 browser.process()?.stderr?.on('data',chunk=>{
  stderrTail=(stderrTail+chunk.toString()).slice(-4096)
  if(stderrTail.includes('seccomp-bpf failure'))sandboxViolation=true
 })
 const args=browser.process().spawnargs
 assert(!args.some(arg=>/^--(?:no-sandbox|disable-setuid-sandbox|disable-seccomp-filter-sandbox|remote-debugging-port)(?:=|$)/.test(arg)))
 assert(args.includes('--remote-debugging-pipe'))
 if(process.env.FNB_BROWSER_MODE==='virtual-display') {
  assert(!args.some(arg=>arg.startsWith('--headless')),'Packaged browser must use the private virtual display')
  assert.equal((await stat(join(profile,'.Xauthority'))).mode&0o777,0o600)
 }

 context=await browser.createBrowserContext()
 const page=await context.newPage()
 await page.setContent('<title>Offline FNB runtime check</title><p id="result">Synthetic only</p>')
 assert.equal(await page.$eval('#result',node=>node.textContent),'Synthetic only')
 // Intercept every request before network access. The bank-shaped origin only
 // exercises origin/form protections; all HTML and account values are invented.
 await page.setRequestInterception(true)
 page.on('request',request=>{
  if(request.url()==='https://www.fnb.co.za/'&&request.isNavigationRequest()) {
   void request.respond({status:200,contentType:'text/html',body:`
    <form method="post" action="https://www.fnb.co.za/login">
     <input id="user"><input id="pass" type="password"><button type="submit">Sign in</button>
    </form>
    <script>document.querySelector('form').addEventListener('submit',event=>{
     event.preventDefault();document.body.innerHTML='<div name="nickname">Synthetic account</div><div name="accountNumber">00123456</div><div name="ledgerBalance">R 12.34</div>';
    });</script>`})
  } else void request.abort()
 })
 const snapshot=await loginAndAccounts(page,{username:'synthetic-user',password:'synthetic-password'})
 assert.equal(snapshot.accounts.length,1)
 assert.equal(snapshot.accounts[0].balance_decimal,'12.34')
 await page.setRequestInterception(false)
 page.removeAllListeners('request')
 console.log('PASS offline FNB login: intercepted synthetic form, typing, submission and account extraction')
 // Request interception misses browser networking and cache-write failures.
 // Use actual loopback HTTP with external networking still disabled by Docker.
 server=createServer((request,response)=>{
  response.writeHead(200,{'Content-Type':'text/html','Cache-Control':'public, max-age=3600'})
  response.end('<title>Offline loopback network</title><p id="network">Synthetic network only</p>')
 })
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 await page.goto('http://127.0.0.1:'+server.address().port,{waitUntil:'domcontentloaded',timeout:15000})
 assert.equal(await page.$eval('#network',node=>node.textContent),'Synthetic network only')
 await new Promise(resolve=>setTimeout(resolve,500))
 assert(!sandboxViolation,'Chromium child sandbox crashed during synthetic network activity')
 console.log('PASS offline Chromium loopback networking: no child sandbox crash')
 await page.goto('chrome://sandbox')
 const status=await page.evaluate(()=>document.body.innerText)
 for(const name of ['PID namespaces','Network namespaces','Seccomp-BPF sandbox']) {
  assert(new RegExp(name+'\\s+Yes').test(status),'Chromium sandbox check failed: '+name)
 }
 console.log('PASS offline FNB browser: non-root, namespace/seccomp sandbox, private pipe, synthetic page')
} finally {
 await context?.close().catch(()=>{})
 await browser?.close().catch(()=>{})
 if(process.env.FNB_BROWSER_MODE==='virtual-display')assert(!(await readdir(profile)).includes('.Xauthority'),'Display authorization must be removed on browser close')
 if(server){server.closeAllConnections();await new Promise(resolve=>server.close(resolve))}
 await rm(profile,{recursive:true,force:true})
 assert(!(await readdir(tmpdir())).includes(profile.split('/').at(-1)))
 assert(!sandboxViolation,'Chromium child sandbox crashed')
}
