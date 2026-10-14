// Private, per-browser X server. No TCP listener or remote-viewing interface.
import {spawn} from 'node:child_process'
import {randomBytes} from 'node:crypto'
import {hostname} from 'node:os'
import {join} from 'node:path'
import {writeFile,rm} from 'node:fs/promises'
import {ProbeError} from './accounts.mjs'

function authorityRecord(number,cookie) {
 const field=value=>{const bytes=Buffer.from(value);const size=Buffer.alloc(2);size.writeUInt16BE(bytes.length);return Buffer.concat([size,bytes])}
 const family=Buffer.from([0,0]);family.writeUInt16BE(256) // FamilyLocal
 return Buffer.concat([family,field(hostname()),field(number),field('MIT-MAGIC-COOKIE-1'),field(cookie)])
}

export async function startVirtualDisplay(profile,{spawnProcess=spawn}={}) {
 const authority=join(profile,'.Xauthority')
 const cookie=randomBytes(16)
 await writeFile(authority,authorityRecord('',cookie),{mode:0o600,flag:'wx'})
 let child,exited=false
 let resolveExit
 const exit=new Promise(resolve=>{resolveExit=resolve})
 const stop=async()=>{
  if(child&&!exited){
   child.kill('SIGTERM')
   const force=setTimeout(()=>child.kill('SIGKILL'),1000)
   try{await exit}finally{clearTimeout(force)}
  }
  await rm(authority,{force:true})
 }
 try {
  child=spawnProcess('/usr/bin/Xvfb',['-displayfd','3','-screen','0','1280x1024x24','-nolisten','tcp','-auth',authority],{stdio:['ignore','ignore','ignore','pipe']})
  child.once('exit',()=>{exited=true;resolveExit()})
  child.once('error',()=>{exited=true;resolveExit()})
  let finishStartup
  let number
  try {
   number=await new Promise((resolve,reject)=>{
    let buffer=''
    const fail=()=>reject(new ProbeError('BROWSER_NOT_FOUND'))
    const timeout=setTimeout(fail,10000)
    const receive=chunk=>{
     buffer+=chunk.toString()
     if(buffer.length>16){fail();return}
     if(buffer.includes('\n')) {
      const value=buffer.trim()
      if(/^[0-9]{1,5}$/.test(value))resolve(value);else fail()
     }
    }
    child.stdio[3].on('data',receive)
    child.once('exit',fail);child.once('error',fail)
    finishStartup=()=>{clearTimeout(timeout);child.stdio[3].off('data',receive);child.off('exit',fail);child.off('error',fail)}
   })
  }finally{finishStartup?.()}
  if(exited)throw new ProbeError('BROWSER_NOT_FOUND')
  await writeFile(authority,authorityRecord(number,cookie),{mode:0o600})
  cookie.fill(0)
  return {display:':'+number,authority,stop,child}
 }catch(error){cookie.fill(0);await stop();throw error}
}
