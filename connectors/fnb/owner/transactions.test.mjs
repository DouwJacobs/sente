import {test} from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {normalizeTransactionTable,transactionDate,transactionMoney,readTransactionDOM,clickTransactionAccountDOM,clickTransactionTabDOM,clickAccountsDOM,fetchTransactions,resolveMaskedCreditAccounts,readCreditIdentityDOM} from './transactions.mjs'
const table=()=>({bank_id:'00123456',account_type:'Cheque',headers:['Date','Description','Reference','Service Fee','Amount','Balance'],posted_history:true,rows:[['03 Oct 2026','Synthetic purchase','ref-one','0','R -0.29','R 1,234.56'],['2026-10-02','Synthetic refund','','0.00','R +12.34','R 1,234.85']]})
test('exact signed money, strict dates, references and identical purchases survive',()=>{
 const input=table();input.rows.push(input.rows[0])
 const out=normalizeTransactionTable(input,'00123456','synthetic-run')
 assert.equal(out.transactions.length,3);assert.equal(out.transactions[0].amount_decimal,'-0.29');assert.equal(out.transactions[1].amount_decimal,'12.34');assert.equal(out.transactions[0].date,'2026-10-03');assert.equal(out.transactions[0].source_reference,'ref-one');assert.equal(out.transactions[0].balance_decimal,'1234.56')
 assert.equal(transactionMoney('R −0.01'),'-0.01');assert.equal(transactionDate('1 Jan 2026'),'2026-01-01')
})
test('credit status rejects uncertainty and skips explicit pending authorizations',()=>{
 const input={...table(),account_type:'Credit',headers:['Date','Description','Amount','Status'],posted_history:false,rows:[['03 Oct 2026','Synthetic pending','-1.00','Pending'],['03 Oct 2026','Synthetic posted','-2.00','Successful']]}
 assert.equal(normalizeTransactionTable(input,'00123456','run').transactions.length,1)
 input.rows[1][3]='';assert.throws(()=>normalizeTransactionTable(input,'00123456','run'),/TRANSACTION_LAYOUT_CHANGED/)
})
test('account mismatch, unknown headers, missing posted evidence, fees, FX and invalid amounts fail closed with counts',()=>{
 const mutations=[input=>input.bank_id='998877',input=>input.account_type='Vehicle',input=>input.headers[4]='Available amount',input=>input.posted_history=false,input=>input.rows[0][3]='-0.01',input=>input.rows[0][4]='USD -0.29',input=>input.rows[0][4]='-0.291',input=>input.rows[0][0]='30 Feb 2026',input=>input.rows.push(['malformed']),input=>input.currency='USD']
 for(const mutate of mutations){const input=table();mutate(input);assert.throws(()=>normalizeTransactionTable(input,'00123456','run'),err=>err.code.startsWith('TRANSACTION_')&&!err.message.includes('Synthetic')&&Object.values(err.diagnostics).every(Number.isInteger))}
 for(const value of ['1e2','1.001','12,34','eB 10','1 CR','(1.00)','90071992547409.92'])assert.throws(()=>transactionMoney(value))
})
test('empty history needs explicit evidence and 150 rows are retained without pagination claims',()=>{
 const input=table();input.rows=[];assert.throws(()=>normalizeTransactionTable(input,'00123456','run'))
 input.empty_confirmed=true;assert.equal(normalizeTransactionTable(input,'00123456','run').transactions.length,0)
 input.rows=Array.from({length:150},()=>table().rows[0]);assert.equal(normalizeTransactionTable(input,'00123456','run').transactions.length,150)
 input.rows.push(table().rows[0]);assert.throws(()=>normalizeTransactionTable(input,'00123456','run'))
})

