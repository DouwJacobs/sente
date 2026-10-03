// GPL-3.0; navigation/table selectors adapted from the pinned fnb-api source.
// See ../upstream/LICENSE and ../UPSTREAM.json. No upstream runtime/float parser.
import {ProbeError} from './accounts.mjs'

export function transactionDate(text){
 const months={Jan:'01',Feb:'02',Mar:'03',Apr:'04',May:'05',Jun:'06',Jul:'07',Aug:'08',Sep:'09',Oct:'10',Nov:'11',Dec:'12'}
 const match=/^(\d{1,2}) ([A-Z][a-z]{2}) (\d{4})$/.exec(text.trim())
 const iso=match&&months[match[2]]?`${match[3]}-${months[match[2]]}-${match[1].padStart(2,'0')}`:text.trim()
 if(!/^\d{4}-\d{2}-\d{2}$/.test(iso)||!Number.isFinite(Date.parse(iso+'T00:00:00Z'))||new Date(iso+'T00:00:00Z').toISOString().slice(0,10)!==iso)throw new ProbeError('TRANSACTION_LAYOUT_CHANGED')
 return iso
}
export function transactionMoney(text){
 if(typeof text!=='string')throw new ProbeError('TRANSACTION_LAYOUT_CHANGED')
 const value=text.trim().replace(/−/g,'-').replace(/^(?:R|ZAR)\s*/,'').replace(/\s/g,'')
 if(!/^[+-]?(?:\d+|\d{1,3}(?:,\d{3})+)(?:\.\d{1,2})?$/.test(value))throw new ProbeError('TRANSACTION_LAYOUT_CHANGED')
 const plain=value.replace(/,/g,''),negative=plain.startsWith('-')
 const [whole,fraction='']=plain.replace(/^[+-]/,'').split('.')
 const cents=BigInt(whole)*100n+BigInt(fraction.padEnd(2,'0'))
 if(cents>9007199254740991n)throw new ProbeError('TRANSACTION_LAYOUT_CHANGED')
 return (negative&&cents!==0n?'-':'')+BigInt(whole).toString()+'.'+fraction.padEnd(2,'0')
}
const headerNames={date:'date','transaction date':'date','posted date':'date','effective date':'date',description:'description',reference:'reference','service fee':'fee',amount:'amount','transaction amount':'amount',balance:'balance','running balance':'balance',status:'status'}
export function normalizeTransactionTable(table,bankID,runID){
 const diagnostics={transaction_headers:table?.headers?.length||0,transaction_rows:table?.rows?.length||0,transaction_invalid_rows:0,transaction_successful_controls:table?.successful_controls||0,transaction_pending_controls:table?.pending_controls||0,transaction_selected_successful:table?.selected_successful||0}
 const fail=code=>{diagnostics.transaction_invalid_rows++;throw new ProbeError(code,diagnostics)}
 if(table?.bank_id!==bankID)return fail('TRANSACTION_ACCOUNT_MISMATCH')
 const type=table.account_type
 if(!['Cheque','Savings','Credit','Easy','Home Loan'].includes(type)){diagnostics.transaction_unsupported_type=1;return fail('TRANSACTION_ACCOUNT_UNSUPPORTED')}
 if(!Array.isArray(table.headers)||!Array.isArray(table.rows)||table.rows.length>150)return fail('TRANSACTION_LAYOUT_CHANGED')
 const columns=table.headers.map(h=>headerNames[h.trim().toLowerCase().replace(/\s+/g,' ')])
 if(columns.some(c=>!c)||new Set(columns).size!==columns.length||!['date','description','amount'].every(c=>columns.includes(c)))return fail('TRANSACTION_LAYOUT_CHANGED')
 // No positional guesses, nor a Pending/Available table masquerading as history.
 if(!columns.includes('status')&&!table.posted_history)return fail('TRANSACTION_LAYOUT_CHANGED')
 if(table.currency&&!/^(?:ZAR|Rand|South African Rand)$/i.test(table.currency)){diagnostics.transaction_unsupported_currency=1;return fail('TRANSACTION_ACCOUNT_UNSUPPORTED')}
 if(!table.rows.length&&!table.empty_confirmed)return fail('TRANSACTION_LAYOUT_CHANGED')
 const transactions=[]
 for(const cells of table.rows){
  if(!Array.isArray(cells)||cells.length!==columns.length||cells.some(c=>typeof c!=='string'))return fail('TRANSACTION_LAYOUT_CHANGED')
  const row=Object.fromEntries(columns.map((c,i)=>[c,cells[i].trim()]))
  if(columns.includes('status')&&!row.status)return fail('TRANSACTION_LAYOUT_CHANGED')
  if(row.status&&/^pending$/i.test(row.status))continue
  if(row.status&&!/^(?:posted|successful|completed)$/i.test(row.status))return fail('TRANSACTION_LAYOUT_CHANGED')
  if(!row.description||Buffer.byteLength(row.description)>4000||Buffer.byteLength(row.reference||'')>256)return fail('TRANSACTION_LAYOUT_CHANGED')
  try{
   const fee=row.fee?transactionMoney(row.fee):'0.00'
   if(fee.startsWith('-'))return fail('TRANSACTION_LAYOUT_CHANGED')
   transactions.push({date:transactionDate(row.date),description:row.description,amount_decimal:transactionMoney(row.amount),status:'posted',...(row.reference?{source_reference:row.reference}:{}),...(row.balance?{balance_decimal:transactionMoney(row.balance)}:{}),service_fee_decimal:fee})
  }catch{return fail('TRANSACTION_LAYOUT_CHANGED')}
 }
 return {run_id:runID,bank_id:bankID,currency:'ZAR',account_type:type,page_rows:table.rows.length,transactions}
}

