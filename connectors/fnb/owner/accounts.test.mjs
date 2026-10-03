import {test} from 'node:test'
import assert from 'node:assert/strict'
import {extractAccounts,normalizeAccounts,readAccountDOM,diagnosticLine} from './accounts.mjs'
test('exports account metadata only, including masked numbers for owner correction',()=>{
 const report=normalizeAccounts([{name:' Synthetic account ',bank_id:'123 456',password:'synthetic-secret',balance:999,transactions:[1]}])
 assert.deepEqual(report,{schema_version:1,source:'fnb-account-discovery',accounts:[{name:'Synthetic account',bank_id:'123456'}]})
 assert.equal(normalizeAccounts([{name:'Masked',bank_id:'******1234'}]).accounts[0].bank_id,'******1234')
})
test('rejects empty, ambiguous and oversized lists with fixed errors',()=>{
 for(const value of [null,[],[{name:'',bank_id:'123'}],[{name:'Account',bank_id:'<bad>'}],[{name:'Account',bank_id:'123'},{name:'Other',bank_id:'123'}],Array(101).fill({name:'Account',bank_id:'123'})])assert.throws(()=>normalizeAccounts(value))
})
test('reads only the account metadata contract from an approved bank page',async()=>{
 let evaluated=false
 const report=await extractAccounts({url:()=> 'https://www.fnb.co.za/accounts',evaluate:async()=>{evaluated=true;return {raw:[{name:'Synthetic',bank_id:'123456'}],diagnostics:{}}}})
 assert.ok(evaluated);assert.equal(report.accounts.length,1)
 evaluated=false
 await assert.rejects(()=>extractAccounts({url:()=> 'https://fnb.co.za.example.com/',evaluate:async()=>{evaluated=true}}))
 assert.equal(evaluated,false)
})

import vm from 'node:vm'
function domRead(names,numbers){
 const node=({text='',hidden=false,input=false})=>({textContent:text,matches:()=>input,getClientRects:()=>hidden?[]:[{}]})
 const document={querySelectorAll:selector=>(selector.includes('nickname')?names:numbers).map(node)}
 return vm.runInNewContext('('+readAccountDOM.toString()+')()', {document,TextEncoder,getComputedStyle:()=>({visibility:'visible'})})
}
test('ignores hidden duplicates and credential inputs without reading their values',()=>{
 // Build separately to ensure even accessing input textContent fails.
 const visible={textContent:'Synthetic',matches:()=>false,getClientRects:()=>[{}]}
 const input={get textContent(){throw Error('must not read input text')},matches:()=>true,getClientRects:()=>[{}]}
 const number={...visible,textContent:'123456'}
 const document={querySelectorAll:s=>s.includes('nickname')?[visible,input]:[number,input]}
 const result=vm.runInNewContext('('+readAccountDOM.toString()+')()', {document,TextEncoder,getComputedStyle:()=>({visibility:'visible'})})
 assert.equal(normalizeAccounts(result.raw).accounts.length,1)
 const hidden=domRead([{text:'Synthetic'},{text:'Hidden',hidden:true}],[{text:'123456'},{text:'123456',hidden:true}])
 assert.equal(normalizeAccounts(hidden.raw).accounts.length,1)
})
test('layout diagnostics reveal only fixed counts, with no account text or secrets',()=>{
 const result=domRead([{text:'Synthetic private name'}],[{text:'Account number: 123456'}])
 assert.equal(result.diagnostics.invalid_numbers,1)
 const line=diagnosticLine({...result.diagnostics,password:'synthetic-secret',raw:result.raw,name_nodes:'malicious text'})
 assert.ok(line.includes('"invalid_numbers":1'))
 for(const text of ['123456','private','secret','malicious'])assert.ok(!line.includes(text))
 assert.equal(diagnosticLine({raw:'synthetic-secret'}),'')
 const mismatch=domRead([{text:'Synthetic'}],[])
 assert.equal(mismatch.raw,null)
 assert.equal(mismatch.diagnostics.visible_numbers,0)
})

test('skipping unsupported numbers requires explicit opt-in and reports only positions',()=>{
 const rows=[{name:'First synthetic',bank_id:'001234'},{name:'Unsupported synthetic',bank_id:'not-a-bank-number'},{name:'Third synthetic',bank_id:'***999'}]
 assert.throws(()=>normalizeAccounts(rows),{code:'ACCOUNT_LAYOUT_CHANGED'})
 let skipped
 const report=normalizeAccounts(rows,{skipUnsupported:true,onSkipped:positions=>{skipped=positions}})
 assert.deepEqual(skipped,[2])
 assert.deepEqual(report.accounts,[{name:'First synthetic',bank_id:'001234'},{name:'Third synthetic',bank_id:'***999'}])
 assert.deepEqual(Object.keys(report),['schema_version','source','accounts'])
})
test('partial discovery still rejects ambiguous lists, invalid names and zero valid accounts',()=>{
 for(const rows of [null,[],[{name:'',bank_id:'bad'}],[{name:'Synthetic',bank_id:'bad'}],[{name:'First',bank_id:'123'},{name:'Second',bank_id:'123'},{name:'Other',bank_id:'bad'}]]){
  let called=false
  assert.throws(()=>normalizeAccounts(rows,{skipUnsupported:true,onSkipped:()=>{called=true}}))
  assert.equal(called,false)
 }
})