test('navigation matches full identity rather than nickname and rejects foreign links',()=>{
 let clicked=0
 const node=(innerText,visible=true)=>({innerText,getClientRects:()=>visible?[{}]:[]})
 const links=[{...node('Same name'),href:'https://www.fnb.co.za/account',click:()=>{clicked=1}},{...node('Same name'),href:'https://www.fnb.co.za/account',click:()=>{clicked=2}}]
 const names=links.map(link=>({...node('Same name'),querySelector:()=>link}))
 const document={querySelectorAll:selector=>selector.includes('nickname')?names:[node('00123456'),node('99887766')]}
 const context={document,getComputedStyle:()=>({visibility:'visible'}),URL,location:{href:'https://www.fnb.co.za/'}}
 assert.equal(vm.runInNewContext('('+clickTransactionAccountDOM.toString()+')("99887766")',context),true);assert.equal(clicked,2)
 links[1].href='https://example.com/account';clicked=0
 assert.equal(vm.runInNewContext('('+clickTransactionAccountDOM.toString()+')("99887766")',context),false);assert.equal(clicked,0)
})
test('DOM extraction requires unique identity fields and ignores hidden rows',()=>{
 const node=(innerText,visible=true)=>({innerText,getClientRects:()=>visible?[{}]:[],querySelector:()=>null})
 const field=(label,value)=>({...node(label),nextElementSibling:node(value)})
 const fields=[field('Account number','00123456'),field('Type','Fusion'),field('Currency','ZAR')]
 const headers=table().headers.map(label=>node(label))
 const row={...node(''),querySelectorAll:()=>table().rows[0].map(value=>node(value))}
 const forbidden={...node('',false),querySelectorAll:()=>{throw Error('hidden bank row read')}}
 const document={querySelector:()=>null,querySelectorAll:selector=>selector==='.dlTitle'?fields:selector==='.tableRow'?[row,forbidden]:selector.startsWith('.tableHeader')?headers:selector.startsWith('h1')?[node('Transaction history')]:[]}
 const context={document,getComputedStyle:()=>({visibility:'visible'})}
 const read=()=>vm.runInNewContext('('+readTransactionDOM.toString()+')()',context)
 const extracted=read();assert.equal(extracted.bank_id,'00123456');assert.equal(extracted.account_type,'Cheque');assert.equal(extracted.rows.length,1);assert.equal(extracted.posted_history,true)
 assert.equal(normalizeTransactionTable(extracted,'00123456','run').transactions[0].amount_decimal,'-0.29')
 fields[1].nextElementSibling.innerText='FNB Home Loan';assert.equal(read().account_type,'Home Loan')
 for(const label of ['FNB Money Maximizer','Money Maximiser']){
  fields[1].nextElementSibling.innerText=label
  const savings=read();assert.equal(savings.account_type,'Savings')
  assert.equal(normalizeTransactionTable(savings,'00123456','run').account_type,'Savings')
 }
 for(const label of ['Vehicle Loan','Global Account','eBucks Account']){fields[1].nextElementSibling.innerText=label;assert.equal(read().account_type,'')}
 fields.push(field('Account number','99887766'));assert.equal(read().bank_id,'')
})

test('synthetic navigation fetches exactly the requested identities and validates each detail page',async()=>{
 const requested=[];let current=''
 const page={url:()=> 'https://www.fnb.co.za/',evaluate:async(fn,arg)=>{
  if(fn===clickTransactionAccountDOM){requested.push(arg);current=arg;return true}
  if(fn===clickTransactionTabDOM||fn===clickAccountsDOM)return true
  if(fn===readTransactionDOM)return {...table(),bank_id:current}
  if(fn.toString().includes('session'))return false
  return true
 }}
 const result=await fetchTransactions({pages:async()=>[page]},['00123456','99887766'],'synthetic-run',new AbortController().signal)
 assert.deepEqual(requested,['00123456','99887766']);assert.equal(result.length,2);assert.equal(result[1].bank_id,'99887766')
 assert.equal(result[0].page_rows,2)
 page.evaluate=async()=>true
 const abort=new AbortController();abort.abort()
 await assert.rejects(()=>fetchTransactions({pages:async()=>[page]},['00123456'],'run',abort.signal),{code:'REFRESH_FAILED'})
 await assert.rejects(()=>fetchTransactions({pages:async()=>[{url:()=> 'https://example.com/'}]},['00123456'],'run'),{code:'TRANSACTION_LAYOUT_CHANGED'})
})

test('nonzero service fees preserve principal and fee decimals without stopping other accounts',async()=>{
 const requested=[];let current=''
 const page={url:()=> 'https://www.fnb.co.za/',evaluate:async(fn,arg)=>{
  if(fn===clickTransactionAccountDOM){requested.push(arg);current=arg;return true}
  if(fn===readTransactionDOM){const input=table();input.bank_id=current;input.rows[0][3]='1.23';return input}
  if(fn.toString().includes('session'))return false
  return true
 }}
 const reports=await fetchTransactions({pages:async()=>[page]},['00123456','99887766'],'run')
 assert.deepEqual(requested,['00123456','99887766']);assert.equal(reports.length,2)
 assert.equal(reports[0].transactions[0].amount_decimal,'-0.29');assert.equal(reports[0].transactions[0].service_fee_decimal,'1.23')
})