// Runs in the bank browser. Only normalized text fields cross the process
// boundary on success; failure exposes fixed numeric counts, never page dumps.
export function readTransactionDOM(){
 const visible=node=>!!node&&node.getClientRects().length>0&&getComputedStyle(node).visibility!=='hidden'
 const text=node=>(node?.innerText||'').trim()
 if(visible(document.querySelector('#loaderOverlay:not(.Hhide)')))return {bank_id:'',headers:[],rows:[]}
 const fields=[...document.querySelectorAll('.dlTitle')].filter(visible)
 const field=label=>{const matches=fields.filter(node=>label.test(text(node)));return matches.length===1?text(matches[0].nextElementSibling):''}
 const bank_id=field(/^Account\s*(?:number|no\.?)$/i).replace(/\s/g,'')
 const type=field(/^Type$/i)
 const currency=field(/^Currency$/i)
 const account_type=/\bHome\s+Loan\b/i.test(type)?'Home Loan':/Cheque|Business Account|Fusion|Current/i.test(type)?'Cheque':/Credit/i.test(type)?'Credit':/Savings|\bMoney\s+Maximi[sz]er\b/i.test(type)?'Savings':/Easy Account/i.test(type)?'Easy':''

 const rowNodes=[...document.querySelectorAll('.tableRow')].filter(visible)
 const key=value=>value.toLowerCase().replace(/\s+/g,' ').trim()
 const known=new Set(['date','transaction date','posted date','effective date','description','reference','service fee','amount','transaction amount','balance','running balance','status'])
 const validHeader=values=>values.length>=3&&new Set(values.map(key)).size===values.length&&values.every(value=>known.has(key(value)))&&['date','description','amount'].every(required=>values.some(value=>key(value)===required||key(value)==='transaction '+required||key(value)==='posted '+required||(required==='date'&&key(value)==='effective date')))
 const leafLabels=root=>[...root.querySelectorAll('*')].filter(node=>visible(node)&&known.has(key(text(node)))&&![...(node.children||[])].some(child=>visible(child)&&key(text(child))===key(text(node))))
 let headers=[...document.querySelectorAll('.tableHeader .tableCellItem, .tableHeader .tableCell, .tableHeaderCell, thead th, [role="columnheader"]')].filter(node=>visible(node)&&!node.querySelector('.tableCellItem')).map(text)
 let headerNode=null
 const rowHeaders=rowNodes.map(node=>[node,[...node.querySelectorAll('.tableCell .tableCellItem')].filter(visible).map(text)]).filter(([,values])=>validHeader(values))
 if(rowHeaders.length===1){[headerNode,headers]=rowHeaders[0]}
 if(rowHeaders.length>1)headers=[]
 // Bank headers need not use the old tableHeader classes. Find a single named
 // header immediately above the rows, within their shared container. We never
 // guess the columns from amounts or from their positional order.
 if(!validHeader(headers)&&rowNodes.length&&rowHeaders.length===0){
  const first=rowNodes[0],last=rowNodes.at(-1),candidates=new Map()
  let root=first.parentElement
  for(let depth=0;root&&depth<8;depth++,root=root.parentElement){
   if(!root.contains(last))continue
   const labels=leafLabels(root).filter(node=>!first.contains(node)&&!!(node.compareDocumentPosition(first)&4))
   for(const label of labels){
    let parent=label.parentElement
    for(let level=0;parent&&root.contains(parent)&&level<6;level++,parent=parent.parentElement){
     if(parent.contains(first))break
     const values=leafLabels(parent).map(text)
     if(validHeader(values)){candidates.set(parent,values);break}
    }
   }
   // Remove wrapper duplicates and reject multiple independent headers.
   const distinct=[...candidates].filter(([node])=>![...candidates.keys()].some(other=>other!==node&&node.contains(other)))
   if(distinct.length===1){[headerNode,headers]=distinct[0];break}
   if(distinct.length>1){headers=[];break}
  }
 }
 const rows=rowNodes.filter(node=>node!==headerNode&&!headerNode?.contains(node)).map(row=>[...row.querySelectorAll('.tableCell .tableCellItem')].filter(visible).map(text))
 const controls=[...document.querySelectorAll('button,a,label,input,[role="tab"],.subTabText,span,div')].filter(node=>visible(node)&&/^(?:Successful|Pending)$/i.test(text(node)||node.value||'')&&![...(node.children||[])].some(child=>visible(child)&&/^(?:Successful|Pending)$/i.test(text(child))))
 const selected=node=>{
  for(let depth=0;node&&depth<3;depth++,node=node.parentElement){
   if(node.getAttribute('aria-selected')==='false'||node.getAttribute('aria-pressed')==='false')return false
   if(node.checked===true||node.getAttribute('aria-selected')==='true'||node.getAttribute('aria-pressed')==='true'||(node.matches('label,button,[role=tab]')&&node.querySelector('input[type=radio]:checked, input[type=checkbox]:checked')))return true
   const classes=typeof node.className==='string'?node.className:''
   if(!/inactive|unselected|unchecked/i.test(classes)&&/active|selected|checked|current/i.test(classes))return true
  }
  return false
 }
 const successful=controls.filter(node=>/^Successful$/i.test(text(node)||node.value||'')),pending=controls.filter(node=>/^Pending$/i.test(text(node)||node.value||''))
 const selected_successful=successful.length===1&&selected(successful[0])&&!pending.some(selected)
 const headings=[...document.querySelectorAll('h1,h2,h3,.subTabText')].filter(visible).map(text)
 // Owner-observed loan history has no status toggles: effective date and a
 // running balance identify its distinct four-column ledger. Require the
 // exact named contract and verified Home Loan type, never arbitrary rows.
 const loan_history=account_type==='Home Loan'&&headers.length===4&&['effective date','description','amount','balance'].every(label=>headers.some(value=>key(value)===label))
 const posted_history=controls.length?selected_successful:(loan_history||headings.some(value=>/^Transaction history$|^Posted transactions$|^Successful transactions$/i.test(value)))&&!headings.some(value=>/pending/i.test(value))

 const empty_confirmed=[...document.querySelectorAll('.table,.tableContainer')].filter(visible).some(node=>/^No (?:posted |successful )?transactions(?: found| available)?\.?$/i.test(text(node)))
 return {bank_id,account_type,currency,headers,rows,posted_history,empty_confirmed,successful_controls:successful.length,pending_controls:pending.length,selected_successful:selected_successful?1:0}
}
export function clickTransactionAccountDOM(bankID){
 const visible=node=>!!node&&node.getClientRects().length>0&&getComputedStyle(node).visibility!=='hidden'
 const names=[...document.querySelectorAll('[name="nickname"]')].filter(visible)
 const numbers=[...document.querySelectorAll('[name="accountNumber"]')].filter(visible)
 if(names.length!==numbers.length)return false
 const matches=numbers.map((node,i)=>(node.innerText||'').replace(/\s/g,'')===bankID?i:-1).filter(i=>i>=0)
 if(matches.length!==1)return false
 const control=names[matches[0]].querySelector('a')
 if(!visible(control))return false
 if(control.href&&!control.href.startsWith('javascript:')){const u=new URL(control.href,location.href);if(u.protocol!=='https:'||!(u.hostname==='fnb.co.za'||u.hostname.endsWith('.fnb.co.za')))return false}
 control.click();return true
}
export function clickTransactionTabDOM(){
 const controls=[...document.querySelectorAll('.subTabText')].filter(node=>node.getClientRects().length>0&&/^Transactions?(?: history)?$/i.test((node.innerText||'').trim()))
 if(controls.length!==1)return false
 const control=controls[0].closest('.subTabButton')||controls[0]
 control.click();return true
}
export function clickSuccessfulDOM(){
 const visible=node=>node.getClientRects().length>0&&getComputedStyle(node).visibility!=='hidden'
 const text=node=>(node.innerText||node.value||'').trim()
 const controls=[...document.querySelectorAll('button,a,label,input,[role="tab"],.subTabText,span,div')].filter(node=>visible(node)&&/^Successful$/i.test(text(node))&&![...(node.children||[])].some(child=>visible(child)&&/^Successful$/i.test(text(child))))
 if(!controls.length)return true // Legacy history/status tables verify separately.
 if(controls.length!==1)return false
 const control=controls[0].closest('a,button,label,[role="tab"]')||controls[0]
 if(control.href&&!control.href.startsWith('javascript:')){const u=new URL(control.href,location.href);if(u.protocol!=='https:'||!(u.hostname==='fnb.co.za'||u.hostname.endsWith('.fnb.co.za')))return false}
 control.click();return true
}
export function clickAccountsDOM(){
 const controls=[...document.querySelectorAll('.shortCutLink')].filter(node=>node.getClientRects().length>0&&/\bAccounts\b/i.test(node.innerText||''))
 if(controls.length!==1)return false
 controls[0].click();return true
}
function approved(page){try{const u=new URL(page.url());return u.protocol==='https:'&&(u.hostname==='fnb.co.za'||u.hostname.endsWith('.fnb.co.za'))}catch{return false}}
async function waitFor(page,signal,read,accept,arg){
 let last
 for(let i=0;i<30;i++){
  if(signal?.aborted)throw new ProbeError('REFRESH_FAILED')
  if(!approved(page))throw new ProbeError('TRANSACTION_LAYOUT_CHANGED')
  if(await page.evaluate(()=>/session\s+will\s+be\s+terminated|(?:will\s+be\s+)?logged\s+out\s+shortly/i.test(document.body?.innerText||'')))throw new ProbeError('SESSION_CONFLICT')
  try{const value=await page.evaluate(read,arg);last=value;if(accept(value))return value}catch(err){if(err instanceof ProbeError)throw err}
  await new Promise(resolve=>setTimeout(resolve,500))
 }
 throw new ProbeError('TRANSACTION_LAYOUT_CHANGED',read===readTransactionDOM?{transaction_rows:last?.rows?.length||0,transaction_headers:last?.headers?.length||0,transaction_identity_fields:last?.bank_id?1:0,transaction_type_fields:last?.account_type?1:0,transaction_successful_controls:last?.successful_controls||0,transaction_pending_controls:last?.pending_controls||0,transaction_selected_successful:last?.selected_successful||0}:undefined)
}
export async function fetchTransactions(context,bankIDs,runID,signal){
 if(!Array.isArray(bankIDs)||!bankIDs.length||bankIDs.length>100||new Set(bankIDs).size!==bankIDs.length||bankIDs.some(id=>!/^\d{3,64}$/.test(id))||!runID)throw new ProbeError('TRANSACTION_LAYOUT_CHANGED')
 const pages=(await context.pages()).filter(approved)
 let page
 for(const candidate of pages)if(await candidate.evaluate(()=>!!document.querySelector('[name="nickname"]'))){page=candidate;break}
 if(!page)throw new ProbeError('TRANSACTION_LAYOUT_CHANGED')
 const reports=[]
 let phase=0
 try {
 for(const bankID of bankIDs){
  if(reports.length){await waitFor(page,signal,clickAccountsDOM,Boolean);await waitFor(page,signal,()=>[...document.querySelectorAll('[name="nickname"]')].filter(n=>n.getClientRects().length).length>0,Boolean)}
  phase=1
  await waitFor(page,signal,clickTransactionAccountDOM,Boolean,bankID)
  phase=2
  await waitFor(page,signal,clickTransactionTabDOM,Boolean)
  await waitFor(page,signal,clickSuccessfulDOM,Boolean)
  phase=3
  const table=await waitFor(page,signal,readTransactionDOM,value=>{if(!value.bank_id||!value.headers.length||(!value.rows.length&&!value.empty_confirmed))return false;return true})
  phase=4
  reports.push(normalizeTransactionTable(table,bankID,runID))
 }
 } catch(err) {
  if(err instanceof ProbeError)err.diagnostics={...err.diagnostics,transaction_accounts:reports.length,transaction_accounts_requested:bankIDs.length,transaction_failed_account_position:reports.length+1,transaction_navigation:phase}
  throw err
 }
 return reports
}
