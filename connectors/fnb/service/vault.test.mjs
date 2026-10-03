import {test} from 'node:test'
import assert from 'node:assert/strict'
import {randomBytes} from 'node:crypto'
import {sealCredentials,unsealCredentials,rotateCredentials} from './vault.mjs'
const credentials={username:'synthetic-user',password:'synthetic-bank-password'}
function options(){const key=randomBytes(32);return {ownerId:'synthetic-owner',connectionId:'synthetic-connection',keyVersion:'v1',key,keys:new Map([['v1',key]])}}
test('authenticated encryption hides both credentials and uses fresh nonces',()=>{
 const o=options(),a=sealCredentials(credentials,o),b=sealCredentials(credentials,o)
 assert.notEqual(a.nonce,b.nonce);assert.notEqual(a.ciphertext,b.ciphertext)
 assert.deepEqual(unsealCredentials(a,o),credentials)
 const stored=JSON.stringify(a);assert.ok(!stored.includes(credentials.username));assert.ok(!stored.includes(credentials.password))
})
test('identity binding, tampering, missing and wrong keys fail without secrets',()=>{
 const o=options(),record=sealCredentials(credentials,o)
 const variants=[{...o,ownerId:'other'},{...o,connectionId:'other'},{...o,keys:new Map()},{...o,keys:new Map([['v1',randomBytes(32)]])}]
 for(const v of variants)assert.throws(()=>unsealCredentials(record,v),{message:'Credentials unavailable; owner action required'})
 for(const field of ['nonce','tag','ciphertext','keyVersion'])assert.throws(()=>unsealCredentials({...record,[field]:'tampered'},o),{message:'Credentials unavailable; owner action required'})
})
test('rotation preserves identity and requires the retained old key',()=>{
 const o=options(),record=sealCredentials(credentials,o),nextKey=randomBytes(32)
 const next=rotateCredentials(record,o,{keyVersion:'v2',key:nextKey})
 assert.equal(next.keyVersion,'v2');assert.deepEqual(unsealCredentials(next,{...o,keys:new Map([['v2',nextKey]])}),credentials)
 assert.throws(()=>unsealCredentials(record,{...o,keys:new Map([['v2',nextKey]])}))
 assert.throws(()=>sealCredentials(credentials,{...o,key:Buffer.alloc(16)}))
})
