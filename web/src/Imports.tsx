import {useTransactionAccess} from './TransactionAccess'
import {PagedSelect,usePagedList,ListStatus,ListNavigation} from './PagedList'
import {useEffect,useRef,useState} from 'react'
import {Upload,FileText} from 'lucide-react'
import {api,money} from './api'
import {Button,Field,Badge,Empty,Loading,Pagination} from './ui'
import {useTask,type PageProps,type Row} from './App'
export function Imports({data,revision,refresh,notify,onReview,onTransactions,onBanking,filterQuery='',onClearFilters}:PageProps&{filterQuery?:string;onClearFilters?:()=>void;onReview:(ids:number[])=>void;onTransactions:(ids:number[])=>void;onBanking:()=>void}){
 const[connection,setConnection]=useState<Row|null>(null),[connectionLoading,setConnectionLoading]=useState(true),[connectionError,setConnectionError]=useState('')
 const processed=useRef(new Set<number>())
 const[processing,setProcessing]=useState(false)
 const[readyImports,setReadyImports]=useState<Record<number,()=>Promise<boolean>>>({}),[adding,setAdding]=useState(false)
 useEffect(()=>{if(!data.user.admin){setConnectionLoading(false);return}let alive=true;setConnectionLoading(true);api('/fnb?summary=1').then(v=>{if(alive){setConnection(v);setConnectionError('')}}).catch(e=>alive&&setConnectionError(e.message)).finally(()=>alive&&setConnectionLoading(false));return()=>{alive=false}},[revision,data.user.admin])
 const register=(id:number,action:(()=>Promise<boolean>)|null)=>setReadyImports(old=>{if(old[id]===action)return old;const next={...old};if(action)next[id]=action;else delete next[id];return next})
 const continueImported=async(ids:number[])=>{try{const pending=await api('/transactions?pending=1&imports='+ids.join(','));if(pending.total)onReview(ids);else onTransactions(ids)}catch(e){notify((e as Error).message,true)}}
 const addReady=async()=>{if(adding)return;setAdding(true);const added:number[]=[];try{for(const [id,commit] of Object.entries(readyImports).slice(0,100)){if(!await commit())break;added.push(Number(id))}if(added.length){refresh();await continueImported(added)}}finally{setAdding(false)}}
 const readyCount=Math.min(100,Object.keys(readyImports).length)
 const editors=data.accounts.filter(a=>a.role==='editor')
 const[account,setAccount]=useState(String(editors[0]?.id||'')),[files,setFiles]=useState<File[]>([]),[previews,setPreviews]=useState<Row[]>([]),[history,setHistory]=useState<Row[]>([])
 const historyStamp=useRef({version:'',revision,filterQuery})
 const[autoHistory,setAutoHistory]=useState<Row[]>([])
 useEffect(()=>{setHistoryPage(0);historyStamp.current.version=''},[filterQuery])
 useEffect(()=>{if(!filterQuery)return;let alive=true;api('/imports?page=0').then(v=>alive&&setAutoHistory(v.items)).catch(e=>alive&&notify(e.message,true));return()=>{alive=false}},[!!filterQuery,revision])
 const[historyPage,setHistoryPage]=useState(0),[historyTotal,setHistoryTotal]=useState(0),[loading,setLoading]=useState(true),[historyError,setHistoryError]=useState(''),[retry,setRetry]=useState(0)
 const{busy,run}=useTask(notify);const input=useRef<HTMLInputElement>(null),dragDepth=useRef(0)
 const[dragging,setDragging]=useState(false)
 const openPicker=()=>{if(!busy&&input.current){input.current.value='';input.current.click()}}
 const selectFiles=(next:File[])=>{if(busy||!next.length)return;setFiles(next);setPreviews([])}
 useEffect(()=>{let alive=true;setLoading(true);setHistoryError('');api('/imports?page='+historyPage+(filterQuery?'&'+filterQuery:'')+(historyPage>0&&historyStamp.current.revision===revision&&historyStamp.current.filterQuery===filterQuery?'&list_version='+historyStamp.current.version:'')).then(v=>{if(alive){if(historyPage>0&&historyPage*20>=v.total){setHistoryPage(Math.max(0,Math.ceil(v.total/20)-1));return}historyStamp.current={version:v.list_version,revision,filterQuery};setHistory(v.items);setHistoryTotal(v.total)}}).catch(e=>{if(alive){notify(e.message,true);if(e.message==='This list changed. Start from the first page.'){setHistoryPage(0);setRetry(v=>v+1)}else setHistoryError(e.message)}}).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[revision,historyPage,retry,filterQuery])
 const processPreviews=async(reports:Row[],openReview:boolean)=>{
  const remaining:Row[]=[],added:number[]=[]
  for(const report of reports){
   processed.current.add(report.id)
   if(report.error||report.error_count||report.candidate_count){remaining.push(report);continue}
   if(report.already_imported){if(report.inserted>0)added.push(report.id);continue}
   if(!report.total)continue
   try{
    await api('/imports/'+report.id+'/commit','POST',{confirm_valid_rows:false,decisions:{},skip_all_candidates:false,classification_version:report.classification_version,...(report.preview_version?{preview_version:report.preview_version}:{})})
    added.push(report.id)
   }catch(e){remaining.push(report);notify((e as Error).message,true)}
  }
  setPreviews(previous=>openReview?remaining:[...previous.filter(p=>!reports.some(report=>report.id===p.id)),...remaining]);refresh()
  if(added.length)notify('Transactions imported. Categorized entries are accepted and unseen.'+(remaining.length?' Some imports need your attention.':''))
  if(openReview&&added.length&&!remaining.length)await continueImported(added)
 }
 const preview=()=>run(async()=>{const body=new FormData();body.set('account_id',account);files.forEach(f=>body.append('files',f));const reports=await api('/imports/preview?paged=1','POST',body);await processPreviews(reports,true)})
 const fetchLive=()=>run(async()=>{const reports=await api('/fnb/transactions?paged=1','POST');await processPreviews(reports,true)})
 const resume=async(h:Row)=>{await run(async()=>{const report=await api('/imports/'+h.id+'?page=0&page_size=50');setAccount(String(h.account_id));await processPreviews([report],true)})}
 // Older staged imports follow the same rules when this workspace is opened.
 // Attempt each ID once per visit; conflicts remain available for explicit decisions.
 useEffect(()=>{
  if(loading||busy||processing)return
  const pending=(filterQuery?autoHistory:history).filter(h=>h.status==='staged'&&!processed.current.has(h.id))
  if(!pending.length)return
  pending.forEach(h=>processed.current.add(h.id));setProcessing(true)
  ;(async()=>{
   const reports:Row[]=[]
   for(const h of pending){try{reports.push(await api('/imports/'+h.id+'?page=0&page_size=50'))}catch(e){notify((e as Error).message,true)}}
   if(reports.length)await processPreviews(reports,false)
  })().finally(()=>setProcessing(false))
 },[history,autoHistory,filterQuery,loading,busy,processing])
 return <>
  {processing&&<Loading>Processing pending imports</Loading>}
  <section className="panel import-start"><div className="section-head"><div><h2>Import transactions</h2><p className="muted">Import → Apply rules → Categorize what is missing</p></div></div>
  {data.user.admin&&<div className="toolbar"><div><strong>FNB connection</strong><p className="muted">{connectionLoading?'Checking connection':connectionError?'Connection status unavailable':connection?.connection?'Get recent completed transactions for your connected accounts.':'Connect FNB to get transactions.'}</p></div><div className="toolbar-actions"><Button variant="primary" loading={busy} disabled={processing||adding||connectionLoading||!connection?.connection||connection.connection.state==='refreshing'} onClick={fetchLive}>Get transactions</Button><Button onClick={onBanking}>{connection?.connection?'Connection settings':'Connect FNB'}</Button></div></div>}
  {connectionError&&<p role="alert">{connectionError}</p>}
  <p className="footnote">Categorized transactions are accepted automatically and start unseen. Only transactions with missing categories appear in Needs review. Mark transactions seen as you check them. Repeat FNB transactions are skipped automatically. Invalid exports or conflicting bank IDs stay in Import activity. FNB provides a limited history, so older transactions may be missing.</p></section>
  <details className="panel upload-panel"><summary>Upload a bank export</summary><div className="upload-content"><div className="section-head"><div><h2>Upload FNB exports</h2><p className="muted">Choose an account, then select CSV, OFX, or ZIP files.</p></div><Upload size={22}/></div>
   {editors.length?<><PagedSelect url="/accounts?role=editor" label="Import into account" hint="The bank account number in each file must match." value={account} options={editors} onChange={value=>{setAccount(value);setPreviews([])}} revision={revision}/>
   <input ref={input} type="file" hidden aria-label="FNB export files" accept=".csv,.ofx,.zip" multiple disabled={busy} onChange={e=>selectFiles(Array.from(e.target.files||[]))}/><button type="button" className={'file-picker'+(dragging?' dragging':'')} disabled={busy} aria-label="Choose FNB export files" onClick={openPicker} onKeyDown={e=>{if(e.key===' '){e.preventDefault();openPicker()}}} onDragEnter={e=>{e.preventDefault();if(!busy&&e.dataTransfer.types.includes('Files')){dragDepth.current++;setDragging(true)}}} onDragOver={e=>{e.preventDefault();e.dataTransfer.dropEffect=busy?'none':'copy'}} onDragLeave={e=>{e.preventDefault();dragDepth.current=Math.max(0,dragDepth.current-1);if(!dragDepth.current)setDragging(false)}} onDrop={e=>{e.preventDefault();dragDepth.current=0;setDragging(false);selectFiles(Array.from(e.dataTransfer.files))}}><FileText size={24} aria-hidden="true"/><span aria-live="polite">{files.length?files.map(f=>f.name).join(', '):'Click to select files or drag and drop'}</span><small>{files.length?'Click to change files · ':''}CSV, OFX or ZIP · Up to 20 files · 25 MiB total · OFX recommended for transaction IDs</small></button><Button variant="primary" loading={busy} disabled={processing||busy||!files.length||!account} onClick={preview}>{busy?'Importing statements':'Import statement'}</Button></>:<Empty title="Add an account first">You need editor access to an account before uploading.</Empty>}
  </div></details>
  {previews.length>0&&<div className="toolbar"><div><h2>Needs attention</h2><p className="muted">Check possible duplicates or invalid data below. Other transactions are imported automatically; only missing categories need review.</p></div><Button variant="primary" loading={adding} disabled={busy||!Object.keys(readyImports).length} onClick={addReady}>{adding?'Adding transactions':readyCount?'Send '+readyCount+' '+(readyCount===1?'account':'accounts')+' to review':'Resolve issues below'}</Button></div>}
  {previews.map((p,index)=><ImportPreview initialOpen={index===0} key={p.id} preview={p} accountName={p.account_name||history.find(h=>h.id===p.id)?.account_name||data.accounts.find(a=>a.bank_id===p.bank_id)?.name||'Account ending '+String(p.bank_id).slice(-4)} register={register} disabled={adding} onReview={()=>{void continueImported([p.id])}} categories={data.categories} groups={data.spendingGroups} notify={notify} refresh={refresh} onDone={()=>{setPreviews(v=>v.filter(item=>item.id!==p.id));setFiles([]);if(input.current)input.current.value=''}}/>)}
  <section className="panel"><div className="section-head"><h2>Import activity</h2></div>{loading&&<Loading>Loading import activity</Loading>}{historyError&&<p role="alert">{historyError} <Button onClick={()=>setRetry(v=>v+1)}>Retry</Button></p>}{!loading&&!historyError&&!history.length?<Empty title={filterQuery?"No matching import activity":"No import activity yet"}>{filterQuery?"Try another category, spending group or description.":"Get transactions or upload an export to begin."}{filterQuery&&<Button onClick={onClearFilters}>Clear filters</Button>}</Empty>:[...new Map(history.map(h=>[h.run_id||'file:'+h.id,true])).keys()].map(key=>{
   const rows=history.filter(h=>(h.run_id||'file:'+h.id)===key),pending=rows[0].pending_accounts??rows.filter(h=>h.status==='staged').length
   return <ActivityRun key={key} summary={<><span>{rows[0].format==='fnb-live'?'Transactions from FNB':rows[0].name}<small>{new Intl.DateTimeFormat('en-ZA',{timeZone:'Africa/Johannesburg',dateStyle:'medium',timeStyle:'short'}).format(new Date(rows[0].created_at.replace(' ','T').replace(/Z$/,'')+'Z'))} · {rows[0].account_total||rows.length} {(rows[0].account_total||rows.length)===1?'account':'accounts'} · {importedCountLabel(rows[0].new_transactions_total??rows.reduce((total,h)=>total+(h.status==='committed'?(h.inserted||0):0),0))} · {rows[0].already_present_total||0} already present · {rows[0].needs_categories_total||0} need categories</small></span><Badge tone={pending?'pending':'good'}>{pending?pending+' need attention':'Added to transactions'}</Badge></>}><ActivityAccounts rows={rows} activityKey={String(key)} filterQuery={filterQuery} revision={revision} busy={busy||adding} resume={resume} onReview={onReview} onTransactions={onTransactions}/></ActivityRun>
  })}<Pagination page={historyPage} total={historyTotal} loading={loading} onChange={setHistoryPage}/></section>

 </>
}
function importedCountLabel(count:number){return count+' new '+(count===1?'transaction':'transactions')+' imported'}
function ActivityRun({summary,children}:{summary:import('react').ReactNode;children:import('react').ReactNode}){
 const[open,setOpen]=useState(false)
 return <details className="details activity-run" onToggle={e=>setOpen(e.currentTarget.open)}><summary>{summary}</summary>{open&&children}</details>
}
function ActivityAccounts({rows,activityKey,filterQuery,revision,busy,resume,onReview,onTransactions}:{rows:Row[];activityKey:string;filterQuery:string;revision:number;busy:boolean;resume:(h:Row)=>Promise<void>;onReview:(ids:number[])=>void;onTransactions:(ids:number[])=>void}){
 const list=usePagedList('/imports?activity='+encodeURIComponent(activityKey)+(filterQuery?'&'+filterQuery:''),revision)
 useEffect(()=>list.setPage(0),[activityKey,filterQuery])
 const shown=list.loading&&!list.items.length?rows:list.items
 return <><ListStatus list={list}/>{shown.map(h=><div className="activity-account" key={h.id}><div><strong>{h.account_name}</strong><small>FNB · ••{h.account_ending} · {h.status==='committed'?importedCountLabel(h.inserted||0)+' · '+(h.skipped||0)+' already present · '+(h.needs_categories||0)+' need categories':'Not imported yet · '+h.row_count+' source rows'}{h.error_count?' · '+h.error_count+' rejected':''}</small>{h.format==='fnb-live'&&<small>Older transactions may be missing.</small>}{h.error&&<p className="error-text">{h.error}</p>}</div><div className="toolbar-actions">{h.status==='staged'?<Button disabled={busy} onClick={()=>resume(h)}>Resolve import issues</Button>:<><Button onClick={()=>onReview([h.id])}>Review transactions</Button><Button onClick={()=>onTransactions([h.id])}>View transactions</Button></>}</div>{h.status==='committed'&&<RetainedRows id={h.id}/>}</div>)}<ListNavigation list={list}/></>
}

function ImportPreview({preview:initial,initialOpen,accountName,register,disabled,onReview,categories,groups,notify,refresh,onDone}:{initialOpen:boolean;accountName:string;register:(id:number,action:(()=>Promise<boolean>)|null)=>void;disabled:boolean;onReview:()=>void;preview:Row;categories:Row[];groups:Row[];notify:PageProps['notify'];refresh:()=>void;onDone:()=>void}){
 const {openTransaction}=useTransactionAccess()
 const[p,setPreview]=useState(initial),[loading,setLoading]=useState(false),[pageError,setPageError]=useState(''),[skipAll,setSkipAll]=useState(false),[stale,setStale]=useState(false)
 const[decisions,setDecisions]=useState<Record<string,string>>({}),[confirm,setConfirm]=useState(false),[page,setPage]=useState(0)
 const{busy,run}=useTask(notify)
 const loadPage=async(next:number)=>{setLoading(true);setPageError('');try{const result=await api('/imports/'+p.id+'?page='+next+'&page_size=50');if(result.preview_version!==p.preview_version||result.classification_version!==p.classification_version){setDecisions({});setSkipAll(false);setConfirm(false);notify('Rules or possible duplicates have changed. Check these transactions again.',true)}setPreview(result);setPage(next);setStale(false)}catch(e){setPageError((e as Error).message)}finally{setLoading(false)}}
 const bad=p.error_count
 const unresolved=!skipAll&&Object.values(decisions).filter(v=>v==='keep'||v==='skip').length<p.candidate_count
 const blocked=busy||stale||loading||!!pageError||!p.total||!!p.error||p.already_imported||unresolved||(!!bad&&!confirm)
 const commit=async()=>{const ok=await run(async()=>{const v=await api('/imports/'+p.id+'/commit','POST',{confirm_valid_rows:confirm,decisions,skip_all_candidates:skipAll,classification_version:p.classification_version,...(p.preview_version?{preview_version:p.preview_version}:{})});notify(v.inserted+' transactions added; '+v.skipped+' skipped. Categorized transactions are accepted and unseen.')},undefined,message=>{if(message.includes('preview changed')||message.includes('Classification rules changed'))setStale(true);return false});if(ok){refresh();onDone()}return ok}
 useEffect(()=>{register(p.id,blocked?null:commit);return()=>register(p.id,null)},[p,decisions,skipAll,confirm,blocked])
 return <section className="panel preview"><details className="preview-account" open={initialOpen}><summary><span><strong>{accountName}</strong><small>{p.total} {p.total===1?'transaction':'transactions'} · {p.candidate_count} duplicate {p.candidate_count===1?'check':'checks'}{bad?' · '+bad+' rejected':''}</small></span><Badge tone={blocked?'pending':'good'}>{p.already_imported?'Already added':!p.total?'No new transactions':blocked?'Needs attention':'Ready to add'}</Badge></summary><div className="preview-content"><div className="section-head"><div><h2>{accountName}</h2><p className="muted">{p.format==='fnb-live'?'Transactions from FNB':p.name} · {p.currency} · account ending {String(p.bank_id).slice(-4)} · {p.total} transactions</p></div><Badge tone="pending">Needs attention</Badge></div>
 {p.coverage&&<p className="footnote">Transactions received: {p.coverage.returned_start&&p.coverage.returned_end?`${p.coverage.returned_start} to ${p.coverage.returned_end}`:'No completed transactions received'} · {p.coverage.returned_rows} bank transactions{p.coverage.service_fee_rows?` · ${p.coverage.service_fee_rows} separate bank fees`:''}. History may be incomplete.{p.coverage.pending_rows?` ${p.coverage.pending_rows} pending bank payments excluded.`:''}{p.coverage.page_limit_reached?' FNB returned its maximum of 150 transactions.':''}</p>}
 {p.error&&<div className="notice error" role="alert">{p.error}</div>}{p.already_imported&&<div className="notice">These transactions have already been imported. They will not be added again.</div>}
 {!!p.balance_date&&<p className="footnote">Bank-reported balance: {money(p.balance_cents)} as of {p.balance_date}.</p>}
 {p.candidate_count>0&&<div className="notice"><span>Check possible duplicates. Similar purchases may be separate transactions.</span><Button disabled={loading} onClick={()=>{setSkipAll(true);setDecisions({})}}>Skip all {p.candidate_count} possible duplicates in this import</Button></div>}
 <fieldset disabled={disabled||busy} className="import-decisions"><div className="preview-rows" aria-busy={loading}>{p.rows.map((r:Row)=><div className="preview-row" key={r.row}><div className="line"><strong>{r.description||'Invalid row'}</strong><strong>{money(r.amount_cents)}</strong></div><small>Row {r.row} · {r.date||'Invalid date'}</small>{r.source_component==='service_fee'&&<p className="footnote">Bank row {r.source_bank_row} service fee · {r.source_bank_description}</p>}{r.error?<p className="error-text">{r.error}</p>:<><div className="row-meta"><span>{categories.find(c=>c.id===r.category_id)?.name||'Uncategorized'}</span>{r.spending_group_id&&<span>{groups.find(g=>g.id===r.spending_group_id)?.name}</span>}{r.suggestion&&<small>{r.suggestion}</small>}{r.rule_conflict&&<Badge tone="pending">Rules disagree — check category and group</Badge>}{(r.rule_match_count||r.rule_matches?.length)>1&&<RuleSuggestions id={p.id} row={r.row} version={p.preview_version} onStale={()=>setStale(true)}/>}{r.duplicate&&<Badge tone={r.duplicate==='conflict'?'bad':'pending'}>{r.duplicate==='exact_source'?'Already imported — skip':r.duplicate==='exact_id'?'Existing transaction ID — skip':r.duplicate==='conflict'?'Conflicting transaction ID':'Possible duplicate'}</Badge>}</div>{r.candidates?.length>0&&<details><summary>Compare possible duplicates</summary>{r.candidates.map((c:Row,i:number)=><div className="candidate" key={i}><small>{c.id?'Existing transaction #'+c.id:c.file?'Batch file '+c.file:'Earlier row '+c.row}</small><div>{c.id?<button type="button" className="transaction-link" onClick={()=>openTransaction(c.id)}>{c.date} · {c.description} · {money(c.amount_cents)}</button>:<span>{c.date} · {c.description} · {money(c.amount_cents)}</span>}</div></div>)}</details>}{['possible','conflict'].includes(r.duplicate)&&<Field label={'Decision for row '+r.row}><select value={decisions[r.row]||(skipAll?'skip':'')} onChange={e=>setDecisions({...decisions,[r.row]:e.target.value})}><option value="">Choose what to do</option><option value="skip">Skip this row</option>{r.duplicate==='possible'&&<option value="keep">Keep as a separate transaction</option>}</select></Field>}</>}</div>)}</div>
 {stale&&<Button disabled={loading} onClick={()=>loadPage(page)}>Reload preview</Button>}
 {loading&&<Loading>Loading import rows</Loading>}{pageError&&<p role="alert">{pageError} <Button onClick={()=>loadPage(page)}>Retry</Button></p>}
 <Pagination page={page} total={p.total} size={50} loading={loading||busy} onChange={loadPage}/>
 {!!bad&&<label className="check"><input type="checkbox" checked={confirm} onChange={e=>setConfirm(e.target.checked)}/>Import valid rows and retain {bad} rejected rows in history</label>}
 </fieldset><div className="editor-actions"><Button variant="primary" loading={busy} disabled={disabled||blocked} onClick={async()=>{if(await commit())onReview()}}>Continue to review</Button><span className="muted">Categorized transactions are accepted automatically; only missing categories need review.</span></div></div></details>
 </section>
}

function RetainedPage({id}:{id:number}){
 const {openTransaction}=useTransactionAccess()
 const[page,setPage]=useState(0),[data,setData]=useState<Row|null>(null),[loading,setLoading]=useState(true),[error,setError]=useState(''),[retry,setRetry]=useState(0)
 useEffect(()=>{let alive=true;setLoading(true);setError('');api('/imports/'+id+'?page='+page+'&page_size=50').then(v=>alive&&setData(v)).catch(e=>alive&&setError(e.message)).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[id,page,retry])
 return <>{loading&&<Loading>Loading import details</Loading>}{error&&<p role="alert">{error} <Button onClick={()=>setRetry(v=>v+1)}>Retry</Button></p>}{data?.rows.map((row:Row)=><div className="candidate" key={row.row}><small>Row {row.row} · {row.date}</small><div>{(row.transaction_id||row.candidates?.length===1&&row.candidates[0].id)?<button type="button" className="transaction-link" onClick={()=>openTransaction(row.transaction_id||row.candidates[0].id)}>{row.description} · {money(row.amount_cents)}</button>:<span>{row.description} · {money(row.amount_cents)}</span>}</div><small>{row.error?'Rejected: '+row.error:data.decisions?.[row.row]==='skip'||['exact_id','exact_source'].includes(row.duplicate)?'Skipped':'Imported'}</small></div>)}<Pagination page={page} total={data?.total||0} size={50} loading={loading} onChange={setPage}/></>
}

function RetainedRows({id}:{id:number}){
const[open,setOpen]=useState(false);return <details onToggle={e=>setOpen(e.currentTarget.open)}><summary>View imported, skipped, and rejected transactions</summary>{open&&<RetainedPage id={id}/>}</details>}

function RuleSuggestions({id,row,version,onStale}:{id:number;row:number;version?:string;onStale:()=>void}){const[open,setOpen]=useState(false);return <details onToggle={e=>setOpen(e.currentTarget.open)}><summary>Compare rule suggestions</summary>{open&&<SuggestionPage url={'/imports/'+id+'/rows/'+row+'/suggestions'+(version?'?preview_version='+version:'')} onStale={onStale}/>}</details>}
function SuggestionPage({url,onStale}:{url:string;onStale:()=>void}){const list=usePagedList(url);useEffect(()=>{if(list.error.includes('Import preview changed'))onStale()},[list.error]);return <><ListStatus list={list}/>{list.items.map(m=><div className="candidate" key={m.id}>{m.pattern} → {m.category_name}{m.spending_group_name?' · '+m.spending_group_name:''}</div>)}<ListNavigation list={list}/></>}
