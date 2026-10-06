import {MerchantAvatar} from './MerchantAvatar'
import {BulkEditor,TransactionLabels} from './CoreWorkflows'
import {useTransactionAccess} from './TransactionAccess'
import {usePagedList,ListStatus,ListNavigation,PagedSelect} from './PagedList'
import {useEffect,useState,useRef} from 'react'
import {Plus,Eye,EyeOff,Trash2,Link2} from 'lucide-react'
import {api,cents,decimal,money,download} from './api'
import {Button,Field,Badge,Empty,Modal,validateFields,Loading,Pagination,ActionMenu} from './ui'
import {ChoiceField,CategoryChoice,GroupDot,type Choice} from './Choices'
import {moneyError} from './validation'
import {proposeRulePattern} from './ruleProposal'
import {useTask,type Row,type PageProps} from './App'
export function Transactions({data,revision,refresh,notify,review,period,account,unassigned,importIds=[],onImport,onClearFilters,stagedCount=0,filterQuery=''}:PageProps&{filterQuery?:string;importIds?:number[];onImport?:()=>void;onClearFilters?:()=>void;stagedCount?:number;review:boolean;period:string;account:string;unassigned:boolean}){
 const {openTransaction}=useTransactionAccess()
 const[bulk,setBulk]=useState(false)
 const scope=new URLSearchParams(filterQuery);if(account)scope.set('account',account);if(period&&!unassigned)scope.set('period',period);if(unassigned)scope.set('unassigned','1');if(importIds.length)scope.set('imports',importIds.join(','))
 const[items,setItems]=useState<Row[]>([]),[offset,setOffset]=useState(0),[loading,setLoading]=useState(false),[selectedRows,setSelectedRows]=useState<Record<number,Row>>({})
 const[listError,setListError]=useState('')
 const listVersion=useRef(''),[total,setTotal]=useState(0),[retry,setRetry]=useState(0)
 const selected=Object.keys(selectedRows).map(Number)
 const toggle=(rows:Row[],checked:boolean)=>setSelectedRows(old=>{const next={...old};for(const row of rows)if(checked)next[row.id]=row;else delete next[row.id];return next})
 const{busy,run}=useTask(notify)
 useEffect(()=>{listVersion.current='';setOffset(0);setSelectedRows({})},[filterQuery,period,account,review,unassigned,importIds])
 useEffect(()=>{let alive=true;setLoading(true);setListError('');const params=new URLSearchParams(filterQuery);params.set('offset',String(offset));if(offset>0&&listVersion.current)params.set('list_version',listVersion.current);if(account)params.set('account',account);if(period&&!unassigned)params.set('period',period);if(importIds.length)params.set('imports',importIds.join(','));if(unassigned)params.set('unassigned','1')
 api('/transactions?'+params).then(v=>{if(alive){listVersion.current=v.list_version;setTotal(v.total);setItems(v.items)}}).catch(e=>{if(alive){notify(e.message,true);setListError(e.message);if(e.message==='This list changed. Start from the first page.'){listVersion.current='';setSelectedRows({});setOffset(0);setRetry(v=>v+1)}}}).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[filterQuery,offset,period,account,review,unassigned,revision,retry,importIds])
 const pageRows=items,pageSelected=pageRows.length>0&&pageRows.every(t=>selected.includes(t.id))
 const setSeen=async(seen:boolean)=>{const entries=Object.values(selectedRows);if(await run(()=>api('/transactions/seen','POST',{seen,items:entries.map(t=>({id:t.id,version:t.version}))}),seen?'Transactions marked seen':'Transactions marked unseen')){setSelectedRows({});listVersion.current='';setOffset(0);refresh()}}
 return <>
 <div className="ledger-tools"><span className="muted">{total} {total===1?'transaction':'transactions'}</span><div className="toolbar-actions">{onImport&&items.length>0&&<Button onClick={onImport}>Import transactions</Button>}<ActionMenu label="Transaction actions"><Button variant="quiet" disabled={busy||loading} onClick={()=>run(()=>download('/transactions/export?'+scope,'transactions.zip'))}>Export filtered transactions</Button></ActionMenu></div></div>
 {bulk&&<BulkEditor rows={Object.values(selectedRows)} data={data} notify={notify} refresh={refresh} onClose={()=>setBulk(false)} onDone={()=>{setBulk(false);setSelectedRows({});listVersion.current='';setOffset(0);refresh()}}/>}
 {selected.length>0&&<div className="toolbar"><div className="toolbar-actions"><Button disabled={busy||loading} onClick={()=>setBulk(true)}>Edit selected ({selected.length})</Button>{selected.length>0&&<Button variant="primary" loading={busy} disabled={busy||loading} onClick={()=>setSeen(true)}><Eye size={17}/>Mark seen ({selected.length})</Button>} {selected.length>0&&<Button loading={busy} disabled={busy||loading} onClick={()=>setSeen(false)}><EyeOff size={17}/>Mark unseen ({selected.length})</Button>}</div></div>}
 {selected.length>0&&<p className="footnote">{selected.length} of 100 selected · Seen/unseen applies only to you.</p>}
 {loading&&<Loading>Loading transactions</Loading>}{listError&&<p role="alert">{listError} <Button onClick={()=>setRetry(v=>v+1)}>Retry</Button></p>}{!items.length&&!loading&&!listError?<section className="panel transaction-panel"><Empty title={'No transactions found'}>{stagedCount>0?stagedCount+(stagedCount===1?' import needs':' imports need')+' attention. Resolve possible duplicates or invalid data in Import activity.':review?'There is nothing to review for these accounts and dates. Clear filters to check other transactions.':'Clear filters or import transactions to get started.'}{(filterQuery||period||account||unassigned||importIds.length>0)&&<Button onClick={onClearFilters}>Clear filters</Button>}{onImport&&<Button onClick={onImport}>{stagedCount?'Resolve import issues':'Import transactions'}</Button>}</Empty></section>:<section className="panel transaction-panel">
  <div className="transaction-head"><label className="check"><input aria-label="Select transactions on this page" disabled={loading||busy||!pageSelected&&selected.length+pageRows.filter(t=>!selected.includes(t.id)).length>100} type="checkbox" checked={pageSelected} onChange={e=>toggle(items,e.target.checked)}/><span className="sr-only">Select transactions</span></label><span>Description / category</span><span>Amount</span></div>
  {items.map(t=><div className="transaction-row" key={t.id}><div className="row-check"><input type="checkbox" aria-label={'Select '+t.description} disabled={loading||busy||!selected.includes(t.id)&&selected.length>=100} checked={selected.includes(t.id)} onChange={e=>toggle([t],e.target.checked)}/></div><button className="transaction-detail" onClick={()=>openTransaction(t.id,()=>setSelectedRows({}),scope.toString())}><div className="merchant-identity transaction-identity"><MerchantAvatar name={t.merchant_name||t.description} logo={t.merchant_logo}/><div className="transaction-description"><strong>{t.merchant_name||t.description}</strong>{t.merchant_name&&t.merchant_name!==t.description&&<small>{t.description}</small>}<small>{t.date} · {t.account_name}{t.household?'':' · Private'}</small><div className="row-meta">{t.spending_group_name&&<span className="group-label"><GroupDot color={t.spending_group_color}/>{t.spending_group_name}</span>}<span>{t.is_transfer?'Transfer':t.allocations.length>1?t.allocations.length+' categories':t.allocations[0]?.category_name||'Uncategorized'}</span>{t.review_state==='pending_review'&&<Badge tone="pending">Needs category</Badge>}<span className="seen-state">{t.seen?'Seen':'Unseen'}</span>{t.household&&!t.period_id&&t.assignment!=='outside'&&<Badge>Needs a period</Badge>}{t.outside_period===1&&<Badge tone="pending">Outside assigned dates</Badge>}</div></div></div><strong className={'transaction-amount '+(t.amount_cents>0?'positive':t.amount_cents<0?'negative':'')}>{money(t.amount_cents)}</strong></button></div>)}
 </section>}
 <Pagination page={Math.floor(offset/100)} total={total} size={100} loading={loading} range onChange={page=>setOffset(page*100)}/>
 </>
}
export function TransactionEditor({transaction:t,data,refresh,notify,onClose,onSaved,reviewQuery,processed=[],onNavigate,onSaveNext}:{reviewQuery?:string;processed?:number[];onNavigate?:(direction:'previous'|'next')=>Promise<void>;onSaveNext?:()=>Promise<void>;transaction:Row;data:PageProps['data'];refresh:()=>void;notify:PageProps['notify'];onClose:()=>void;onSaved:()=>void}){
 const {openTransaction}=useTransactionAccess()
 const[note,setNote]=useState(t.note||''),[merchant,setMerchant]=useState<number|null>(t.merchant_id||null),[tags,setTags]=useState<Row[]>(t.tags||[])
 const[nav,setNav]=useState<Row|null>(null),[leave,setLeave]=useState<null|(()=>void)>(null)
 const draft=()=>JSON.stringify({date,amount,description,alloc,spendingGroup,transfer,assignment,period,note,merchant,tags,saveRule,pattern})
 const baseline=useRef('')
 const[date,setDate]=useState(t.date),[amount,setAmount]=useState(decimal(t.amount_cents)),[description,setDescription]=useState(t.description)
 const[alloc,setAlloc]=useState<Row[]>(t.allocations.map((a:Row)=>({...a,amount:decimal(a.amount_cents)})))
 const[spendingGroup,setSpendingGroup]=useState<number|null>(t.spending_group_id||null)
 const[transfer,setTransfer]=useState(!!t.is_transfer||String(t.spending_group_name||'').trim().toLowerCase()==='transfer'),[assignment,setAssignment]=useState(t.assignment),[period,setPeriod]=useState(String(t.period_id||''))
 const[seen,setSeen]=useState(!!t.seen)
 const changeSeen=async()=>{if(await run(()=>api('/transactions/seen','POST',{seen:!seen,items:[{id:t.id,version:t.version}]}),seen?'Transaction marked unseen':'Transaction marked seen')){setSeen(!seen);onSaved();refresh()}}
 const[saveRule,setSaveRule]=useState(false),[pattern,setPattern]=useState(proposeRulePattern(t.description)),[historyOpen,setHistoryOpen]=useState(false)
 const[counterpart,setCounterpart]=useState(''),[candidate,setCandidate]=useState<Row|null>(null)
 const{busy,run}=useTask(notify)
 const allocated=alloc.reduce((sum,a)=>{try{return sum+cents(a.amount)}catch{return sum}},0)
 let original=0;try{original=cents(amount)}catch{}
 const update=(i:number,key:string,value:unknown)=>setAlloc(v=>v.map((a,n)=>n===i?{...a,[key]:value}:a))
 const canSaveRule=alloc.length===1&&!!alloc[0]?.category_id&&!transfer&&original!==0
 if(!baseline.current)baseline.current=draft()
 const requestLeave=(action:()=>void)=>{if(t.can_edit&&draft()!==baseline.current)setLeave(()=>action);else action()}
 useEffect(()=>{if(reviewQuery===undefined)return;let alive=true;const p=new URLSearchParams(reviewQuery);p.set('anchor_id',String(t.id));p.set('anchor_date',t.date);p.set('processed',processed.join(','));api('/transactions/navigation?'+p).then(v=>alive&&setNav(v)).catch(e=>notify(e.message,true));return()=>{alive=false}},[reviewQuery,t.id,t.date,processed.join(',')])
 const save=async(after?:()=>void)=>{
  const ok=await run(async()=>{
   const parsed=cents(amount)
   const result=await api('/transactions/'+t.id,'PUT',{version:t.version,date,amount_cents:parsed,description,note,merchant_id:merchant,clear_merchant:merchant===null,tag_ids:tags.map(tag=>tag.id),allocations:alloc.map(a=>({category_id:a.category_id?Number(a.category_id):null,amount_cents:cents(a.amount),note:a.note||''})),is_transfer:transfer,spending_group_id:spendingGroup,assignment,period_id:assignment==='manual'?Number(period):null,rule:saveRule&&canSaveRule?{pattern}:null})

   let message=transfer||alloc.every(a=>a.category_id)?'Transaction saved and accepted':'Transaction saved; missing categories still need review'
   const applied=result.pending_rule?.applied||0,conflicts=result.pending_rule?.conflicts||0
   if(applied)message+=`. Rule applied to ${applied} other uncategorized ${applied===1?'transaction':'transactions'}; they are accepted and unseen.`
   if(conflicts)message+=` ${conflicts} matching ${conflicts===1?'transaction needs':'transactions need'} manual categorization because rules disagree.`
   notify(message)
  })
  if(ok){baseline.current=draft();setLeave(null);onSaved();refresh();if(after)after();else onClose()}
 }
 const submit=(container:HTMLElement)=>{if(validateFields(container))save()}
 return <Modal size="wide" title={'Transaction #'+t.id} onClose={()=>requestLeave(onClose)}>
  {leave&&<Modal title="Unsaved changes" onClose={()=>setLeave(null)}><p>Save your changes before leaving this transaction?</p><div className="editor-actions"><Button variant="primary" loading={busy} onClick={e=>{const parent=Array.from(document.querySelectorAll('dialog[open]')).find(d=>d.getAttribute('aria-label')==='Transaction #'+t.id);const action=leave;setLeave(null);requestAnimationFrame(()=>{if(parent&&validateFields(parent.querySelector('fieldset')!))save(action)})}}>Save and continue</Button><Button onClick={()=>{const action=leave;setLeave(null);action()}}>Discard changes</Button><Button onClick={()=>setLeave(null)}>Stay here</Button></div></Modal>}
  <div className="transaction-editor-context">{reviewQuery!==undefined&&<nav className="toolbar" aria-label="Review transactions"><Button disabled={busy||!nav?.previous} onClick={()=>requestLeave(()=>onNavigate?.('previous'))}>Previous</Button><Button disabled={busy||!nav?.next} onClick={()=>requestLeave(()=>onNavigate?.('next'))}>Next</Button></nav>}
  <div className="editor-intro"><Badge tone={t.review_state==='approved'?'good':'pending'}>{t.review_state==='approved'?'Accepted':'Needs category'}</Badge><span>{t.account_name}</span><Badge tone={seen?'good':'pending'}>{seen?'Seen by you':'Unseen by you'}</Badge>{!t.can_edit&&<Badge>View only</Badge>}<Button variant="quiet" loading={busy} disabled={busy} onClick={changeSeen}>{seen?<EyeOff size={17}/>:<Eye size={17}/>}Mark {seen?'unseen':'seen'}</Button></div></div>
  <fieldset className="transaction-editor-fields" disabled={!t.can_edit||busy}><div className="transaction-editor-grid"><section className="transaction-editor-pane" aria-label="Transaction details"><h3>Transaction details</h3><div className="form-grid"><Field label="Date"><input type="date" required value={date} onChange={e=>setDate(e.target.value)}/></Field><Field label="Signed amount (ZAR)" validate={value=>moneyError(value)} hint="Expenses are negative; income and refunds positive."><input inputMode="decimal" required value={amount} onChange={e=>{setAmount(e.target.value);if(alloc.length===1)update(0,'amount',e.target.value)}}/></Field></div><Field label="Description"><input required maxLength={1000} value={description} onChange={e=>setDescription(e.target.value)}/></Field>
  <details className="details editor-section" open={!!t.note||!!t.merchant_id||!!t.tags?.length}><summary>Notes, merchant and tags{(note||merchant||tags.length)?' · Details added':''}</summary>
  <Field label="Transaction note"><textarea maxLength={2000} value={note} onChange={e=>setNote(e.target.value)}/></Field>
  <TransactionLabels globalMerchants={!!data.user.budget_member} account={t.account_id} merchant={merchant} tags={tags} onMerchant={(id,choice)=>{
   setMerchant(id)
   if(id&&choice?.category_id&&alloc.length===1&&!alloc[0]?.category_id){
    update(0,'category_id',choice.category_id)
    if(choice.spending_group_id&&!spendingGroup){
     setSpendingGroup(choice.spending_group_id)
    }
   }
  }} onTags={setTags} notify={notify} disabled={!t.can_edit||busy}/>
  </details>
  <details className="details editor-section" open={assignment!=='auto'}><summary>Budget assignment{assignment==='manual'?' · Specific period':assignment==='outside'?' · Outside budgets':' · Automatic'}</summary>
  {data.user.budget_member&&!!t.household&&<div className="form-grid"><Field label="Budget assignment"><select value={assignment} onChange={e=>setAssignment(e.target.value)}><option value="auto">Match period dates automatically</option><option value="manual">Assign to a specific period</option><option value="outside">Leave outside budgets</option></select></Field>{assignment==='manual'&&<Field label="Assigned period"><select required value={period} onChange={e=>setPeriod(e.target.value)}><option value="">Choose a period</option>{data.periods.map(p=><option value={p.id} key={p.id}>{p.name} · {p.start_date} – {p.end_date}</option>)}</select></Field>}</div>}
  </details>
  </section><section className="transaction-editor-pane transaction-classification" aria-label="Classification"><h3>Classification</h3>
  <ChoiceField source="/spending-groups" label="Spending group" value={spendingGroup} onChange={(id,group)=>{
   const isTransfer=group?.name.trim().toLowerCase()==='transfer'
   if((t.transfer_counterpart_id||t.transfer_counterpart_hidden)&&!isTransfer){notify('Unlink the transfer before changing its spending group.',true);return}
   setSpendingGroup(id);setTransfer(isTransfer)
  }} options={data.spendingGroups.map(g=>({id:g.id,name:g.name,color:g.color}))} disabled={!t.can_edit||busy}/>
  <div className="section-head allocation-heading"><h4>{alloc.length>1?'Split categories':'Category'}</h4><Button disabled={alloc.length>=100||!t.can_edit} onClick={()=>setAlloc([...alloc,{category_id:null,amount:'0.00',note:''}])}><Plus size={16}/>Add split</Button></div>
  {alloc.map((a,i)=><div className={'allocation editor-allocation '+(alloc.length===1?'single-allocation':'split-allocation')} key={i}><CategoryChoice label={alloc.length===1?'Category':'Category '+(i+1)} data={data} value={a.category_id||null} onChange={id=>update(i,'category_id',id)} refresh={refresh} notify={notify} disabled={!t.can_edit||busy}/>{(alloc.length>1||allocated!==original)&&<Field label="Amount" validate={value=>{
   const error=moneyError(value);if(error)return error
   const parsed=cents(value)
   if((original<0&&parsed>0)||(original>0&&parsed<0))return 'Use the same sign as the transaction.'
   if(i===alloc.length-1&&allocated!==original)return 'Category amounts must add up to '+money(original)+'.'
   return ''
  }}><input inputMode="decimal" required value={a.amount} onChange={e=>update(i,'amount',e.target.value)}/></Field>}<details className="allocation-note" open={!!a.note}><summary>{alloc.length>1?'Split note':'Category note'}{a.note?' · Added':' (optional)'}</summary><Field label="Note"><input maxLength={500} value={a.note||''} onChange={e=>update(i,'note',e.target.value)}/></Field></details>{alloc.length>1&&<Button variant="quiet" aria-label={'Remove allocation '+(i+1)} onClick={()=>setAlloc(alloc.filter((_,n)=>n!==i))}><Trash2 size={17}/></Button>}</div>)}
  {alloc.length>1&&<div className={'split-total '+(original!==allocated?'negative':'')}><span>Assigned {money(allocated)}</span><strong>Remaining {money(original-allocated)}</strong></div>}
  {transfer&&<p className="muted">Transfer excluded from income and spending. No category is required.</p>}
  {canSaveRule&&<details className="details editor-section"><summary>Automatically categorize similar transactions{saveRule?' · Enabled':''}</summary><div className="rule-offer"><h3>Proposed automatic rule</h3><p className="muted">Uses this category and group for future matches in this account, and fills eligible uncategorized entries already imported. Existing categories, splits and groups are kept. Rule-applied entries start unseen.</p><label className="check"><input type="checkbox" checked={saveRule} onChange={e=>setSaveRule(e.target.checked)}/>Use this category and spending group for similar transactions in this account</label><Field label="Description contains" hint="Use the shop or provider name, without the changing reference number." validate={value=>!saveRule?'':value.trim().length<2?'Use at least 2 non-space characters.':new TextEncoder().encode(value.trim()).length>200?'Use a shorter description match.':''}><input required={saveRule} maxLength={200} value={pattern} onChange={e=>setPattern(e.target.value)}/></Field></div></details>}
  </section></div></fieldset>
  <details className="details transaction-editor-extras"><summary>Transfer links, original details and history</summary><div className="transaction-extra-grid">

  <details className="details"><summary>Link or unlink transfer</summary>{t.transfer_counterpart_id||t.transfer_counterpart_hidden?<><p>{t.transfer_counterpart_hidden?'The linked account is not accessible to you.':<button type="button" className="transaction-link" onClick={()=>openTransaction(t.transfer_counterpart_id)}>Linked to transaction #{t.transfer_counterpart_id}</button>}</p>{t.can_edit&&!t.transfer_counterpart_hidden&&<Button loading={busy} disabled={busy} onClick={async()=>{if(await run(()=>api('/transfers/'+t.id,'DELETE'),'Transfer unlinked')){refresh();onClose()}}}>Unlink transfer</Button>}</>:t.can_edit?<><p className="muted">Enter the transaction number from the other account. The amounts must match, with one payment and one deposit. Record bank fees separately.</p><div className="form-grid transfer-lookup"><Field label="Transaction number in the other account" validate={value=>value&&!/^[1-9][0-9]*$/.test(value)?'Enter a valid transaction number.':''}><input inputMode="numeric" value={counterpart} onChange={e=>{setCounterpart(e.target.value);setCandidate(null)}}/></Field><Button loading={busy} disabled={busy||!counterpart} onClick={e=>{if(validateFields(e.currentTarget.closest('.form-grid')!))run(async()=>{const v=await api('/transactions?id='+encodeURIComponent(counterpart));if(!v.items.length)throw new Error('This transaction was not found, or you do not have access to its account.');setCandidate(v.items[0])})}}>Find transaction</Button></div>{candidate&&<div className="notice"><button type="button" className="transaction-link" onClick={()=>openTransaction(candidate.id)}>{candidate.date} · {candidate.description} · {candidate.account_name} · {money(candidate.amount_cents)}</button><Button loading={busy} disabled={busy} onClick={async()=>{if(await run(()=>api('/transfers','POST',{left_id:t.id,right_id:candidate.id,left_version:t.version,right_version:candidate.version}),'Transfer linked and accepted.')){refresh();onClose()}}}><Link2 size={16}/>Link transactions</Button></div>}</>:<p>Editor access is required.</p>}</details>
  <details className="details"><summary>Original import details</summary><dl className="provenance">{Object.entries(t.provenance).map(([k,v])=><div key={k}><dt>{k.replaceAll('_',' ')}</dt><dd>{typeof v==='object'?JSON.stringify(v):String(v??'—')}</dd></div>)}</dl></details>
  <details className="details" onToggle={e=>setHistoryOpen(e.currentTarget.open)}><summary>Change history</summary>{historyOpen&&<AuditHistory id={t.id}/>}</details>
  </div></details>
  <div className="transaction-editor-footer"><small className="muted">Saving marks this transaction seen by you.</small><div className="editor-actions">{t.can_edit&&<Button variant="primary" loading={busy} disabled={busy} onClick={e=>submit(e.currentTarget.closest('.modal-body')!.querySelector('fieldset')!)}>Save changes</Button>}{t.can_edit&&reviewQuery!==undefined&&<Button loading={busy} onClick={e=>{if(validateFields(e.currentTarget.closest('.modal-body')!.querySelector('fieldset')!))save(()=>onSaveNext?.())}}>Save and next</Button>}<Button disabled={busy} onClick={()=>requestLeave(onClose)}>Close</Button></div></div>
 </Modal>
}

function AuditHistory({id}:{id:number}){const list=usePagedList('/audit/'+id);return <><ListStatus list={list}/>{list.items.map(h=><div className="history-row" key={h.id}><strong>{h.action.replaceAll('_',' ')}</strong><small>{h.created_at} UTC · {h.username||'System'}</small></div>)}<ListNavigation list={list}/></>}
