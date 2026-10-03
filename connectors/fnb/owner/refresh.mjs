// GPL-3.0; selectors adapted from ../upstream at the pinned commit.
// Automatic account/balance and opt-in transaction-preview worker. Never run live from agent tools.
import {mkdtemp,rm,access} from 'node:fs/promises'
import {tmpdir,homedir} from 'node:os'
import {join} from 'node:path'
import {normalizeAccounts,ProbeError} from './accounts.mjs'
import {fetchTransactions} from './transactions.mjs'
export function exactBalance(text){
 if(typeof text!=='string')throw new ProbeError('BALANCE_LAYOUT_CHANGED')
 // Ledger fields provide ZAR units; signs and omitted decimal zeros are exact.
 let value=text.trim().replace(/−/g,'-').replace(/^R\s*/,'').replace(/\s/g,'')
 if(!/^[+-]?(?:\d+|\d{1,3}(?:,\d{3})+)(?:\.\d{1,2})?$/.test(value))throw new ProbeError('BALANCE_LAYOUT_CHANGED')
 value=value.replace(/,/g,'');const negative=value.startsWith('-')
 const [whole,fraction='']=value.replace(/^[+-]/,'').split('.')
 const decimals=fraction.padEnd(2,'0'),amount=BigInt(whole)*100n+BigInt(decimals)
 if(amount>9007199254740991n)throw new ProbeError('BALANCE_LAYOUT_CHANGED')
 return (negative&&amount!==0n?'-':'')+BigInt(whole).toString()+'.'+decimals
}

