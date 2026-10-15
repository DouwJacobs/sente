import {MerchantRuleEditor} from './features/merchants/MerchantRuleEditor'
import {useEffect,useState} from 'react'
import {api} from './api'
import {CircleCheck,Pause} from 'lucide-react'
import {MerchantAvatar} from './MerchantAvatar'
import {Button,Field,Form,Modal,StatusIcon,Loading,Pagination,ActionMenu,Empty} from './ui'
import {GroupDot} from './Choices'
import {usePagedList,ListStatus,ListNavigation} from './PagedList'
import {BulkEditor} from './CoreWorkflows'
import {useTask} from './shared/useTask'
import {type PageProps,type Row} from './shared/types'
export function CategoryEditor({category,notify,onClose,onDone}:{category:Row;notify:PageProps['notify'];onClose:()=>void;onDone:()=>void}){
 const[name,setName]=useState(category.name),[archived,setArchived]=useState(!!category.archived),[deps,setDeps]=useState<Row|null>(null),[error,setError]=useState('')
 const{busy,run}=useTask(notify)
 useEffect(()=>{api('/categories/'+category.id+'/dependencies').then(setDeps).catch(e=>notify(e.message,true))},[category.id])
 const blocked=deps&&(deps.blocked||deps.active_rules>0||deps.carry_forward_periods.length>0)
 return <Modal title={'Edit category · '+category.name} onClose={onClose}><Form onSubmit={()=>run(async()=>{const body:Record<string,unknown>={name,archived,version:category.version};await api('/categories/'+category.id,'PUT',body);onDone()},'Category saved',message=>{if(message==='Category name already exists'){setError(message);return true}return false})}><Field label="Category name" serverError={error}><input autoFocus required maxLength={100} value={name} onChange={e=>{setName(e.target.value);setError('')}}/></Field><p className="footnote">Type: {category.kind}. Renaming keeps historical transactions and budgets connected.</p><label className="check"><input type="checkbox" checked={archived} disabled={!category.archived&&!!blocked} onChange={e=>setArchived(e.target.checked)}/>Archived</label>{!deps?<Loading>Checking dependencies</Loading>:blocked?<><p>Pause or replace active rules and stop upcoming carry-forward before archiving.</p><p>{deps.active_rules} active rules{deps.restricted_rules?' · Some dependencies belong to other accounts':''}</p>{deps.carry_forward_periods.map((p:Row)=><p key={p.id}>{p.name} · {p.entries} upcoming budget entries</p>)}</>:<p className="footnote">Archived categories keep historical budgets and transactions. They cannot be newly assigned. Restoring does not enable rules.</p>}<div className="editor-actions"><Button type="submit" variant="primary" loading={busy} disabled={!deps}>Save category</Button><Button onClick={onClose}>Cancel</Button></div></Form></Modal>
}
export function MerchantRules(props:PageProps){
 const{data,revision,notify,refresh}=props,list=usePagedList('/merchant-rules',revision),{busy,run}=useTask(notify)
 const[edit,setEdit]=useState<Row|null>(null),[preview,setPreview]=useState<Row|null>(null),[selection,setSelection]=useState<Record<number,Row>>({}),[bulk,setBulk]=useState(false),[page,setPage]=useState(0)
 const openEditor=(rule?:Row)=>setEdit(rule?{...rule,account_id:rule.account_id??0}:{account_id:data.user.budget_member?0:data.accounts.find(a=>a.role==='editor')?.id||'',merchant_id:null,pattern:'',direction:'any',priority:0,enabled:true})
 const loadPreview=(id:number,nextPage=0)=>run(async()=>{setPreview({...await api('/merchant-rules/'+id+'/preview?page='+nextPage,'POST'),rule_id:id});setPage(nextPage)})
 return <section className="panel">
  <div className="section-head">
   <div><h2>Merchant naming rules</h2><p className="muted">Give one merchant several description rules, each with its own category and spending group.</p></div>
   {(list.loading||list.error||list.items.length>0)&&<Button onClick={()=>openEditor()}>Add merchant rule</Button>}
  </div>
  <ListStatus list={list}/>
  {!list.loading&&!list.error&&!list.items.length&&<Empty kind="categories" title="Recognize your regular merchants"><p>Turn bank descriptions into familiar merchant names. Each matching rule can assign its own category for future imports.</p><Button variant="primary" onClick={()=>openEditor()}>Add merchant rule</Button></Empty>}
  {list.items.map(rule=>(
   <div className="line rule-list-row merchant-rule-row" key={rule.id}>
    <div className="merchant-identity">
     <MerchantAvatar name={rule.merchant_name} logo={rule.merchant_logo}/>
     <div className="rule-list-content">
      <strong>{rule.merchant_name}</strong>
      <small>{rule.pattern}</small>
      {(rule.category_name||rule.spending_group_name)&&<small>
       {rule.category_name}{rule.category_name&&rule.spending_group_name?' · ':''}
       {rule.spending_group_name&&<span className="group-label"><GroupDot color={rule.spending_group_color}/>{rule.spending_group_name}</span>}
      </small>}
     </div>
    </div>
    <div className="rule-list-actions">
     <StatusIcon label={rule.enabled?'Active':'Paused'} icon={rule.enabled?CircleCheck:Pause} tone={rule.enabled?'good':'neutral'}/>
     <ActionMenu label={'Actions for merchant rule '+rule.merchant_name}>
      <Button variant="quiet" disabled={busy} onClick={()=>openEditor(rule)}>Edit</Button>
      {rule.enabled&&<Button variant="quiet" disabled={busy} onClick={()=>{setSelection({});loadPreview(rule.id)}}>Preview unnamed transactions</Button>}
     </ActionMenu>
    </div>
   </div>
  ))}
  <ListNavigation list={list}/>

 {edit&&<MerchantRuleEditor rule={edit} data={data} notify={notify} refresh={refresh} onClose={()=>setEdit(null)}/>}
 {preview&&<Modal title="Matching unnamed transactions" size="wide" onClose={()=>setPreview(null)}><p>{preview.total} matches. Classification below describes future imports. This action assigns merchant names only and preserves existing categories and groups. Select up to 100 to apply explicitly.</p>{preview.items.map((t:Row)=><label key={t.id} className="line"><input type="checkbox" checked={!!selection[t.id]} disabled={!selection[t.id]&&Object.keys(selection).length>=100} onChange={e=>setSelection(old=>{const next={...old};if(e.target.checked)next[t.id]=t;else delete next[t.id];return next})}/><span>{t.date} · {t.description}<small>{t.classification}</small></span></label>)}<Pagination page={page} total={preview.total} size={100} loading={busy} onChange={p=>loadPreview(preview.rule_id,p)}/><Button disabled={!Object.keys(selection).length} onClick={()=>setBulk(true)}>Preview selected changes</Button></Modal>}
 {bulk&&preview&&<BulkEditor {...props} rows={Object.values(selection)} ruleId={preview.rule_id} onClose={()=>setBulk(false)} onDone={()=>{setBulk(false);setPreview(null);setSelection({});refresh()}}/>}
 </section>
}
