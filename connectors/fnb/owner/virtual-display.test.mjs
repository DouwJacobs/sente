import test from 'node:test'
import assert from 'node:assert/strict'
import {EventEmitter} from 'node:events'
import {PassThrough} from 'node:stream'
import {mkdtemp,readFile,stat,readdir,rm} from 'node:fs/promises'
import {tmpdir} from 'node:os'
import {join} from 'node:path'
import {startVirtualDisplay} from './virtual-display.mjs'

function fakeServer(report,capture) {
 return (path,args,options)=>{
  capture.path=path;capture.args=args;capture.options=options
  const child=new EventEmitter()
  child.stdio=[null,null,null,new PassThrough()]
  child.kill=signal=>{capture.signal=signal;queueMicrotask(()=>child.emit('exit',0));return true}
  queueMicrotask(()=>report(child))
  return child
 }
}

test('private display has owner-only authorization, disables TCP and cleans up',async()=>{
 const profile=await mkdtemp(join(tmpdir(),'synthetic-display-'))
 const capture={}
 try {
  const display=await startVirtualDisplay(profile,{spawnProcess:fakeServer(child=>child.stdio[3].write('123\n'),capture)})
  assert.equal(display.display,':123')
  assert.equal(capture.path,'/usr/bin/Xvfb')
  assert(capture.args.includes('-auth'))
  assert.deepEqual(capture.args.slice(capture.args.indexOf('-nolisten'),capture.args.indexOf('-nolisten')+2),['-nolisten','tcp'])
  assert.equal((await stat(display.authority)).mode&0o777,0o600)
  const record=await readFile(display.authority)
  assert(record.includes(Buffer.from('123')))
  assert(record.includes(Buffer.from('MIT-MAGIC-COOKIE-1')))
  await display.stop()
  assert.equal(capture.signal,'SIGTERM')
  assert.deepEqual(await readdir(profile),[])
 }finally{await rm(profile,{recursive:true,force:true})}
})

test('failed or malformed display startup removes its authorization file',async()=>{
 for(const report of [child=>child.emit('error',Error('synthetic failure')),child=>child.stdio[3].write('invalid\n')]) {
  const profile=await mkdtemp(join(tmpdir(),'synthetic-display-'))
  try {
   await assert.rejects(startVirtualDisplay(profile,{spawnProcess:fakeServer(report,{})}))
   assert.deepEqual(await readdir(profile),[])
  }finally{await rm(profile,{recursive:true,force:true})}
 }
})