export function unsupportedBalanceUnit(text){
 const value=text.trim()
 if(/^eB(?:ucks)?(?=\s|[+−-]?\d)/i.test(value))return 'reward_entries'
 if(/^(?:(?:CHF|EUR|USD|GBP|AUD|CAD|JPY|CNY|NZD|HKD|SGD|AED|ZMW|BWP|MUR)\b|[€£$¥])/i.test(value))return 'non_zar_entries'
 return null
}
export function normalizeSnapshot(raw,hidden=[],diagnostics={}){
 if(!Array.isArray(raw)||raw.length>100)throw new ProbeError('ACCOUNT_LAYOUT_CHANGED')
 const excluded=new Set(hidden),accounts=[];let skipped=0
 for(const row of raw){
  if(typeof row.name!=='string'||!row.name.trim()||Buffer.byteLength(row.name.trim())>100)throw new ProbeError('ACCOUNT_LAYOUT_CHANGED')
  if(typeof row.bank_id!=='string'||!/^[0-9]{3,64}$/.test(row.bank_id.replace(/\s/g,''))){skipped++;continue}
  const metadata=normalizeAccounts([{name:row.name,bank_id:row.bank_id}]).accounts[0]
  if(!/^[0-9]{3,64}$/.test(metadata.bank_id)){skipped++;continue}
  let balance=null
  if(!excluded.has(metadata.bank_id)&&row.balance_text!=null){
   const text=row.balance_text.trim()
   const unsupported=unsupportedBalanceUnit(text)
   if(unsupported){skipped++;diagnostics[unsupported]=(diagnostics[unsupported]||0)+1;continue}
   if(!text)diagnostics.blank_balances=(diagnostics.blank_balances||0)+1
   else if(/^(?:[-–—]+|N\/A|Not available)$/i.test(text))diagnostics.placeholder_balances=(diagnostics.placeholder_balances||0)+1
   else {try{balance=exactBalance(text)}catch(err){
    diagnostics.invalid_amounts=(diagnostics.invalid_amounts||0)+1
    const shape=/[CD]R$/i.test(text)?'sign_suffix':/^\(.*\)$/.test(text)?'parenthesized':/,\d{2}$/.test(text)&&!text.includes('.')?'comma_decimal':'other_format'
    diagnostics[shape]=(diagnostics[shape]||0)+1
    for(const [key,match] of [['trailing_minus',/-$/.test(text)],['currency_suffix',/R$/i.test(text)],['unknown_text',/[a-z]/i.test(text.replace(/R|ZAR|CR|DR/gi,''))],['repeated_decimal',(text.match(/\./g)||[]).length>1],['balance_label',/\bbalance\b/i.test(text)],['unavailable_text',/\b(?:unavailable|not\s+(?:applicable|available)|unable|restricted)\b/i.test(text)],['loading_text',/\b(?:loading|please\s+wait)\b/i.test(text)]])if(match)diagnostics[key]=(diagnostics[key]||0)+1
    err.diagnostics=diagnostics;throw err
   }}
  }
  accounts.push({...metadata,balance_decimal:balance})
 }
 if(!accounts.length)throw new ProbeError('ACCOUNT_LIST_NOT_FOUND')
 normalizeAccounts(accounts) // duplicate identities remain an error
 return {accounts,skipped,...(Object.keys(diagnostics).length?{diagnostics}: {})}
}
export function readBalanceDOM(hidden){
 const visible=node=>!node.matches('input,textarea,select,[contenteditable]')&&node.getClientRects().length>0&&getComputedStyle(node).visibility!=='hidden'
 const names=[...document.querySelectorAll('[name="nickname"]')].filter(visible)
 const numbers=[...document.querySelectorAll('[name="accountNumber"]')].filter(visible)
 const balances=[...document.querySelectorAll('[name="ledgerBalance"]')].filter(visible)
 const diagnostics={name_nodes:names.length,number_nodes:numbers.length,ledger_nodes:balances.length,matched_rows:0,missing_rows:0,unsupported_entries:0,hidden_rows:0}
 if(!names.length||names.length!==numbers.length)return {rows:null,diagnostics}
 const rows=names.map((node,i)=>{
  const bank_id=(numbers[i].textContent||'').replace(/\s/g,'')
  if(hidden.includes(bank_id)){diagnostics.hidden_rows++;return {name:node.textContent||'',bank_id,balance_text:null}}
  if(!/^[0-9]{3,64}$/.test(bank_id)){diagnostics.unsupported_entries++;return {name:node.textContent||'',bank_id,balance_text:null}}
  let parent=node.parentElement,balanceNode
  for(let depth=0;parent&&depth<8;depth++,parent=parent.parentElement){
   const rowNames=[...parent.querySelectorAll('[name="nickname"]')].filter(visible)
   if(rowNames.length>1)break
   const rowNumbers=[...parent.querySelectorAll('[name="accountNumber"]')].filter(visible)
   const rowBalances=[...parent.querySelectorAll('[name="ledgerBalance"]')].filter(visible)
   if(rowNames.length===1&&rowNumbers.length===1&&rowNumbers[0]===numbers[i]&&rowBalances.length===1){balanceNode=rowBalances[0];break}
  }
  if(!balanceNode&&balances.length===names.length)balanceNode=balances[i]
  if(!balanceNode){diagnostics.missing_rows++;return {name:node.textContent||'',bank_id,balance_text:null}}
  diagnostics.matched_rows++
  return {name:node.textContent||'',bank_id,balance_text:balanceNode.innerText??balanceNode.textContent??''}
 })
 return {rows:diagnostics.missing_rows?null:rows,diagnostics}
}

