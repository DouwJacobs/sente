import {test} from 'node:test'
import assert from 'node:assert/strict'
import {randomBytes} from 'node:crypto'
import {createMockServer} from './mock-server.mjs'
test('service fails closed outside mock mode',()=>{
 assert.throws(()=>createMockServer({mode:'live',token:'x'.repeat(32)}))
 assert.throws(()=>createMockServer({mode:'mock',token:''}))
})
test('read-only authenticated report contract; credential routes absent',async()=>{
 const token=randomBytes(32).toString('hex'),server=createMockServer({mode:'mock',token})
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve))
 try{
  const url='http://127.0.0.1:'+server.address().port,headers={Authorization:'Bearer '+token}
  assert.equal((await fetch(url+'/v1/report')).status,401)
  const status=await(await fetch(url+'/v1/status',{headers})).json();assert.equal(status.live_enabled,false);assert.equal(status.credential_input_enabled,false)
  const report=await(await fetch(url+'/v1/report',{headers})).json();assert.equal(report.transactions[0].amount_decimal,'-0.29')
  assert.equal((await fetch(url+'/v1/credentials',{headers})).status,404)
  assert.equal((await fetch(url+'/v1/credentials',{method:'POST',headers,body:'synthetic-only'})).status,405)
 }finally{await new Promise(resolve=>server.close(resolve))}
})