test('unsupported third account reports only position and type/currency reason, never partial success',async()=>{
 for(const reason of ['type','currency']){
  let current=''
  const ids=['00123456','99887766','11223344','55667788']
  const requested=[]
  const page={url:()=> 'https://www.fnb.co.za/',evaluate:async(fn,arg)=>{
   if(fn===clickTransactionAccountDOM){requested.push(arg);current=arg;return true}
   if(fn===readTransactionDOM){const input={...table(),bank_id:current};if(current===ids[2]){if(reason==='type')input.account_type='';else input.currency='USD'}return input}
   if(fn.toString().includes('session'))return false
   return true
  }}
  await assert.rejects(()=>fetchTransactions({pages:async()=>[page]},ids,'run'),err=>{
   assert.equal(err.code,'TRANSACTION_ACCOUNT_UNSUPPORTED')
   assert.equal(err.diagnostics.transaction_accounts,2)
   assert.equal(err.diagnostics.transaction_accounts_requested,4)
   assert.equal(err.diagnostics.transaction_failed_account_position,3)
   assert.equal(err.diagnostics['transaction_unsupported_'+reason],1)
   assert.ok(Object.values(err.diagnostics).every(Number.isInteger))
   assert.ok(!JSON.stringify(err).includes(ids[2]))
   return true
  })
  assert.deepEqual(requested,ids.slice(0,3))
 }
})


test('home loan in third position preserves signed entries and permits all seven account reports',async()=>{
 const ids=['00123456','99887766','11223344','55667788','22113344','88776655','12344321']
 let current='';const requested=[]
 const page={url:()=> 'https://www.fnb.co.za/',evaluate:async(fn,arg)=>{
  if(fn===clickTransactionAccountDOM){requested.push(arg);current=arg;return true}
  if(fn===readTransactionDOM)return {...table(),bank_id:current,account_type:current===ids[2]?'Home Loan':'Cheque',currency:'ZAR'}
  if(fn.toString().includes('session'))return false
  return true
 }}
 const reports=await fetchTransactions({pages:async()=>[page]},ids,'run')
 assert.deepEqual(requested,ids);assert.equal(reports.length,7)
 assert.equal(reports[2].account_type,'Home Loan')
 assert.equal(reports[2].transactions[0].amount_decimal,'-0.29')
 assert.equal(reports[2].transactions[1].amount_decimal,'12.34')
})


test('Money Maximizer mapped to Savings in third position does not stop subsequent accounts',async()=>{
 const ids=['00123456','99887766','11223344','55667788','22113344','88776655','12344321']
 const requested=[];let current=''
 const node=innerText=>({innerText,getClientRects:()=>[{}],querySelector:()=>null})
 const field=(label,value)=>({...node(label),nextElementSibling:node(value)})
 const fields=[field('Account number',ids[2]),field('Type','FNB Money Maximizer'),field('Currency','ZAR')]
 const row={...node(''),querySelectorAll:()=>table().rows[0].map(node)}
 const document={querySelector:()=>null,querySelectorAll:selector=>selector==='.dlTitle'?fields:selector==='.tableRow'?[row]:selector.startsWith('.tableHeader')?table().headers.map(node):selector.startsWith('h1')?[node('Transaction history')]:[]}
 const savings=vm.runInNewContext('('+readTransactionDOM.toString()+')()',{document,getComputedStyle:()=>({visibility:'visible'})})
 const page={url:()=> 'https://www.fnb.co.za/',evaluate:async(fn,arg)=>{
  if(fn===clickTransactionAccountDOM){requested.push(arg);current=arg;return true}
  if(fn===readTransactionDOM)return current===ids[2]?savings:{...table(),bank_id:current}
  if(fn.toString().includes('session'))return false
  return true
 }}
 const reports=await fetchTransactions({pages:async()=>[page]},ids,'run')
 assert.deepEqual(requested,ids);assert.equal(reports.length,7)
 assert.equal(reports[2].account_type,'Savings')
 assert.equal(reports[2].transactions[0].amount_decimal,'-0.29')
})


