import {test} from 'node:test'
import assert from 'node:assert/strict'
import {exactBalance,workerFailure,normalizeSnapshot,readBalanceDOM,loginAndAccounts,logoutBankSession,clickLogoutDOM,balanceFieldCountsDOM,previousSessionDOM,signedOutDOM,finishBankSession} from './refresh.mjs'
import vm from 'node:vm'
test('uses exact signed ZAR decimals and rejects ambiguous money or reward units',()=>{
 for(const [text,expected] of [['R 1,234.56','1234.56'],['-0.01','-0.01'],['R -9 876.54','-9876.54'],['0.00','0.00'],['R 0','0.00'],['R +12.3','12.30'],['R −1.23','-1.23']])assert.equal(exactBalance(text),expected)
 for(const text of ['1,23.45','eB 123.00','12.345','12,50','NaN','90071992547410.00'])assert.throws(()=>exactBalance(text),{code:'BALANCE_LAYOUT_CHANGED'})
})
test('filters unsupported identities, excludes hidden balances and strips non-account fields',()=>{
 const result=normalizeSnapshot([{name:'Synthetic',bank_id:'001234',balance_text:'R 12.34',password:'secret',transactions:[1]},{name:'Hidden',bank_id:'222222',balance_text:'not even parsed'},{name:'Rewards',bank_id:'eB',balance_text:'eB 200'}],['222222'])
 assert.deepEqual(result,{accounts:[{name:'Synthetic',bank_id:'001234',balance_decimal:'12.34'},{name:'Hidden',bank_id:'222222',balance_decimal:null}],skipped:1})
 assert.throws(()=>normalizeSnapshot([{name:'A',bank_id:'123456',balance_text:'1.00'},{name:'B',bank_id:'123456',balance_text:'2.00'}]))
})
test('DOM balance reader never accesses hidden or unsupported account balance text',()=>{
 const node=text=>({textContent:text,matches:()=>false,getClientRects:()=>[{}]})
 const names=[node('Synthetic'),node('Hidden'),node('Rewards')],numbers=[node('123456'),node('222222'),node('eB')]
 const forbidden={matches:()=>false,getClientRects:()=>[{}],get textContent(){throw Error('forbidden balance read')}}
 const balances=[node('R 1.23'),forbidden,forbidden]
 const document={querySelectorAll:selector=>selector.includes('nickname')?names:selector.includes('accountNumber')?numbers:balances}
 const raw=vm.runInNewContext('('+readBalanceDOM.toString()+')(["222222"])',{document,getComputedStyle:()=>({visibility:'visible'})})
 assert.equal(raw.rows[0].balance_text,'R 1.23');assert.equal(raw.rows[1].balance_text,null);assert.equal(raw.rows[2].balance_text,null)
})
test('login refuses to type credentials on a different origin or incompatible form',async()=>{
 let typed=false
 const page={goto:async()=>{},url:()=> 'https://example.com/',type:async()=>{typed=true}}
 await assert.rejects(()=>loginAndAccounts(page,{username:'synthetic',password:'synthetic-secret'}),{code:'LOGIN_LAYOUT_CHANGED'})
 assert.equal(typed,false)
 page.url=()=> 'https://www.fnb.co.za/';page.waitForSelector=async()=>{};page.evaluate=async()=>false
 await assert.rejects(()=>loginAndAccounts(page,{username:'synthetic',password:'synthetic-secret'}),{code:'LOGIN_LAYOUT_CHANGED'})
 assert.equal(typed,false)
})

test('missing ledger fields produce safe counts rather than mismatched balances',()=>{
 const node=text=>({textContent:text,matches:()=>false,getClientRects:()=>[{}]})
 const document={querySelectorAll:selector=>selector.includes('nickname')?[node('Synthetic')]:selector.includes('accountNumber')?[node('123456')]:[]}
 const result=vm.runInNewContext('('+readBalanceDOM.toString()+')([])',{document,getComputedStyle:()=>({visibility:'visible'})})
 assert.equal(result.rows,null);assert.equal(result.diagnostics.missing_rows,1)
 assert.ok(!JSON.stringify(result.diagnostics).includes('123456'))
})
test('unavailable balances preserve identity without inventing zero and format errors expose only counts',()=>{
 const result=normalizeSnapshot([{name:'Synthetic',bank_id:'123456',balance_text:'—'}],[],{})
 assert.equal(result.accounts[0].balance_decimal,null);assert.equal(result.diagnostics.placeholder_balances,1)
 let failure
 try{normalizeSnapshot([{name:'Synthetic secret',bank_id:'123456',balance_text:'R 1.23 CR'}],[],{ledger_nodes:1})}catch(err){failure=err}
 assert.equal(failure.code,'BALANCE_LAYOUT_CHANGED');assert.equal(failure.diagnostics.sign_suffix,1)
 for(const text of ['secret','123456','1.23','CR'])assert.ok(!JSON.stringify(failure.diagnostics).includes(text))
})

