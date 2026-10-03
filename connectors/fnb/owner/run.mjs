import {mkdtemp,mkdir,writeFile,rm,access} from 'node:fs/promises'
import {homedir,tmpdir} from 'node:os'
import {join,resolve,dirname,relative} from 'node:path'
import {fileURLToPath} from 'node:url'
import {createInterface} from 'node:readline/promises'
import {extractAccounts,normalizeAccounts,ProbeError,errorMessages,diagnosticLine} from './accounts.mjs'

const sourceRoot=resolve(dirname(fileURLToPath(import.meta.url)),'../../..')
const args=process.argv.slice(2)
function argument(name){const i=args.indexOf(name);return i<0?undefined:args[i+1]}
function outsideWorkspace(path){const rel=relative(sourceRoot,resolve(path));return rel.startsWith('..')||rel.startsWith('/')||/^[a-z]:/i.test(rel)}
async function browserExecutable(){
 const configured=argument('--browser')
 const choices=configured?[configured]:process.platform==='win32'?[
  join(process.env.ProgramFiles||'C:/Program Files','Google/Chrome/Application/chrome.exe'),
  join(process.env['ProgramFiles(x86)']||'C:/Program Files (x86)','Microsoft/Edge/Application/msedge.exe'),
  join(process.env.LOCALAPPDATA||join(homedir(),'AppData/Local'),'Google/Chrome/Application/chrome.exe')
 ]:['/usr/bin/google-chrome','/usr/bin/chromium','/usr/bin/chromium-browser']
 for(const path of choices){try{await access(path);return path}catch{}}
 throw new ProbeError('BROWSER_NOT_FOUND')
}
const output=resolve(argument('--output')||join(homedir(),'Downloads','fnb-accounts-'+Date.now()+'.json'))
let browser,context,profile,terminal
const abort=new AbortController()
const cancel=()=>abort.abort()
process.on('SIGINT',cancel);process.on('SIGTERM',cancel)
try{
 if(!outsideWorkspace(output))throw new ProbeError('OUTPUT_IN_WORKSPACE')
 let report,skipped=[]
 if(args.includes('--mock')){
  report=normalizeAccounts([{name:'Synthetic cheque account',bank_id:'12345678901'},{name:'Synthetic savings account',bank_id:'22222222222'}])
 }else{
  if(!process.stdin.isTTY)throw new ProbeError('CANCELLED')
  let puppeteer
  try{puppeteer=(await import('puppeteer-core')).default}catch{throw new ProbeError('DEPENDENCY_MISSING')}
  const executablePath=await browserExecutable()
  profile=await mkdtemp(join(tmpdir(),'finance-fnb-owner-'))
  // Deliberately do not pass credential environment variables to Chrome.
  const env={}
  for(const key of ['PATH','Path','SYSTEMROOT','SystemRoot','WINDIR','TEMP','TMP','HOME','USERPROFILE','LOCALAPPDATA','APPDATA','DISPLAY','WAYLAND_DISPLAY','XDG_RUNTIME_DIR'])if(process.env[key])env[key]=process.env[key]
  browser=await puppeteer.launch({executablePath,headless:false,pipe:true,userDataDir:profile,env,timeout:30000})
  context=await browser.createBrowserContext();const page=await context.newPage()
  await page.goto('https://www.fnb.co.za/',{waitUntil:'domcontentloaded',timeout:60000})
  process.stdout.write('Sign in yourself in the temporary browser. Complete approvals and open Accounts.\nCredentials are entered on FNB only; this script does not read them.\n')
  terminal=createInterface({input:process.stdin,output:process.stdout})
  await terminal.question('Press Enter here once the account summary is visible (Ctrl+C to cancel): ',{signal:abort.signal})
  const pages=(await context.pages()).reverse()
  let failure=new ProbeError('ACCOUNT_LIST_NOT_FOUND')
  for(const candidate of pages){try{report=await extractAccounts(candidate,{skipUnsupported:args.includes('--skip-unsupported'),onSkipped:positions=>{skipped=positions}});break}catch(err){if(err instanceof ProbeError&&err.code!=='BANK_PAGE_REQUIRED')failure=err}}
  if(!report)throw failure
 }
 await mkdir(dirname(output),{recursive:true,mode:0o700})
 try{await writeFile(output,JSON.stringify(report,null,2)+'\n',{flag:'wx',mode:0o600})}catch(err){if(err.code==='EEXIST')throw new ProbeError('OUTPUT_EXISTS');throw err}
 if(skipped.length)process.stdout.write('PARTIAL_DISCOVERY: Skipped '+skipped.length+' unsupported account-number field(s), at summary position(s): '+skipped.join(', ')+'. Add these accounts manually in the tracker if needed.\n')
 process.stdout.write('Discovered '+report.accounts.length+' accounts. No transactions or balances were read.\nSaved account metadata to '+output+'\nImport this file in the tracker under Settings → Accounts → Import discovered accounts.\n')
}catch(err){const code=abort.signal.aborted?'CANCELLED':err instanceof ProbeError?err.code:'TEST_FAILED';process.stderr.write(code+': '+(errorMessages[code]||errorMessages.TEST_FAILED)+'\n');if(args.includes('--diagnose')){const line=diagnosticLine(err.diagnostics);if(line)process.stderr.write(line+'\n')}process.exitCode=1}
finally{
 process.removeListener('SIGINT',cancel);process.removeListener('SIGTERM',cancel)
 terminal?.close()
 try{await context?.close()}catch{}
 try{await browser?.close()}catch{}
 if(profile){try{await rm(profile,{recursive:true,force:true})}catch{process.stderr.write('PROFILE_CLEANUP_REQUIRED: Close the temporary browser and remove its temporary profile.\n')}}
}