test('loan Effective Date ledger preserves zero adjustments, repayments and standalone charge rows',()=>{
 const input={bank_id:'00123456',account_type:'Home Loan',currency:'ZAR',posted_history:true,headers:['Effective Date','Description','Amount','Balance'],rows:[['03 Oct 2026','Synthetic adjustment','0.00','-1000.00'],['02 Oct 2026','Synthetic fee','-5.00','-1000.00'],['01 Oct 2026','Synthetic interest','-10.00','-995.00'],['30 Sep 2026','Synthetic repayment','50.00','-985.00']]}
 const report=normalizeTransactionTable(input,'00123456','run')
 assert.equal(report.page_rows,4)
 assert.deepEqual(report.transactions.map(r=>r.amount_decimal),['0.00','-5.00','-10.00','50.00'])
 assert.ok(report.transactions.every(r=>r.service_fee_decimal==='0.00'&&!r.source_reference))
 input.posted_history=false
 assert.throws(()=>normalizeTransactionTable(input,'00123456','run'),{code:'TRANSACTION_LAYOUT_CHANGED'})
})


test('masked credit identity is verified, used for navigation, and not guessed from nickname',async()=>{
 const raw=[{name:'Synthetic nickname',bank_id:'1234****5678',balance_text:null}]
 const full='123400005678',clicked=[]
 const page={url:()=> 'https://www.fnb.co.za/',evaluate:async(fn,arg)=>{
  if(fn===clickTransactionAccountDOM){clicked.push(arg);return true}
  if(fn===readCreditIdentityDOM)return {bank_id:full,credit:true,currency:'ZAR'}
  if(fn===readTransactionDOM)return {...table(),bank_id:full,account_type:'Credit'}
  if(fn.toString().includes('session'))return false
  return true
 }}
 const aliases=await resolveMaskedCreditAccounts(page,raw)
 assert.deepEqual(aliases,{[full]:'1234****5678'})
 const reports=await fetchTransactions({pages:async()=>[page]},[full],'run',undefined,aliases)
 assert.deepEqual(clicked,['1234****5678','1234****5678'])
 assert.equal(reports[0].bank_id,full);assert.equal(reports[0].account_type,'Credit')
 assert.ok(!JSON.stringify(reports).includes('****'))
 const original=page.evaluate
 for(const identity of [{bank_id:'999900005678',credit:true,currency:'ZAR'},{bank_id:full,credit:true,currency:'USD'},{bank_id:'1234****9999',credit:true,currency:'ZAR'}]){
  page.evaluate=async(fn,arg)=>fn===readCreditIdentityDOM?identity:original(fn,arg)
  await assert.rejects(()=>resolveMaskedCreditAccounts(page,raw),err=>{assert.equal(err.code,'ACCOUNT_LAYOUT_CHANGED');assert.equal(err.diagnostics.masked_credit_failed,1);assert.equal(err.diagnostics.masked_credit_navigation,3);assert.equal(err.diagnostics[identity.currency==='USD'?'masked_credit_non_zar':identity.bank_id.includes('*')?'masked_credit_detail_masked':'masked_credit_number_mismatch'],1);return !JSON.stringify(err).includes(full)})
 }
})
test('hidden masked identity is not navigated and ambiguous hidden suffixes reject',async()=>{
 const raw=[{name:'Synthetic',bank_id:'1234****5678'}],full='123400005678'
 const page={evaluate:async()=>{throw Error('hidden identity navigated')}}
 assert.deepEqual(await resolveMaskedCreditAccounts(page,raw,undefined,[full]),{[full]:raw[0].bank_id})
 await assert.rejects(()=>resolveMaskedCreditAccounts(page,raw,undefined,[full,'123411115678']),{code:'ACCOUNT_LAYOUT_CHANGED'})
})


test('owner-selected masked summary identity supports credit fetch without inventing digits',async()=>{
 const mask='123456******7890',rows=[{name:'Synthetic card',bank_id:mask}]
 const page={url:()=> 'https://www.fnb.co.za/',evaluate:async(fn)=>{
  if(fn===readCreditIdentityDOM)return {bank_id:mask,credit:true,currency:'ZAR'}
  if(fn===readTransactionDOM)return {...table(),bank_id:mask,account_type:'Credit'}
  if(fn.toString().includes('session'))return false
  return true
 }}
 const aliases=await resolveMaskedCreditAccounts(page,rows)
 assert.deepEqual(aliases,{[mask]:mask})
 const reports=await fetchTransactions({pages:async()=>[page]},[mask],'run',undefined,aliases)
 assert.equal(reports[0].bank_id,mask);assert.equal(reports[0].transactions.length,2)
 await assert.rejects(()=>resolveMaskedCreditAccounts(page,[...rows,...rows]),{code:'ACCOUNT_LAYOUT_CHANGED'})
 const invalid={...table(),bank_id:mask,account_type:'Savings'}
 assert.throws(()=>normalizeTransactionTable(invalid,mask,'run'),{code:'TRANSACTION_ACCOUNT_UNSUPPORTED'})
})