test('bank logout occurs once and waits for signed-out state',async()=>{
 let clicked=0
 const page={url:()=> 'https://www.fnb.co.za/accounts',evaluate:async fn=>{if(fn===clickLogoutDOM){clicked++;return true}return true}}
 assert.equal(await logoutBankSession({pages:async()=>[page]}),true)
 assert.equal(clicked,1)
 const foreign={url:()=> 'https://example.com/',evaluate:async()=>{throw Error('must not inspect foreign page')}}
 assert.equal(await logoutBankSession({pages:async()=>[foreign]}),false)
})

test('evaluation failures return fallback counts without raw browser errors',async()=>{
 const page={url:()=> 'https://www.fnb.co.za/accounts',goto:async()=>{},waitForSelector:async()=>{},type:async()=>{},browserContext:()=>({pages:async()=>[page]}),evaluate:async fn=>{
  if(fn===previousSessionDOM)return false
  if(fn===readBalanceDOM)throw Error('synthetic-secret browser content')
  if(fn===balanceFieldCountsDOM)return {name_nodes:13,number_nodes:13,ledger_nodes:0}
  return true
 }}
 let error;try{await loginAndAccounts(page,{username:'synthetic',password:'synthetic-secret'})}catch(err){error=err}
 assert.equal(error.code,'BALANCE_LAYOUT_CHANGED');assert.equal(error.diagnostics.evaluation_failed,1);assert.equal(error.diagnostics.ledger_nodes,0)
 assert.ok(!JSON.stringify(error.diagnostics).includes('secret'))
})

test('Log Off control is accepted without following foreign links',()=>{
 let clicked=0
 const control={textContent:'Log Off',tagName:'BUTTON',getClientRects:()=>[{}],click:()=>{clicked++}}
 const document={querySelectorAll:()=>[control]}
 assert.equal(vm.runInNewContext('('+clickLogoutDOM.toString()+')()',{document,getComputedStyle:()=>({visibility:'visible'})}),true)
 assert.equal(clicked,1)
})

test('previous-session warning stops account waiting immediately with a fixed error',async()=>{
 for(const text of ['Your session will be terminated. Will be logged out shortly','You will be logged out shortly']){
  assert.equal(vm.runInNewContext('('+previousSessionDOM.toString()+')()',{document:{body:{innerText:text}}}),true)
 }
 assert.equal(vm.runInNewContext('('+previousSessionDOM.toString()+')()',{document:{body:{innerText:'Accounts summary'}}}),false)
 let accountReads=0
 const page={url:()=> 'https://www.fnb.co.za/accounts',goto:async()=>{},waitForSelector:async()=>{},type:async()=>{},browserContext:()=>({pages:async()=>[page]}),evaluate:async fn=>{
  if(fn===previousSessionDOM)return true
  if(fn===readBalanceDOM)accountReads++
  return true
 }}
 await assert.rejects(()=>loginAndAccounts(page,{username:'synthetic',password:'synthetic-secret'}),{code:'SESSION_CONFLICT'})
 assert.equal(accountReads,0)
})

test('ledger selector ignores the separate available balance column',()=>{
 const node=text=>({textContent:text,matches:()=>false,getClientRects:()=>[{}]})
 const forbidden={get textContent(){throw Error('available balance must not be read')}}
 const document={querySelectorAll:selector=>selector==='[name="nickname"]'?[node('Synthetic')]:selector==='[name="accountNumber"]'?[node('123456')]:selector==='[name="ledgerBalance"]'?[node('R 1.23')]:[forbidden]}
 const result=vm.runInNewContext('('+readBalanceDOM.toString()+')([])',{document,getComputedStyle:()=>({visibility:'visible'})})
 assert.equal(result.rows[0].balance_text,'R 1.23');assert.equal(result.diagnostics.ledger_nodes,1)
})

