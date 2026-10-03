import {usePagedList,ListStatus,ListNavigation,PagedSelect} from './PagedList'
import {useEffect,useState,useRef} from 'react'
import {Plus,Search,CheckCheck,Trash2,Link2} from 'lucide-react'
import {api,cents,decimal,money} from './api'
import {Button,Field,Badge,Empty,Modal,validateFields,Loading,Pagination} from './ui'
import {ChoiceField,CategoryChoice,GroupDot} from './Choices'
import {moneyError} from './validation'
import {useTask,type Row,type PageProps} from './App'
export function Transactions({data,revision,refresh,notify,review,period,account,unassigned,onUnassignedChange}:PageProps&{review:boolean;period:string;account:string;unassigned:boolean;onUnassignedChange:(v:boolean)=>void}){
 const[items,setItems]=useState<Row[]>([]),[q,setQ]=useState(''),[query,setQuery]=useState(''),[offset,setOffset]=useState(0),[loading,setLoading]=useState(false),[selectedRows,setSelectedRows]=useState<Record<number,Row>>({}),[editing,setEditing]=useState<Row|null>(null)
 const listVersion=useRef(''),[total,setTotal]=useState(0),[retry,setRetry]=useState(0)
 const selected=Object.keys(selectedRows).map(Number)
 const toggle=(rows:Row[],checked:boolean)=>setSelectedRows(old=>{const next={...old};for(const row of rows)if(checked)next[row.id]=row;else delete next[row.id];return next})
 const{busy,run}=useTask(notify)
 useEffect(()=>{const timer=setTimeout(()=>setQuery(q),250);return()=>clearTimeout(timer)},[q])
 useEffect(()=>{listVersion.current='';setOffset(0);setSelectedRows({})},[query,period,account,review,unassigned])
 useEffect(()=>{let alive=true;setLoading(true);const params=new URLSearchParams({q:query,offset:String(offset)});if(offset>0&&listVersion.current)params.set('list_version',listVersion.current);if(account)params.set('account',account);if(period&&!unassigned)params.set('period',period);if(review)params.set('pending','1');if(unassigned)params.set('unassigned','1')
 api('/transactions?'+params).then(v=>{if(alive){listVersion.current=v.list_version;setTotal(v.total);setItems(v.items)}}).catch(e=>{if(alive){notify(e.message,true);if(e.message==='This list changed. Start from the first page.'){listVersion.current='';setSelectedRows({});setOffset(0);setRetry(v=>v+1)}}}).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[query,offset,period,account,review,unassigned,revision,retry])
 const pagePending=items.filter(t=>t.can_edit&&t.review_state==='pending_review'),pageSelected=pagePending.length>0&&pagePending.every(t=>selected.includes(t.id))
 const approve=async()=>{const entries=Object.values(selectedRows);if(await run(()=>api('/review','POST',{items:entries.map(t=>({id:t.id,version:t.version}))}),'Transactions approved')){setSelectedRows({});refresh()}}
 return <>
 <div className="toolbar"><label className="search"><Search size={18}/><input aria-label="Search transactions" placeholder="Search descriptions" value={q} onChange={e=>setQ(e.target.value)}/></label><div className="toolbar-actions">{data.user.budget_member&&<label className="check"><input type="checkbox" checked={unassigned} onChange={e=>onUnassignedChange(e.target.checked)}/>Needs a period</label>}{selected.length>0&&<Button variant="primary" loading={busy} disabled={busy||loading} onClick={approve}><CheckCheck size={17}/>Approve {selected.length}</Button>}</div></div>
 <p className="footnote">Select up to 100 transactions across pages. Approval applies to your selection.</p>
 {loading&&<Loading>Loading transactions</Loading>}{!items.length&&!loading?<Empty title={review?'Nothing awaiting review':'No transactions found'}>{review?'New imports appear here for your check.':'Try a different filter or upload an FNB export.'}</Empty>:<section className="panel transaction-panel">
  <div className="transaction-head"><label className="check"><input aria-label="Select pending transactions on this page" disabled={loading||busy||!pageSelected&&selected.length+pagePending.filter(t=>!selected.includes(t.id)).length>100} type="checkbox" checked={items.some(t=>t.can_edit&&t.review_state==='pending_review')&&items.filter(t=>t.can_edit&&t.review_state==='pending_review').every(t=>selected.includes(t.id))} onChange={e=>toggle(items.filter(t=>t.can_edit&&t.review_state==='pending_review'),e.target.checked)}/><span className="sr-only">Select pending</span></label><span>Description / category</span><span>Amount</span></div>
  {items.map(t=><div className="transaction-row" key={t.id}><div className="row-check">{t.can_edit&&t.review_state==='pending_review'?<input type="checkbox" aria-label={'Select '+t.description} disabled={loading||busy||!selected.includes(t.id)&&selected.length>=100} checked={selected.includes(t.id)} onChange={e=>toggle([t],e.target.checked)}/>:<span/>}</div><button className="transaction-detail" onClick={()=>setEditing(t)}><div className="transaction-description"><strong>{t.description}</strong><small>{t.date} · {t.account_name}{t.household?'':' · Private'}</small><div className="row-meta">{t.spending_group_name&&<span className="group-label"><GroupDot color={t.spending_group_color}/>{t.spending_group_name}</span>}<span>{t.is_transfer?'Transfer':t.allocations.length>1?t.allocations.length+' categories':t.allocations[0]?.category_name||'Uncategorized'}</span>{t.review_state==='pending_review'&&<Badge tone="pending">Pending review</Badge>}{t.review_state==='approved'&&<Badge tone="good">Approved</Badge>}{t.household&&!t.period_id&&t.assignment!=='outside'&&<Badge>Needs a period</Badge>}{t.outside_period===1&&<Badge tone="pending">Outside assigned dates</Badge>}</div></div><strong className={'transaction-amount '+(t.amount_cents>0?'positive':'')}>{money(t.amount_cents)}</strong></button></div>)}
 </section>}
 <Pagination page={Math.floor(offset/100)} total={total} size={100} loading={loading} range onChange={page=>setOffset(page*100)}/>
 {editing&&<TransactionEditor transaction={editing} data={data} refresh={refresh} notify={notify} onClose={()=>setEditing(null)}/>}
 </>
}
function TransactionEditor({transaction:t,data,refresh,notify,onClose}:{transaction:Row;data:PageProps['data'];refresh:()=>void;notify:PageProps['notify'];onClose:()=>void}){
 const[date,setDate]=useState(t.date),[amount,setAmount]=useState(decimal(t.amount_cents)),[description,setDescription]=useState(t.description)
 const[alloc,setAlloc]=useState<Row[]>(t.allocations.map((a:Row)=>({...a,amount:decimal(a.amount_cents)})))
 const[spendingGroup,setSpendingGroup]=useState<number|null>(t.spending_group_id||null)
 const[transfer,setTransfer]=useState(!!t.is_transfer),[assignment,setAssignment]=useState(t.assignment),[period,setPeriod]=useState(String(t.period_id||''))
 const[saveRule,setSaveRule]=useState(false),[pattern,setPattern]=useState(t.description.slice(0,200)),[historyOpen,setHistoryOpen]=useState(false)
 const[counterpart,setCounterpart]=useState(''),[candidate,setCandidate]=useState<Row|null>(null)
 const{busy,run}=useTask(notify)
 const allocated=alloc.reduce((sum,a)=>{try{return sum+cents(a.amount)}catch{return sum}},0)
 let original=0;try{original=cents(amount)}catch{}
 const update=(i:number,key:string,value:unknown)=>setAlloc(v=>v.map((a,n)=>n===i?{...a,[key]:value}:a))
 const canSaveRule=alloc.length===1&&!!alloc[0]?.category_id&&!transfer&&original!==0
 const save=async()=>{
  const ok=await run(async()=>{
   const parsed=cents(amount)
   await api('/transactions/'+t.id,'PUT',{version:t.version,date,amount_cents:parsed,description,allocations:alloc.map(a=>({category_id:a.category_id?Number(a.category_id):null,amount_cents:cents(a.amount),note:a.note||''})),is_transfer:transfer,spending_group_id:spendingGroup,assignment,period_id:assignment==='manual'?Number(period):null,rule:saveRule&&canSaveRule?{pattern}:null})

  },'Transaction saved and returned to pending review')
  if(ok){refresh();onClose()}
 }
 const approve=async()=>{if(await run(()=>api('/review','POST',{items:[{id:t.id,version:t.version}]}),'Transaction approved')){refresh();onClose()}}
 return <Modal title={'Transaction #'+t.id} onClose={onClose}>
  <div className="editor-intro"><Badge tone={t.review_state==='approved'?'good':'pending'}>{t.review_state==='approved'?'Approved':'Pending review'}</Badge><span>{t.account_name}</span>{!t.can_edit&&<Badge>View only</Badge>}</div>
  <fieldset disabled={!t.can_edit||busy}><div className="form-grid"><Field label="Date"><input type="date" required value={date} onChange={e=>setDate(e.target.value)}/></Field><Field label="Signed amount (ZAR)" validate={value=>moneyError(value)} hint="Expenses are negative; income and refunds positive."><input inputMode="decimal" required value={amount} onChange={e=>{setAmount(e.target.value);if(alloc.length===1)update(0,'amount',e.target.value)}}/></Field></div><Field label="Description"><input required maxLength={1000} value={description} onChange={e=>setDescription(e.target.value)}/></Field>
  <ChoiceField source="/spending-groups" label="Spending group" value={spendingGroup} onChange={setSpendingGroup} options={data.spendingGroups.map(g=>({id:g.id,name:g.name,color:g.color}))} disabled={!t.can_edit||busy}/>
  <div className="section-head"><h3>Category allocations</h3><Button disabled={alloc.length>=100||!t.can_edit} onClick={()=>setAlloc([...alloc,{category_id:null,amount:'0.00',note:''}])}><Plus size={16}/>Add split</Button></div>
  {alloc.map((a,i)=><div className="allocation" key={i}><CategoryChoice label={alloc.length===1?'Category':'Category '+(i+1)} data={data} value={a.category_id||null} onChange={id=>update(i,'category_id',id)} refresh={refresh} notify={notify} disabled={!t.can_edit||busy}/><Field label="Amount" validate={value=>{
   const error=moneyError(value);if(error)return error
   const parsed=cents(value)
   if((original<0&&parsed>0)||(original>0&&parsed<0))return 'Use the same sign as the transaction.'
   if(i===alloc.length-1&&allocated!==original)return 'Allocations must total '+money(original)+'.'
   return ''
  }}><input inputMode="decimal" required value={a.amount} onChange={e=>update(i,'amount',e.target.value)}/></Field><Field label="Note"><input maxLength={500} value={a.note||''} onChange={e=>update(i,'note',e.target.value)}/></Field>{alloc.length>1&&<Button variant="quiet" aria-label={'Remove allocation '+(i+1)} onClick={()=>setAlloc(alloc.filter((_,n)=>n!==i))}><Trash2 size={17}/></Button>}</div>)}
  <div className={'split-total '+(original!==allocated?'negative':'')}><span>Allocated {money(allocated)}</span><strong>Remaining {money(original-allocated)}</strong></div>
  <label className="check"><input type="checkbox" checked={transfer} disabled={!!t.transfer_counterpart_id||t.transfer_counterpart_hidden} onChange={e=>setTransfer(e.target.checked)}/>This is a transfer, excluded from income and spending</label>
  {data.user.budget_member&&!!t.household&&<div className="form-grid"><Field label="Budget assignment"><select value={assignment} onChange={e=>setAssignment(e.target.value)}><option value="auto">Match period dates automatically</option><option value="manual">Assign to a specific period</option><option value="outside">Leave outside budgets</option></select></Field>{assignment==='manual'&&<Field label="Assigned period"><select required value={period} onChange={e=>setPeriod(e.target.value)}><option value="">Choose a period</option>{data.periods.map(p=><option value={p.id} key={p.id}>{p.name} · {p.start_date} – {p.end_date}</option>)}</select></Field>}</div>}
  {canSaveRule&&<div className="rule-offer"><label className="check"><input type="checkbox" checked={saveRule} onChange={e=>setSaveRule(e.target.checked)}/>Use this category and spending group for similar transactions in this account</label>{saveRule&&<Field label="Description contains" hint="Use the shop or provider name, without the changing reference number." validate={value=>value.trim().length<2?'Use at least 2 non-space characters.':new TextEncoder().encode(value.trim()).length>200?'Use a shorter description match.':''}><input required minLength={2} maxLength={200} value={pattern} onChange={e=>setPattern(e.target.value)}/></Field>}</div>}
  </fieldset>
  <div className="editor-actions">{t.can_edit&&<Button variant="primary" loading={busy} disabled={busy} onClick={e=>{if(validateFields(e.currentTarget.closest('.modal-body')!.querySelector('fieldset')!))save()}}>Save changes</Button>}{t.can_edit&&t.review_state==='pending_review'&&<Button loading={busy} disabled={busy||spendingGroup!==(t.spending_group_id||null)||date!==t.date||amount!==decimal(t.amount_cents)||description!==t.description||JSON.stringify(alloc)!==JSON.stringify(t.allocations.map((a:Row)=>({...a,amount:decimal(a.amount_cents)})))||transfer!==!!t.is_transfer||assignment!==t.assignment||period!==String(t.period_id||'')} onClick={approve}>Approve current details</Button>}<Button onClick={onClose}>Close</Button></div>
  <details className="details"><summary>Link or unlink transfer</summary>{t.transfer_counterpart_id||t.transfer_counterpart_hidden?<><p>{t.transfer_counterpart_hidden?'The linked account is not accessible to you.':'Linked to transaction #'+t.transfer_counterpart_id}</p>{t.can_edit&&!t.transfer_counterpart_hidden&&<Button loading={busy} disabled={busy} onClick={async()=>{if(await run(()=>api('/transfers/'+t.id,'DELETE'),'Transfer unlinked')){refresh();onClose()}}}>Unlink transfer</Button>}</>:t.can_edit?<><p className="muted">Use the transaction number from the opposite account. Amounts must be equal and opposite. Record fees separately.</p><div className="form-grid"><Field label="Counterpart transaction number" validate={value=>value&&!/^[1-9][0-9]*$/.test(value)?'Enter a valid transaction number.':''}><input inputMode="numeric" value={counterpart} onChange={e=>{setCounterpart(e.target.value);setCandidate(null)}}/></Field><Button loading={busy} disabled={busy||!counterpart} onClick={e=>{if(validateFields(e.currentTarget.closest('.form-grid')!))run(async()=>{const v=await api('/transactions?id='+encodeURIComponent(counterpart));if(!v.items.length)throw new Error('Transaction not found or inaccessible');setCandidate(v.items[0])})}}>Find transaction</Button></div>{candidate&&<div className="notice"><div>{candidate.date} · {candidate.description} · {candidate.account_name} · {money(candidate.amount_cents)}</div><Button loading={busy} disabled={busy} onClick={async()=>{if(await run(()=>api('/transfers','POST',{left_id:t.id,right_id:candidate.id,left_version:t.version,right_version:candidate.version}),'Transfer linked; both entries need review')){refresh();onClose()}}}><Link2 size={16}/>Link these entries</Button></div>}</>:<p>Editor access is required.</p>}</details>
  <details className="details"><summary>Original import details</summary><dl className="provenance">{Object.entries(t.provenance).map(([k,v])=><div key={k}><dt>{k.replaceAll('_',' ')}</dt><dd>{typeof v==='object'?JSON.stringify(v):String(v??'—')}</dd></div>)}</dl></details>
  <details className="details" onToggle={e=>setHistoryOpen(e.currentTarget.open)}><summary>Change history</summary>{historyOpen&&<AuditHistory id={t.id}/>}</details>
 </Modal>
}

function AuditHistory({id}:{id:number}){const list=usePagedList('/audit/'+id);return <><ListStatus list={list}/>{list.items.map(h=><div className="history-row" key={h.id}><strong>{h.action.replaceAll('_',' ')}</strong><small>{h.created_at} UTC · {h.username||'System'}</small></div>)}<ListNavigation list={list}/></>}
