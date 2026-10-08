import test from 'node:test'
import assert from 'node:assert/strict'
import {PassThrough} from 'node:stream'
import {readWorkerInput} from './refresh.mjs'

test('worker receives credentials once and cancellation separately',async()=>{
 const stream=new PassThrough(),abort=new AbortController()
 const input=readWorkerInput(stream,abort)
 stream.write(JSON.stringify({username:'synthetic',password:'synthetic'})+'\n')
 const result=await input
 assert.equal(result.credentials.username,'synthetic')
 assert.equal(abort.signal.aborted,false)
 stream.write('{"cancel":true}\n')
 assert.equal(abort.signal.aborted,true)
 result.control.close();stream.destroy()
})
test('oversized first input rejects without credentials',async()=>{
 const stream=new PassThrough(),abort=new AbortController()
 const input=readWorkerInput(stream,abort)
 stream.end('x'.repeat(16385)+'\n')
 await assert.rejects(input)
 assert.equal(abort.signal.aborted,true)
})

test('loss of the parent control pipe aborts an active worker',async()=>{
 const stream=new PassThrough(),abort=new AbortController()
 const input=readWorkerInput(stream,abort)
 stream.write('{"username":"synthetic","password":"synthetic"}\n')
 await input
 stream.end()
 await new Promise(resolve=>setImmediate(resolve))
 assert.equal(abort.signal.aborted,true)
})