test('completed logout confirmation is accepted without login fields, never while authenticated',()=>{
 const confirmation=text=>vm.runInNewContext('('+signedOutDOM.toString()+')()',{document:{body:{innerText:text},querySelector:()=>null,querySelectorAll:()=>[]},getComputedStyle:()=>({visibility:'visible'})})
 for(const text of ['You have successfully logged out of banking','You have been logged out','You have successfully logged off','You are now logged out','Log off successful','You’ve been signed out','Logged out successfully'])assert.equal(confirmation(text),2)
 for(const text of ['Will be logged out shortly','You will be logged out shortly','Session will be terminated','Accounts','You are being logged out'])assert.equal(confirmation(text),0)
 const visible={textContent:'Log off',getClientRects:()=>[{}]}
 assert.equal(vm.runInNewContext('('+signedOutDOM.toString()+')()',{document:{body:{innerText:'You have been logged out'},querySelector:()=>null,querySelectorAll:selector=>selector.includes('nickname')?[visible]:[]},getComputedStyle:()=>({visibility:'visible'})}),0)
})
test('logout follows bank frames and navigation races without inspecting foreign frames',async()=>{
 let reads=0
 const frame={url:()=> 'https://www.fnb.co.za/logout',evaluate:async fn=>{if(fn===clickLogoutDOM)return true;if(reads++===0)throw Error('navigation');return 2}}
 const foreign={url:()=> 'https://example.com/',evaluate:async()=>{throw Error('must not inspect')}}
 const page={url:()=> 'https://www.fnb.co.za/',frames:()=>[frame,foreign]}
 const diagnostics={}
 assert.equal(await logoutBankSession({pages:async()=>[page]},diagnostics,{attempts:3,delayMs:0}),true)
 assert.deepEqual(diagnostics,{logout_clicked:1,logout_confirmed:1})
})
test('unconfirmed logout remains a failure, and confirmed logout preserves balance failure',async()=>{
 const page={url:()=> 'https://www.fnb.co.za/',evaluate:async fn=>fn===clickLogoutDOM}
 const diagnostics={}
 await assert.rejects(()=>logoutBankSession({pages:async()=>[page]},diagnostics,{attempts:1,delayMs:0}),{code:'LOGOUT_REQUIRED'})
 assert.equal(diagnostics.logout_unconfirmed,1)
 page.evaluate=async()=>true
 const result=await finishBankSession({pages:async()=>[page]},{accounts:[],skipped:0,error:'BALANCE_LAYOUT_CHANGED',diagnostics:{unknown_text:1}})
 assert.equal(result.error,'BALANCE_LAYOUT_CHANGED');assert.equal(result.diagnostics.logout_confirmed,1)
})
test('balance reader uses rendered ledger text rather than hidden helper text',()=>{
 const node=text=>({textContent:text,matches:()=>false,getClientRects:()=>[{}]})
 const ledger={...node('R 1.23 hidden helper'),innerText:'R 1.23'}
 const document={querySelectorAll:selector=>selector.includes('nickname')?[node('Synthetic')]:selector.includes('accountNumber')?[node('123456')]:[ledger]}
 const result=vm.runInNewContext('('+readBalanceDOM.toString()+')([])',{document,getComputedStyle:()=>({visibility:'visible'})})
 assert.equal(result.rows[0].balance_text,'R 1.23')
})

test('failed cleanup discards snapshots and preserves the separate balance-failure counter',async()=>{
 const page={url:()=> 'https://www.fnb.co.za/',evaluate:async()=>false}
 const result=await finishBankSession({pages:async()=>[page]},{accounts:[{name:'Synthetic',bank_id:'123456',balance_decimal:'1.23'}],skipped:0})
 assert.equal(result.error,'LOGOUT_REQUIRED');assert.deepEqual(result.accounts,[]);assert.equal(result.diagnostics.logout_unconfirmed,1)
 const broken={pages:async()=>{throw Error('synthetic browser failure')}}
 const failure=await finishBankSession(broken,{accounts:[],skipped:0,error:'BALANCE_LAYOUT_CHANGED',diagnostics:{unknown_text:1}})
 assert.equal(failure.diagnostics.balance_failure,1);assert.equal(failure.diagnostics.unknown_text,1)
 assert.ok(!JSON.stringify(failure).includes('browser failure'))
})

