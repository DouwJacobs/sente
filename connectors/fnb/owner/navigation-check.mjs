// Owner-run public connectivity probe. No database, credentials or page content.
import {mkdtemp,rm,statfs} from 'node:fs/promises'
import {tmpdir} from 'node:os'
import {join} from 'node:path'
import puppeteer from 'puppeteer-core'
import {launchBrowser} from './browser-runtime.mjs'
import {observeBrowserStderr} from './browser-crash-diagnostics.mjs'
import {loginFailureDiagnostics,observeNavigation,navigationOriginCounts,bankValidationOrigin,waitForBankValidation} from './login-diagnostics.mjs'

let browser,context,profile,stopBrowserDiagnostics
const browserCounts={}
let result={navigation_probe_failed:1}
let publicHost
try {
 profile=await mkdtemp(join(tmpdir(),'finance-fnb-navigation-check-'))
 browser=await launchBrowser(puppeteer,{profile,visible:process.argv.includes('--visible')})
 stopBrowserDiagnostics=observeBrowserStderr(browser,browserCounts)
 context=await browser.createBrowserContext()
 const page=await context.newPage()
 const counts={login_step:1}
 const stopObserving=observeNavigation(page,counts)
 try {
  const response=await page.goto('https://www.fnb.co.za/',{waitUntil:'domcontentloaded',timeout:45000})
  if(bankValidationOrigin(page.url())){
   counts.bank_validation_waited=1
   counts.bank_validation_cleared=Number(await waitForBankValidation(page)&&navigationOriginCounts(page.url()).bank_origin===1)
  }
  // Explicit owner-only public probe option; never expose a URL, path, query,
  // userinfo or page content. The live worker/API never exports hostnames.
  if(process.argv.includes('--show-public-host')) {
   const finalURL=new URL(page.url()),responseURL=new URL(response?.url()||page.url())
   publicHost={final_hostname:finalURL.hostname,response_hostname:responseURL.hostname}
  }
  result={...counts,navigation_ok:1,http_status:response?.status()||0,
   ...navigationOriginCounts(page.url()),
   response_bank_origin:navigationOriginCounts(response?.url()||'').bank_origin,
   navigation_redirects:Math.min(response?.request().redirectChain().length||0,100)}
 } catch(error) {result={...counts,...loginFailureDiagnostics(error,1)}}finally{stopObserving()}
} catch {result={browser_start_failed:1}}
finally {
 for(const [path,key] of [['/tmp','probe_tmp_free_mb'],['/dev/shm','probe_shm_free_mb']]) {
  try{const info=await statfs(path);browserCounts[key]=Math.floor(info.bavail*info.bsize/1048576)}catch{}
 }
 stopBrowserDiagnostics?.()
 await context?.close().catch(()=>{})
 await browser?.close().catch(()=>{})
 if(profile)await rm(profile,{recursive:true,force:true}).catch(()=>{result={profile_cleanup_failed:1}})
}
console.log(JSON.stringify({...result,...browserCounts}))
if(publicHost)console.log(JSON.stringify(publicHost))
