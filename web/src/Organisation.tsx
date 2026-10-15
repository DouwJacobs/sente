import {useEffect,useState} from 'react'
import {api} from './api'
import {Button,Field,Form,Modal,Loading} from './ui'
import {useTask} from './shared/useTask'
import {type PageProps,type Row} from './shared/types'
export function CategoryEditor({category,notify,onClose,onDone}:{category:Row;notify:PageProps['notify'];onClose:()=>void;onDone:()=>void}){
 const[name,setName]=useState(category.name),[archived,setArchived]=useState(!!category.archived),[deps,setDeps]=useState<Row|null>(null),[error,setError]=useState('')
 const{busy,run}=useTask(notify)
 useEffect(()=>{api('/categories/'+category.id+'/dependencies').then(setDeps).catch(e=>notify(e.message,true))},[category.id])
 const blocked=deps&&(deps.blocked||deps.active_rules>0||deps.carry_forward_periods.length>0)
 return <Modal title={'Edit category · '+category.name} onClose={onClose}><Form onSubmit={()=>run(async()=>{const body:Record<string,unknown>={name,archived,version:category.version};await api('/categories/'+category.id,'PUT',body);onDone()},'Category saved',message=>{if(message==='Category name already exists'){setError(message);return true}return false})}><Field label="Category name" serverError={error}><input autoFocus required maxLength={100} value={name} onChange={e=>{setName(e.target.value);setError('')}}/></Field><p className="footnote">Type: {category.kind}. Renaming keeps historical transactions and budgets connected.</p><label className="check"><input type="checkbox" checked={archived} disabled={!category.archived&&!!blocked} onChange={e=>setArchived(e.target.checked)}/>Archived</label>{!deps?<Loading>Checking dependencies</Loading>:blocked?<><p>Pause or replace active rules and stop upcoming carry-forward before archiving.</p><p>{deps.active_rules} active rules{deps.restricted_rules?' · Some dependencies belong to other accounts':''}</p>{deps.carry_forward_periods.map((p:Row)=><p key={p.id}>{p.name} · {p.entries} upcoming budget entries</p>)}</>:<p className="footnote">Archived categories keep historical budgets and transactions. They cannot be newly assigned. Restoring does not enable rules.</p>}<div className="editor-actions"><Button type="submit" variant="primary" loading={busy} disabled={!deps}>Save category</Button><Button onClick={onClose}>Cancel</Button></div></Form></Modal>
}
export {RulesPage as MerchantRules} from "./features/rules/RulesPage";
