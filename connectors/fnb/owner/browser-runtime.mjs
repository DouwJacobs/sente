// Shared by the live worker and the offline production-image startup check.
import {access} from 'node:fs/promises'
import {homedir} from 'node:os'
import {join} from 'node:path'
import {ProbeError} from './accounts.mjs'

async function executable() {
 const candidates=process.platform==='win32'?[
  join(process.env.ProgramFiles||'C:/Program Files','Google/Chrome/Application/chrome.exe'),
  join(process.env['ProgramFiles(x86)']||'C:/Program Files (x86)','Microsoft/Edge/Application/msedge.exe'),
  join(process.env.LOCALAPPDATA||join(homedir(),'AppData/Local'),'Google/Chrome/Application/chrome.exe')
 ]:['/usr/bin/google-chrome','/usr/bin/chromium','/usr/bin/chromium-browser']
 for(const file of candidates){try{await access(file);return file}catch{}}
 throw new ProbeError('BROWSER_NOT_FOUND')
}

export async function launchBrowser(puppeteer,{profile,visible=false}) {
 const env={}
 for(const key of ['PATH','Path','SYSTEMROOT','SystemRoot','WINDIR','TEMP','TMP','HOME','USERPROFILE','LOCALAPPDATA','APPDATA','DISPLAY','WAYLAND_DISPLAY','XDG_RUNTIME_DIR']) {
  if(process.env[key])env[key]=process.env[key]
 }
 // Keep browser caches and crash metadata inside the private ephemeral profile.
 if(process.platform==='linux') {
  env.HOME=profile
  env.XDG_CONFIG_HOME=join(profile,'config')
  env.XDG_CACHE_HOME=join(profile,'cache')
 }
 return puppeteer.launch({
  executablePath:await executable(),headless:!visible,pipe:true,userDataDir:profile,
  env,timeout:30000,ignoreDefaultArgs:['--disable-dev-shm-usage'],
  args:['--disable-breakpad','--disable-crash-reporter']
 })
}
