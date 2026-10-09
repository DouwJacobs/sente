import {PagedSelect} from './PagedList'
import {useEffect,useState,useRef} from 'react'
import {Plus,Pause,Play,CircleCheck,Globe} from 'lucide-react'
import {api} from './api'
import {Button,Field,Form,StatusIcon,ActionMenu,Empty,Modal,validateFields,Pagination,Loading} from './ui'
import {CreateCategory} from './Choices'
import {useTask} from './shared/useTask'
import {type PageProps,type Row} from './shared/types'

function groupedRules(rules:Row[]){
 const groups=new Map<string,Row[]>()
 for(const rule of rules){
  const key=JSON.stringify([rule.normalized_pattern||rule.pattern.trim().toLowerCase().replace(/\s+/g,' '),rule.category_id,rule.spending_group_id,rule.direction,rule.priority,rule.enabled,!!rule.builtin])
  groups.set(key,[...(groups.get(key)||[]),rule])
 }
 return [...groups.entries()].map(([key,rows])=>({key,rows,rule:rows[0]}))
}
const definition=(r:Row,account:number,enabled=!!r.enabled)=>({account_id:account,pattern:r.pattern,category_id:Number(r.category_id),spending_group_id:r.spending_group_id?Number(r.spending_group_id):null,direction:r.direction,priority:Number(r.priority),enabled,version:r.version||0})
export function RuleEditor({rule,data,revision,notify,refresh,onClose,onDone}:{rule?:Row;data:PageProps['data'];revision?:number;notify:PageProps['notify'];refresh:()=>void;onClose:()=>void;onDone?:()=>void}){
 const{busy,run}=useTask(notify)
 const rows=rule?.rows||(rule?.id!==undefined?[rule]:undefined)
 const[editing,setEditing]=useState<Row>(()=>rows?{...rows[0],enabled:!!rows[0].enabled,rows}:{pattern:'',category_id:'',spending_group_id:null,direction:'any',priority:0,enabled:true})
 const[scope,setScope]=useState('all'),[test,setTest]=useState(''),[testDirection,setTestDirection]=useState(editing.direction==='credit'?'credit':'debit'),[preview,setPreview]=useState<Row|null>(null),[creatingCategory,setCreatingCategory]=useState(false)
 const[extraEditor,setExtraEditor]=useState<Row|null>(null)
 const editors=[...data.accounts,...(extraEditor&&!data.accounts.some(a=>a.id===extraEditor.id)?[extraEditor]:[])].filter(a=>a.role==='editor')
 const change=(key:string,value:unknown)=>{setEditing(e=>({...e,[key]:value}));setPreview(null)}
 const targets:Row[]=editing.builtin?editors.map(a=>({account_id:a.id,id:editing.id,version:editing.version})):editing.rows||editors.filter(a=>scope==='all'||String(a.id)===scope).map(a=>({account_id:a.id}))
 const scopeText=(r:Row[])=>{
  if(r[0]?.builtin)return 'All enabled accounts'
  const accounts=[...new Map(r.map(x=>[x.account_id,x.account_name])).entries()]
  return accounts.length===editors.length&&accounts.every(([id])=>editors.some(a=>a.id===id))?'All current accounts you can edit':accounts.length===1?accounts[0][1]:accounts.length+' accounts'
 }
 const save=async()=>{
  if(!editing||(!editing.builtin&&!targets.length))return
  if(await run(()=>api('/rules/batch','POST',{all_current:!editing.rows&&!editing.builtin&&scope==='all',rules:!editing.rows&&!editing.builtin&&scope==='all'?[{id:0,rule:definition(editing,0)}]:editing.builtin?[{id:editing.id,rule:definition(editing,0)}]:targets.map(r=>({id:r.id||0,rule:{...definition(editing,Number(r.account_id)),version:r.version||0}}))}),'Rule saved')){
   refresh();onDone?onDone():onClose()
  }
 }
 return <>
  <Modal title={editing.rows?'Edit rule':'Add rule'} onClose={onClose}><Form onSubmit={save}>
  <Field label="Description contains" hint="For example, a shop or service provider name." validate={value=>value.trim().length<2?'Use at least 2 non-space characters.':new TextEncoder().encode(value.trim()).length>200?'Use a shorter description match.':''}><input autoFocus required minLength={2} maxLength={200} value={editing.pattern} onChange={e=>change('pattern',e.target.value)}/></Field>
  <PagedSelect url="/categories?active=1" label="Category" required value={editing.category_id} options={data.categories.filter(c=>!c.archived||c.id===Number(editing.category_id))} onChange={value=>change('category_id',value)} revision={revision}/>
  {data.user.budget_member&&<Button type="button" variant="quiet" loading={busy} disabled={busy} onClick={()=>setCreatingCategory(true)}><Plus size={16}/>Create category</Button>}
  <PagedSelect url="/spending-groups" label="Spending group" empty="Leave unassigned" value={editing.spending_group_id||''} options={data.spendingGroups} onChange={value=>change('spending_group_id',value||null)} revision={revision}/>
  <details className="details"><summary>More options</summary>
  {editing.rows?<p className="muted">Applies to {scopeText(editing.rows)}.</p>:<PagedSelect url="/accounts?role=editor" label="Applies to" empty="Choose accounts" value={scope} options={[{id:'all',name:'All current accounts you can edit'},...editors]} onChange={value=>{setScope(value);setPreview(null)}} onSelectRow={row=>setExtraEditor(row?.id==='all'?null:row||null)} revision={revision}/>}
  <Field label="Payment direction"><select value={editing.direction} onChange={e=>{change('direction',e.target.value);setTestDirection(e.target.value==='credit'?'credit':'debit')}}><option value="any">Money in or out</option><option value="debit">Money out</option><option value="credit">Money in</option></select></Field>
  <Field label="Priority"><input type="number" required min={-1000000} max={1000000} step={1} value={editing.priority} onChange={e=>change('priority',e.target.value)}/></Field>
  </details>
  <details className="details"><summary>Check an example</summary>
  <Field label="Example description"><input maxLength={1000} value={test} onChange={e=>{setTest(e.target.value);setPreview(null)}}/></Field>
  {editing.direction==='any'&&<Field label="Example direction"><select value={testDirection} onChange={e=>{setTestDirection(e.target.value);setPreview(null)}}><option value="debit">Money out</option><option value="credit">Money in</option></select></Field>}
  <Button type="button" loading={busy} disabled={busy||!test.trim()||!targets.length} onClick={e=>{
   if(e.currentTarget.form&&!validateFields(e.currentTarget.form))return
   run(async()=>{
    if((!editing.rows&&scope==='all')||editing.builtin){setPreview(await api('/rules/preview','POST',{all_current:true,description:test,direction:testDirection,draft:definition(editing,0),replace_id:editing.id||0}));return}
    const results=await Promise.all(targets.map(target=>api('/rules/preview','POST',{account_id:Number(target.account_id),description:test,direction:testDirection,draft:definition(editing,Number(target.account_id)),replace_id:target.id||0})))
    const draftMatches=(result:Row,index:number)=>result.rule_matches?.some((match:Row)=>match.id===(targets[index].id||0))
    const conflict=results.some((result,index)=>result.rule_conflict&&draftMatches(result,index)),matches=results.filter(draftMatches)
    setPreview({conflict,matches:matches.length,accounts:results.length})
   })
  }}>Check rule</Button>
  {preview&&<p role="status">{preview.conflict?'Another matching rule suggests a different category or spending group. Check both rules.':preview.matches?'Matches '+preview.matches+' of '+preview.accounts+' accounts: '+(data.categories.find(c=>c.id===Number(editing.category_id))?.name||'selected category')+(editing.spending_group_id?' · '+data.spendingGroups.find(g=>g.id===Number(editing.spending_group_id))?.name:''):'This example does not match.'}</p>}
  </details>
  <p className="muted">{editing.builtin?'Applies to all enabled accounts, including accounts added later.':editing.rows?'Changes apply to this rule’s current accounts.':'Applies to '+(scope==='all'?'all current accounts you can edit.':editors.find(a=>String(a.id)===scope)?.name+'.')}</p>
  <Button type="submit" variant="primary" loading={busy} disabled={busy||(!editing.builtin&&!targets.length)}>Save rule</Button>
  </Form></Modal>
  {creatingCategory&&<Modal title="Create category" onClose={()=>setCreatingCategory(false)}><CreateCategory name="" notify={notify} done={id=>{change('category_id',id);setCreatingCategory(false);refresh()}}/></Modal>}
 </>
}
export function Rules({data,refresh,notify,revision,onAccounts,onCreateCategory}:Pick<PageProps,'data'|'refresh'|'notify'|'revision'>&{onAccounts:()=>void;onCreateCategory:()=>void}){
 const[editing,setEditing]=useState<Row|null>(null)
 const stamp=useRef({version:'',revision})
 const[page,setPage]=useState(0),[total,setTotal]=useState(0),[rules,setRules]=useState<Row[]>([]),[loading,setLoading]=useState(true),[loadError,setLoadError]=useState(''),[retry,setRetry]=useState(0)
 useEffect(()=>{let alive=true;setLoading(true);setLoadError('');api('/rules?page='+page+(page>0&&stamp.current.revision===revision?'&list_version='+stamp.current.version:'')).then(v=>{if(alive){if(page>0&&page*20>=v.total){setPage(Math.max(0,Math.ceil(v.total/20)-1));return}stamp.current={version:v.list_version,revision};setRules(v.items);setTotal(v.total)}}).catch(e=>{if(alive){notify(e.message,true);if(e.message==='This list changed. Start from the first page.'){stamp.current.version='';setPage(0);setRetry(v=>v+1)}else setLoadError(e.message)}}).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[page,revision,retry])
 const{busy,run}=useTask(notify)
 const editors=data.accounts.filter(a=>a.role==='editor'),groups=groupedRules(rules)
 const open=(rows?:Row[])=>setEditing(rows?{...rows[0],enabled:!!rows[0].enabled,rows}:{pattern:'',category_id:'',spending_group_id:null,direction:'any',priority:0,enabled:true})
 const scopeText=(rows:Row[])=>{
  if(rows[0]?.builtin)return 'All enabled accounts'
  const accounts=[...new Map(rows.map(r=>[r.account_id,r.account_name])).entries()]
  return accounts.length===editors.length&&accounts.every(([id])=>editors.some(a=>a.id===id))?'All current accounts you can edit':accounts.length===1?accounts[0][1]:accounts.length+' accounts'
 }
 return <section className="panel">
 <div className="section-head"><div><h2>Rules</h2><p className="muted">Create your own rules or import a configuration in Settings. Account rules take precedence over global fallback rules.</p></div><Button disabled={!editors.length||!data.categories.length} onClick={()=>open()}><Plus size={16}/>Add rule</Button></div>
 {!editors.length?<div className="rule-prerequisite"><p className="muted">{data.user.admin?'Add or discover an account before creating account rules.':'You need editor access to an account to create account rules.'}</p><Button onClick={onAccounts}>Open Accounts</Button></div>:!data.categories.length?<div className="rule-prerequisite"><p className="muted">{data.user.budget_member?'Create a category before adding an automatic rule.':'Ask a budget member to create a category before adding a rule.'}</p>{data.user.budget_member&&<Button onClick={onCreateCategory}>Create category</Button>}</div>:null}
 {loading&&<Loading>Loading rules</Loading>}{loadError&&<p role="alert">{loadError} <Button onClick={()=>setRetry(v=>v+1)}>Retry</Button></p>}
 {!rules.length&&!loading&&!loadError?<Empty kind="categories" title="No rules yet"><p>{editors.length&&data.categories.length?'Match a bank description to a category so future imports need less manual review.':'Rules need an account you can edit and a category to match.'}</p>{editors.length&&data.categories.length?<Button variant="primary" onClick={()=>open()}>Create first rule</Button>:!editors.length?<Button variant="primary" onClick={onAccounts}>Open Accounts</Button>:data.user.budget_member?<Button variant="primary" onClick={onCreateCategory}>Create category</Button>:null}</Empty>:groups.map(({key,rows,rule:r})=>{
  const editable=r.builtin?data.user.budget_member:rows.every(rule=>editors.some(a=>a.id===rule.account_id))
  return (
   <div className="rule-row rule-list-row" key={key}>
    <div className="rule-list-content">
     <strong title={'Description contains “'+r.pattern+'”'}>{r.pattern}</strong>
     <small>{r.category_name}{r.spending_group_name?' · '+r.spending_group_name:''}</small>
     <small title={rows.map(row=>row.account_name).join(', ')}>
      {scopeText(rows)}{r.direction!=='any'?' · '+(r.direction==='debit'?'Money out':'Money in'):''}
     </small>
    </div>
    <div className="rule-list-actions">
     <StatusIcon label={r.enabled?'Active':'Paused'} icon={r.enabled?CircleCheck:Pause} tone={r.enabled?'good':'neutral'}/>
     {!!r.builtin&&<StatusIcon label="Global fallback" icon={Globe}/>}
     {editable&&(
      <ActionMenu label={'Actions for rule '+r.pattern}>
       <Button variant="quiet" aria-label={'Edit rule '+r.pattern} disabled={busy} onClick={()=>open(rows)}>Edit</Button>
       <Button variant="quiet" disabled={busy} onClick={async()=>{
        if(await run(()=>api('/rules/batch','POST',{rules:rows.map(row=>({id:row.id,rule:definition(row,Number(row.account_id),!r.enabled)}))}),r.enabled?'Rule paused':'Rule resumed'))refresh()
       }}>{r.enabled?<Pause size={16}/>:<Play size={16}/>} {r.enabled?'Pause':'Resume'}</Button>
       <Button variant="quiet" aria-label={'Delete rule '+r.pattern} disabled={busy} onClick={async()=>{
        if(await run(()=>api('/rules/batch','DELETE',{rules:rows.map(row=>({id:row.id,version:row.version}))}),'Rule deleted'))refresh()
       }}>Delete</Button>
      </ActionMenu>
     )}
    </div>
   </div>
  )
 })}
 <Pagination page={page} total={total} loading={loading} onChange={setPage}/>
 <p className="footnote">Rules suggest categories and spending groups for you to review.</p>
 {editing&&<RuleEditor rule={editing} data={data} revision={revision} notify={notify} refresh={refresh} onClose={()=>setEditing(null)}/>}
 </section>
}