export function balanceFieldCountsDOM(){
 const count=selector=>document.querySelectorAll(selector).length
 return {name_nodes:count('[name="nickname"]'),number_nodes:count('[name="accountNumber"]'),ledger_nodes:count('[name="ledgerBalance"]')}
}
export function previousSessionDOM(){
 // Inspect locally and return only a boolean; never export bank page text.
 const text=document.body?.innerText||''
 return /session\s+will\s+be\s+terminated|(?:will\s+be\s+)?logged\s+out\s+shortly/i.test(text)
}
export function clickLogoutDOM(){
 const visible=node=>node.getClientRects().length>0&&getComputedStyle(node).visibility!=='hidden'
 const controls=[...document.querySelectorAll('a,button,input[type="button"],input[type="submit"]')]
 const control=controls.find(node=>{
  if(!visible(node)||!/^log\s*(?:out|off)$|^sign\s*out$/i.test((node.textContent||node.value||'').trim()))return false
  if(node.tagName==='A'&&node.href&&!node.href.startsWith('javascript:')){
   const url=new URL(node.href,location.href)
   if(url.protocol!=='https:'||!(url.hostname==='fnb.co.za'||url.hostname.endsWith('.fnb.co.za')))return false
  }
  return true
 })
 if(!control)return false
 control.click();return true
}
export function signedOutDOM(){
 const visible=node=>!!node&&node.getClientRects().length>0&&getComputedStyle(node).visibility!=='hidden'
 // A public header can retain hidden login inputs after redirect.
 const authenticated=[...document.querySelectorAll('[name="nickname"],#newsLanding')].some(visible)
 const logout=[...document.querySelectorAll('a,button,input[type="button"],input[type="submit"]')].some(node=>visible(node)&&/^log\s*(?:out|off)$|^sign\s*out$/i.test((node.textContent||node.value||'').trim()))
 if(authenticated||logout)return 0
 const user=document.querySelector('#user'),pass=document.querySelector('#pass')
 if(visible(user)&&visible(pass))return 1
 // Only explicit completed logout confirms success; impending logout does not.
 const text=document.body?.innerText||''
 if(/\b(?:you(?:['’]ve|\s+(?:have|are))?\s+(?:(?:now|successfully|been)\s+)*|successfully\s+)(?:logged|signed)\s+(?:off|out)\b|\b(?:logged|signed)\s+(?:off|out)\s+successfully\b|\b(?:log\s*(?:out|off)|sign\s*out)\s+(?:was\s+)?successful\b/i.test(text))return 2
 return 0
}
async function bankSurfaces(context){
 const surfaces=[]
 for(const page of await context.pages()){
  if(!bankPage(page))continue
  for(const frame of page.frames?.()||[page])if(bankPage(frame))surfaces.push(frame)
 }
 return surfaces
}
export async function logoutBankSession(context,diagnostics={},options={}){
 let attempted=false
 for(const surface of await bankSurfaces(context)){try{if(await surface.evaluate(clickLogoutDOM)){attempted=true;diagnostics.logout_clicked=1;break}}catch{}}
 if(!attempted)return false
 // Bounded wait covers redirect and explicit confirmation, including bank frames.
 for(let attempt=0;attempt<(options.attempts??30);attempt++){
  for(const surface of await bankSurfaces(context)){
   try{const confirmed=await surface.evaluate(signedOutDOM);if(confirmed){diagnostics.logout_confirmed=1;return true}}catch{}
  }
  await new Promise(resolve=>setTimeout(resolve,options.delayMs??500))
 }
 diagnostics.logout_unconfirmed=1
 throw new ProbeError('LOGOUT_REQUIRED')
}
export async function finishBankSession(context,result){
 const diagnostics={...(result?.diagnostics||{})}
 try{
  const signedOut=await logoutBankSession(context,diagnostics)
  if(!signedOut){
   let authenticated=!!result?.accounts?.length
   for(const surface of await bankSurfaces(context)){try{authenticated=authenticated||await surface.evaluate(()=>{
    const visible=node=>node.getClientRects().length>0&&getComputedStyle(node).visibility!=='hidden'
    return [...document.querySelectorAll('[name="nickname"],#newsLanding')].some(visible)
   })}catch{}}
   if(authenticated)throw new ProbeError('LOGOUT_REQUIRED')
  }
  return {...result,diagnostics}
 }catch{
  diagnostics.logout_unconfirmed=1
  if(result?.error==='BALANCE_LAYOUT_CHANGED')diagnostics.balance_failure=1
  if(result?.error?.startsWith('TRANSACTION_'))diagnostics.transaction_failure=1
  return {accounts:[],skipped:0,error:'LOGOUT_REQUIRED',diagnostics}
 }
}
function bankPage(page){try{const u=new URL(page.url());return u.protocol==='https:'&&(u.hostname==='fnb.co.za'||u.hostname.endsWith('.fnb.co.za'))}catch{return false}}
export async function loginAndAccounts(page,credentials,signal){
 await page.goto('https://www.fnb.co.za/',{waitUntil:'domcontentloaded',timeout:45000})
 if(!bankPage(page))throw new ProbeError('LOGIN_LAYOUT_CHANGED')
 try{await page.waitForSelector('#user',{visible:true,timeout:10000});await page.waitForSelector('#pass',{visible:true,timeout:10000})}catch{throw new ProbeError('LOGIN_LAYOUT_CHANGED')}
 // Never send credentials to an unapproved origin or unrelated form.
 if(!bankPage(page))throw new ProbeError('LOGIN_LAYOUT_CHANGED')
 const valid=await page.evaluate(()=>{
  const user=document.querySelector('#user'),pass=document.querySelector('#pass')
  if(!user||!pass||pass.type!=='password'||!user.form||user.form!==pass.form)return false
  const target=new URL(user.form.action||location.href,location.href)
  return target.protocol==='https:'&&(target.hostname==='fnb.co.za'||target.hostname.endsWith('.fnb.co.za'))
 })
 if(!valid)throw new ProbeError('LOGIN_LAYOUT_CHANGED')
 await page.type('#user',credentials.username);await page.type('#pass',credentials.password)
 const submitted=await page.evaluate(()=>{
  const form=document.querySelector('#user').form
  const button=form.querySelector('input[type="submit"],button[type="submit"]')
  if(!button)return false;button.click();return true
 })
 if(!submitted)throw new ProbeError('LOGIN_LAYOUT_CHANGED')
 // One login attempt only. Owner can complete approval in visible manual runs.
 const deadline=Date.now()+110000
 while(Date.now()<deadline){
  if(signal?.aborted)throw new ProbeError('REFRESH_FAILED')
  const pages=await page.browserContext().pages()
  for(const candidate of pages){
   if(!bankPage(candidate))continue
   try{
    if(await candidate.evaluate(previousSessionDOM))throw new ProbeError('SESSION_CONFLICT')
    const hasAccounts=await candidate.evaluate(()=>!!document.querySelector('[name="nickname"]'))
    if(hasAccounts){
     // Names can render before asynchronously populated balance fields.
     let result
     for(let attempt=0;attempt<10;attempt++){
      if(signal?.aborted)throw new ProbeError('REFRESH_FAILED')
      if(await candidate.evaluate(previousSessionDOM))throw new ProbeError('SESSION_CONFLICT')
      try{result=await candidate.evaluate(readBalanceDOM,credentials.hidden||[])}catch{let diagnostics={evaluation_failed:1};try{diagnostics={...(await candidate.evaluate(balanceFieldCountsDOM)),evaluation_failed:1}}catch{}throw new ProbeError('BALANCE_LAYOUT_CHANGED',diagnostics)}
      if(result.rows)break
      await new Promise(resolve=>setTimeout(resolve,500))
     }
     if(!result.rows)throw new ProbeError('BALANCE_LAYOUT_CHANGED',result.diagnostics)
     return normalizeSnapshot(result.rows,credentials.hidden||[],result.diagnostics)
    }
    await candidate.evaluate(()=>{
     const link=[...document.querySelectorAll('.shortCutLink')].find(node=>/\bAccounts\b/.test(node.textContent||''))
     if(link&&link.getClientRects().length)link.click()
    })
   }catch(err){if(err instanceof ProbeError)throw err}
  }
  await new Promise(resolve=>setTimeout(resolve,1000))
 }
 throw new ProbeError('APPROVAL_REQUIRED')
}
async function executable(){
 const candidates=process.platform==='win32'?[
 join(process.env.ProgramFiles||'C:/Program Files','Google/Chrome/Application/chrome.exe'),
 join(process.env['ProgramFiles(x86)']||'C:/Program Files (x86)','Microsoft/Edge/Application/msedge.exe'),
 join(process.env.LOCALAPPDATA||join(homedir(),'AppData/Local'),'Google/Chrome/Application/chrome.exe')]:['/usr/bin/google-chrome','/usr/bin/chromium','/usr/bin/chromium-browser']
 for(const file of candidates){try{await access(file);return file}catch{}}
 throw new ProbeError('BROWSER_NOT_FOUND')
}
export async function main(){
 let browser,context,profile;let result
 const abort=new AbortController()
 const cancel=()=>abort.abort()
 process.on('SIGINT',cancel);process.on('SIGTERM',cancel)
 const timeout=setTimeout(cancel,205000)
 try{
  let input='';for await(const chunk of process.stdin){input+=chunk;if(input.length>16384)throw new ProbeError('REFRESH_FAILED')}
  const credentials=JSON.parse(input);input=''
  if(typeof credentials.username!=='string'||typeof credentials.password!=='string'||!credentials.username||!credentials.password||!Array.isArray(credentials.hidden||[]))throw new ProbeError('REFRESH_FAILED')
  const {default:puppeteer}=await import('puppeteer-core')
  profile=await mkdtemp(join(tmpdir(),'finance-fnb-refresh-'))
  const env={};for(const key of ['PATH','Path','SYSTEMROOT','SystemRoot','WINDIR','TEMP','TMP','HOME','USERPROFILE','LOCALAPPDATA','APPDATA','DISPLAY','WAYLAND_DISPLAY','XDG_RUNTIME_DIR'])if(process.env[key])env[key]=process.env[key]
  browser=await puppeteer.launch({executablePath:await executable(),headless:!credentials.visible,pipe:true,userDataDir:profile,env,timeout:30000})
  context=await browser.createBrowserContext();const page=await context.newPage();if(credentials.visible)await page.bringToFront()
  result=await loginAndAccounts(page,credentials,abort.signal)
  if(credentials.transaction_accounts?.length){
   if(credentials.transaction_accounts.some(id=>!result.accounts.some(account=>account.bank_id===id)))throw new ProbeError('TRANSACTION_ACCOUNT_MISMATCH')
   result.reports=await fetchTransactions(context,credentials.transaction_accounts,credentials.run_id,abort.signal)
   result.diagnostics={...result.diagnostics,transaction_accounts:result.reports.length,transaction_accounts_requested:credentials.transaction_accounts.length,transaction_fee_rows:result.reports.reduce((sum,r)=>sum+r.transactions.filter(t=>t.service_fee_decimal&&t.service_fee_decimal!=='0.00').length,0),transaction_rows:result.reports.reduce((sum,r)=>sum+r.transactions.length,0)}
  }
  credentials.username='';credentials.password=''
 }catch(err){
  const allowed=['SESSION_CONFLICT','LOGIN_LAYOUT_CHANGED','APPROVAL_REQUIRED','ACCOUNT_LAYOUT_CHANGED','BALANCE_LAYOUT_CHANGED','BROWSER_NOT_FOUND','TRANSACTION_LAYOUT_CHANGED','TRANSACTION_ACCOUNT_MISMATCH','TRANSACTION_FEE_REVIEW_REQUIRED','TRANSACTION_ACCOUNT_UNSUPPORTED']
  result={accounts:[],skipped:0,error:err instanceof ProbeError&&allowed.includes(err.code)?err.code:'REFRESH_FAILED',...(err instanceof ProbeError&&err.diagnostics?{diagnostics:err.diagnostics}:{})}
 }finally{
  if(context)result=await finishBankSession(context,result)
  clearTimeout(timeout);process.removeListener('SIGINT',cancel);process.removeListener('SIGTERM',cancel)
  try{await context?.close()}catch{};try{await browser?.close()}catch{}
  if(profile){try{await rm(profile,{recursive:true,force:true})}catch{result={accounts:[],skipped:0,error:'REFRESH_FAILED'}}}
 }
 process.stdout.write(JSON.stringify(result))
}
// Import-safe for synthetic unit tests; no browser is launched on import.
if(process.argv[1]?.replace(/\\/g,'/').endsWith('/refresh.mjs'))await main()
