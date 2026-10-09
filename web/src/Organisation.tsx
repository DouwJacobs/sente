import {useEffect,useState} from 'react'
import {api} from './api'
import {CircleCheck,Pause} from 'lucide-react'
import {MerchantAvatar,MerchantLogoField} from './MerchantAvatar'
import {Button,Field,Form,Modal,StatusIcon,Loading,Pagination,ActionMenu,Empty} from './ui'
import {ChoiceField,CategoryChoice,GroupDot} from './Choices'
import {PagedSelect,usePagedList,ListStatus,ListNavigation} from './PagedList'
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
export function MerchantRuleEditor({rule,data,notify,refresh,onClose,onDone}:{rule?:Row;data:PageProps['data'];notify:PageProps['notify'];refresh:()=>void;onClose:()=>void;onDone?:()=>void}){
 const{busy,run}=useTask(notify)
 const[edit,setEdit]=useState<Row>(()=>rule?{...rule,account_id:rule.account_id??0}:{account_id:data.user.budget_member?0:data.accounts.find(a=>a.role==='editor')?.id||'',merchant_id:null,pattern:'',direction:'any',priority:0,enabled:true})
 const[merchantName,setMerchantName]=useState(''),[nameError,setNameError]=useState(''),[logo,setLogo]=useState(rule?.merchant_logo||''),[logoVersion,setLogoVersion]=useState(rule?.merchant_version||1),[logoDirty,setLogoDirty]=useState(false),[logoBusy,setLogoBusy]=useState(false)
 const[merchantCategory,setMerchantCategory]=useState<number|null>(rule?.category_id??null),[merchantGroup,setMerchantGroup]=useState<number|null>(rule?.spending_group_id??null),[categoryDirty,setCategoryDirty]=useState(false)
 const update=(key:string,value:unknown)=>setEdit(old=>({...old,[key]:value}))
 const validatePattern=(v:string)=>{const t=v.trim();if(new TextEncoder().encode(t).length<2)return 'Use at least 2 characters.';if(new TextEncoder().encode(t).length>200)return 'Use a shorter description match.';if(/[|*()+\[{^$\\]/.test(t)){try{new RegExp(t,'i')}catch{return 'Fix the regex syntax (e.g. unmatched brackets).'}}return ''}
 return <Modal title={edit.id?'Edit merchant rule':'New merchant rule'} onClose={onClose}><Form onSubmit={()=>run(async()=>{let merchant=edit.merchant_id,version=logoVersion;if(!merchant&&merchantName.trim()){const body:Record<string,unknown>={kind:'merchant',account_id:Number(edit.account_id),name:merchantName};if(categoryDirty){if(merchantCategory!==null)body.category_id=merchantCategory;if(merchantGroup!==null)body.spending_group_id=merchantGroup};const created=await api('/labels','POST',body);merchant=created.id;version=created.logo_version}if(!merchant){setNameError('Choose a merchant or enter a new merchant name.');return}const rulePayload:Record<string,unknown>={account_id:Number(edit.account_id),merchant_id:merchant,pattern:edit.pattern,direction:edit.direction,enabled:!!edit.enabled,version:edit.version||0,priority:Number(edit.priority)};if(logoDirty)Object.assign(rulePayload,{merchant_logo:logo,merchant_version:version});if(categoryDirty){if(merchantCategory!==null)rulePayload.category_id=merchantCategory;else rulePayload.clear_category=true;if(merchantGroup!==null)rulePayload.spending_group_id=merchantGroup;else rulePayload.clear_group=true};await api('/merchant-rules'+(edit.id?'/'+edit.id:''),edit.id?'PUT':'POST',rulePayload);refresh();onDone?onDone():onClose()},'Merchant rule saved')}><PagedSelect url="/accounts?role=editor" label="Scope" required value={edit.account_id} options={data.accounts.filter(a=>a.role==='editor')} specialOptions={data.user.budget_member?[{value:'0',label:'All accounts'}]:[]} hint="All accounts shares this rule across future imports." onChange={value=>{update('account_id',value);update('merchant_id',null);setLogo('');setLogoDirty(false);setMerchantCategory(null);setMerchantGroup(null);setCategoryDirty(false)}}/>{edit.account_id!==''&&<ChoiceField label="Merchant" onError={message=>notify(message,true)} options={[]} source={'/labels?kind=merchant&'+(Number(edit.account_id)===0?'scope=global':'account='+edit.account_id)} value={edit.merchant_id} onChange={async id=>{update('merchant_id',id);setNameError('');setLogo('');setLogoDirty(false);setMerchantCategory(null);setMerchantGroup(null);setCategoryDirty(false);if(id){try{const selected=(await api('/labels?kind=merchant&id='+id)).items[0];setLogo(selected?.logo_data||'');setLogoVersion(selected?.logo_version||1);setMerchantCategory(selected?.category_id??null);setMerchantGroup(selected?.spending_group_id??null)}catch(e){notify((e as Error).message,true)}}}}/>}<Field label="New merchant name" serverError={nameError} hint="Use this only if the merchant is not already listed."><input required={!edit.merchant_id} minLength={2} maxLength={100} value={merchantName} onChange={e=>{setMerchantName(e.target.value);setNameError('')}}/></Field><MerchantLogoField name={merchantName||edit.merchant_name||'Merchant'} value={logo} onChange={value=>{setLogo(value);setLogoDirty(true)}} onBusy={setLogoBusy}/><Field label="Description contains" hint="Plain text or regex. Use | for alternatives, e.g. checkers|shoprite." validate={validatePattern}><input required minLength={2} maxLength={200} value={edit.pattern} onChange={e=>update('pattern',e.target.value)}/></Field><CategoryChoice label="Default category" data={data} value={merchantCategory} onChange={id=>{setMerchantCategory(id);setCategoryDirty(true)}} refresh={refresh} notify={notify}/><ChoiceField label="Default spending group" source="/spending-groups" options={[]} value={merchantGroup} empty="No group" onChange={id=>{setMerchantGroup(id);setCategoryDirty(true)}} onError={message=>notify(message,true)}/><details className="details"><summary>More options{edit.direction!=='any'||Number(edit.priority)!==0||!edit.enabled?' · Custom settings':''}</summary><div className="form-grid"><Field label="Money direction"><select value={edit.direction} onChange={e=>update('direction',e.target.value)}><option value="any">Any direction</option><option value="debit">Money out</option><option value="credit">Money in</option></select></Field><Field label="Priority"><input type="number" min={-1000000} max={1000000} required value={edit.priority} onChange={e=>update('priority',e.target.value)}/></Field></div><label className="check"><input type="checkbox" checked={!!edit.enabled} onChange={e=>update('enabled',e.target.checked)}/>Active</label></details><p className="footnote">Highest priority wins. Equal-priority rules with different names leave the merchant unset. The default category is used during import when no other rule matches.</p><div className="editor-actions"><Button variant="primary" type="submit" loading={busy} disabled={logoBusy}>Save rule</Button><Button onClick={onClose}>Cancel</Button></div></Form></Modal>
}
export function MerchantRules(props:PageProps){
 const{data,revision,notify,refresh}=props,list=usePagedList('/merchant-rules',revision),{busy,run}=useTask(notify)
 const[edit,setEdit]=useState<Row|null>(null),[preview,setPreview]=useState<Row|null>(null),[selection,setSelection]=useState<Record<number,Row>>({}),[bulk,setBulk]=useState(false),[page,setPage]=useState(0)
 const openEditor=(rule?:Row)=>setEdit(rule?{...rule,account_id:rule.account_id??0}:{account_id:data.user.budget_member?0:data.accounts.find(a=>a.role==='editor')?.id||'',merchant_id:null,pattern:'',direction:'any',priority:0,enabled:true})
 const loadPreview=(id:number,nextPage=0)=>run(async()=>{setPreview({...await api('/merchant-rules/'+id+'/preview?page='+nextPage,'POST'),rule_id:id});setPage(nextPage)})
 return <section className="panel">
  <div className="section-head">
   <div><h2>Merchant naming rules</h2><p className="muted">Recognize merchants across accounts, with an optional account scope.</p></div>
   {(list.loading||list.error||list.items.length>0)&&<Button onClick={()=>openEditor()}>Add merchant rule</Button>}
  </div>
  <ListStatus list={list}/>
  {!list.loading&&!list.error&&!list.items.length&&<Empty kind="categories" title="Recognize your regular merchants"><p>Turn bank descriptions into familiar merchant names. You can also set a default category for future imports.</p><Button variant="primary" onClick={()=>openEditor()}>Add merchant rule</Button></Empty>}
  {list.items.map(rule=>(
   <div className="line rule-list-row merchant-rule-row" key={rule.id}>
    <div className="merchant-identity">
     <MerchantAvatar name={rule.merchant_name} logo={rule.merchant_logo}/>
     <div className="rule-list-content">
      <strong>{rule.merchant_name}</strong>
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
 {preview&&<Modal title="Matching unnamed transactions" size="wide" onClose={()=>setPreview(null)}><p>{preview.total} matches. Select up to 100 to apply explicitly.</p>{preview.items.map((t:Row)=><label key={t.id} className="line"><input type="checkbox" checked={!!selection[t.id]} disabled={!selection[t.id]&&Object.keys(selection).length>=100} onChange={e=>setSelection(old=>{const next={...old};if(e.target.checked)next[t.id]=t;else delete next[t.id];return next})}/><span>{t.date} · {t.description}</span></label>)}<Pagination page={page} total={preview.total} size={100} loading={busy} onChange={p=>loadPreview(preview.rule_id,p)}/><Button disabled={!Object.keys(selection).length} onClick={()=>setBulk(true)}>Preview selected changes</Button></Modal>}
 {bulk&&preview&&<BulkEditor {...props} rows={Object.values(selection)} ruleId={preview.rule_id} onClose={()=>setBulk(false)} onDone={()=>{setBulk(false);setPreview(null);setSelection({});refresh()}}/>}
 </section>
}