test('rewards and foreign currency balances do not fail or enter a ZAR snapshot',()=>{
 const result=normalizeSnapshot([
  {name:'Synthetic ZAR',bank_id:'111111',balance_text:'R -12.34'},
  {name:'Synthetic reward',bank_id:'222222',balance_text:'eB 75.00'},
  {name:'Synthetic CHF',bank_id:'333333',balance_text:'CHF 8.50'},
  {name:'Synthetic EUR',bank_id:'444444',balance_text:'€ 3.21'},
  {name:'Synthetic masked card',bank_id:'123***000',balance_text:'R -50.00'},
  {name:'Synthetic loan',bank_id:'555555',balance_text:'R -1,234.56'}
 ],[],{})
 assert.equal(result.skipped,4)
 assert.deepEqual(result.accounts.map(row=>row.bank_id),['111111','555555'])
 assert.deepEqual(result.accounts.map(row=>row.balance_decimal),['-12.34','-1234.56'])
 assert.equal(result.diagnostics.reward_entries,1);assert.equal(result.diagnostics.non_zar_entries,2)
 assert.ok(!JSON.stringify(result.diagnostics).includes('75.00'))
 assert.throws(()=>normalizeSnapshot([{name:'Synthetic unknown',bank_id:'666666',balance_text:'Unexpected text'}]),{code:'BALANCE_LAYOUT_CHANGED'})
 assert.throws(()=>normalizeSnapshot([{name:'Only foreign',bank_id:'777777',balance_text:'CHF 8.50'}]),{code:'ACCOUNT_LIST_NOT_FOUND'})
})

test('unconfirmed logout discards all transaction reports as well as balances',async()=>{
 const context={pages:async()=>[{url:()=> 'https://www.fnb.co.za/',evaluate:async()=>false}]}
 const result=await finishBankSession(context,{accounts:[{name:'Synthetic',bank_id:'00123456'}],reports:[{transactions:[{description:'Synthetic secret bank row'}]}],skipped:0})
 assert.equal(result.error,'LOGOUT_REQUIRED');assert.equal(result.accounts.length,0);assert.equal(result.reports,undefined)
})


test('resolved masked credit summary uses full identity and preserves hidden balance exclusion',()=>{
 const full='123400005678',mask='1234****5678'
 const node=text=>({textContent:text,innerText:text,matches:()=>false,getClientRects:()=>[{}]})
 const names=[node('Synthetic credit')],numbers=[node(mask)]
 let balanceReads=0
 const balance={matches:()=>false,getClientRects:()=>[{}],get innerText(){balanceReads++;return 'R -12.34'}}
 const document={querySelectorAll:selector=>selector.includes('nickname')?names:selector.includes('accountNumber')?numbers:[balance]}
 const read=hidden=>vm.runInNewContext('('+readBalanceDOM.toString()+')(options)',{document,options:{hidden,aliases:{[full]:mask}},getComputedStyle:()=>({visibility:'visible'})})
 let result=read([full]);assert.equal(result.rows[0].bank_id,full);assert.equal(result.rows[0].balance_text,null);assert.equal(balanceReads,0)
 result=read([]);assert.equal(result.rows[0].bank_id,full);assert.equal(balanceReads,1)
 assert.equal(normalizeSnapshot(result.rows).accounts[0].balance_decimal,'-12.34')
})


test('masked credit discovery requires verified product marker and retains ZAR/duplicate checks',()=>{
 const mask='123456******7890',card={name:'Synthetic card',bank_id:mask,account_type:'Credit',balance_text:'R -12.34'}
 const snapshot=normalizeSnapshot([card]);assert.equal(snapshot.accounts[0].bank_id,mask);assert.equal(snapshot.accounts[0].account_type,'Credit');assert.equal(snapshot.accounts[0].balance_decimal,'-12.34')
 assert.throws(()=>normalizeSnapshot([{...card,account_type:'Savings'}]),{code:'ACCOUNT_LIST_NOT_FOUND'})
 assert.throws(()=>normalizeSnapshot([{...card,balance_text:'USD 1.00'}]),{code:'ACCOUNT_LIST_NOT_FOUND'})
 assert.throws(()=>normalizeSnapshot([card,card]),{code:'ACCOUNT_LAYOUT_CHANGED'})
})


test('worker failures distinguish startup and timeout without exposing exception text',()=>{
 const secret=new Error('synthetic password and browser command details')
 assert.deepEqual(workerFailure(secret,4),{accounts:[],skipped:0,error:'CONNECTOR_START_FAILED',diagnostics:{refresh_phase:4}})
 assert.equal(workerFailure(secret,5).error,'REFRESH_FAILED')
 assert.equal(workerFailure(secret,5,true).error,'CONNECTOR_TIMEOUT')
 assert.ok(!JSON.stringify(workerFailure(secret,5,true)).includes(secret.message))
})
