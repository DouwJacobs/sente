import {useRef,useState} from 'react'
import {Plus} from 'lucide-react'
import {api,cents,decimal,money} from './api'
import {Field,Form,Button,Empty,Modal,Badge,validateFields} from './ui'
import {moneyError} from './validation'
import {CreateCategory,GroupDot} from './Choices'
import {usePagedList,ListStatus,ListNavigation,PagedSelect} from './PagedList'
import {useTask,type PageProps,type Row} from './App'

type CategoryDraft={id:number;name:string;amount:string;carry:boolean;upcoming:boolean;baseAmount:number;remove?:boolean}
type GroupDraft={id:number;name:string;color?:string;target_cents?:number;added?:boolean;remove?:boolean;targets:Record<number,CategoryDraft>}

export function LimitEditor({period,notify,refresh,revision,onDone}:{period:Row;notify:PageProps['notify'];refresh:()=>void;revision:number;onDone:()=>void}){
 const list=usePagedList('/periods/'+period.id+'/budget-groups',revision),[drafts,setDrafts]=useState<Record<number,GroupDraft>>({})
 const[addingGroup,setAddingGroup]=useState(false),[groupId,setGroupId]=useState(''),[chosenGroup,setChosenGroup]=useState<Row|undefined>()
 const[addingCategory,setAddingCategory]=useState<Row|null>(null),[category,setCategory]=useState(''),[chosenCategory,setChosenCategory]=useState<Row|undefined>(),[amount,setAmount]=useState('0.00'),[future,setFuture]=useState(false),[creating,setCreating]=useState(false)
 const form=useRef<HTMLFormElement>(null),{busy,run}=useTask(notify)
 const groups=[...new Map([...list.items,...Object.values(drafts).filter(g=>g.added)].map(g=>[g.id,g])).values()].filter(g=>!drafts[g.id]?.remove)
 const edit=(g:Row,target?:CategoryDraft)=>setDrafts(old=>({...old,[g.id]:{...g,...old[g.id],targets:{...old[g.id]?.targets,...(target?{[target.id]:target}:{})}}}))
 const addCategory=(g:Row)=>{if(form.current&&!validateFields(form.current))return;setAddingCategory(g);setCategory('');setChosenCategory(undefined);setAmount('0.00');setFuture(false);setCreating(false)}
 return <>
  <p className="muted">Build this period's budget with the groups and categories you need.</p>
  <Button onClick={()=>{if(!form.current||validateFields(form.current)){setGroupId('');setChosenGroup(undefined);setAddingGroup(true)}}}><Plus size={16}/>Add group</Button>
  <ListStatus list={list}/>
  <Form ref={form} onSubmit={async()=>{const changes=Object.values(drafts);if(!changes.length){onDone();return}if(await run(()=>api('/periods/'+period.id+'/budget','PUT',{version:period.version,groups:changes.map(g=>({group_id:g.id,remove:!!g.remove,targets:g.remove?[]:Object.values(g.targets).map(c=>({category_id:c.id,amount_cents:c.remove?0:cents(c.amount),carry_forward:c.carry,apply_upcoming:c.upcoming,remove:!!c.remove}))}))}),'Budget saved')){refresh();onDone()}}}>
   {!list.loading&&!groups.length&&<Empty title="Start building your budget">Add a group, then choose its categories and amounts.</Empty>}
   {groups.map(g=><section className="budget-builder-group" key={g.id}><div className="section-head"><h3><GroupDot color={g.color}/>{g.name}</h3><strong className="builder-group-total">{money((g.target_cents||0)+Object.values(drafts[g.id]?.targets||{}).reduce((sum,c)=>sum+(c.remove?-c.baseAmount:moneyError(c.amount,true)?0:cents(c.amount)-c.baseAmount),0))}</strong><Button variant="quiet" onClick={()=>setDrafts(old=>({...old,[g.id]:{...g,...old[g.id],remove:true,targets:{}}}))}>Remove group</Button></div>
    <BudgetCategories period={period.id} group={g} draft={drafts[g.id]} revision={revision} onEdit={c=>edit(g,c)} validate={()=>!form.current||validateFields(form.current)}/>
    <Button onClick={()=>addCategory(g)}><Plus size={16}/>Add category to {g.name}</Button>
   </section>)}
   <ListNavigation list={{...list,setPage:page=>{if(!form.current||validateFields(form.current))list.setPage(page)}}}/>
   <p className="footnote">Save applies your changes together. To move an existing limit from No spending group, add it to its new group and remove the old entry. Transaction history stays intact.</p>
   <div className="editor-actions"><Button type="submit" variant="primary" loading={busy} disabled={list.loading||!!list.error}>Save budget</Button><Button onClick={onDone}>Cancel</Button></div>
  </Form>
  {addingGroup&&<Modal size="compact" title="Add budget group" onClose={()=>setAddingGroup(false)}><Form onSubmit={async()=>{await run(async()=>{const id=Number(groupId),saved=await api('/periods/'+period.id+'/budget-groups?page=0&id='+id),existing=saved.items[0];setDrafts(old=>({...old,[id]:{...existing,...old[id],id,name:chosenGroup?.name||'No spending group',color:chosenGroup?.color,added:true,remove:false,targets:old[id]?.targets||{}}}));setAddingGroup(false)})}}><PagedSelect url="/spending-groups" label="Spending group" empty="Choose a group" specialOptions={[{value:'0',label:'No spending group'}]} required value={groupId} onChange={setGroupId} onSelectRow={setChosenGroup}/><Button type="submit" variant="primary" loading={busy}>Add group to budget</Button></Form></Modal>}
  {addingCategory&&<Modal size="medium" title={'Add category · '+addingCategory.name} onClose={()=>setAddingCategory(null)}>{creating?<><CreateCategory name="" group={addingCategory.id} expenseOnly notify={notify} done={(id,name)=>{setCategory(String(id));setChosenCategory({id,name:name||'New category'});setCreating(false);refresh()}}/><Button onClick={()=>setCreating(false)}>Back to budget category</Button></>:<Form onSubmit={async()=>{await run(async()=>{const saved=await api('/periods/'+period.id+'/targets?group='+addingCategory.id+'&budget_only=1&page=0&id='+category);edit(addingCategory,{id:Number(category),name:chosenCategory?.name||'Category',amount,carry:future,upcoming:future,baseAmount:drafts[addingCategory.id]?.targets[Number(category)]?.baseAmount??saved.items[0]?.amount_cents??0});setAddingCategory(null)})}}><PagedSelect url="/categories?kind=expense&active=1" label="Category" required value={category} onChange={setCategory} onSelectRow={setChosenCategory} options={chosenCategory?[chosenCategory]:[]}/><Button onClick={()=>setCreating(true)}><Plus size={16}/>Create expense category</Button><Field label="Budget amount" validate={v=>moneyError(v,true)}><input autoFocus={!!chosenCategory} inputMode="decimal" required value={amount} onChange={e=>setAmount(e.target.value)}/></Field><Field label="Use this category in"><select value={future?'future':'once'} onChange={e=>setFuture(e.target.value==='future')}><option value="once">This budget only</option><option value="future">This and upcoming budgets</option></select></Field>{future&&<p className="footnote">Use this amount in new periods and upcoming budgets where this category isn't already included. Existing amounts stay as set.</p>}<Button type="submit" variant="primary" loading={busy}>Add category to budget</Button></Form>}</Modal>}
 </>
}

function BudgetCategories({period,group,draft,revision,onEdit,validate}:{period:number;group:Row;draft?:GroupDraft;revision:number;onEdit:(c:CategoryDraft)=>void;validate:()=>boolean}){
 const list=usePagedList('/periods/'+period+'/targets?group='+group.id+'&budget_only=1',revision)
 const rows=[...new Map([...list.items.map(c=>({id:c.id,name:c.name,amount:decimal(c.amount_cents),carry:!!c.carry_forward,upcoming:false,baseAmount:c.amount_cents})),...Object.values(draft?.targets||{})].map(c=>[c.id,c])).values()].filter(c=>!draft?.targets[c.id]?.remove)
 return <><ListStatus list={list}/>{!list.loading&&!rows.length&&<p className="muted">Add the categories you want to budget for in this group.</p>}{rows.map(c=>{const current=draft?.targets[c.id]||c;return <div className="budget-builder-category" key={c.id}><Field label={current.name} validate={v=>moneyError(v,true)}><input inputMode="decimal" required value={current.amount} onChange={e=>onEdit({...current,amount:e.target.value})}/></Field><Field label={'Schedule · '+current.name}><select value={current.carry?'upcoming':'once'} onChange={e=>onEdit({...current,carry:e.target.value==='upcoming',upcoming:false})}><option value="once">This budget only</option><option value="upcoming">Copy to new budgets</option></select></Field><Button variant="quiet" aria-label={'Remove '+current.name+' from '+group.name} onClick={()=>onEdit({...current,remove:true})}>Remove</Button></div>})}<ListNavigation list={{...list,setPage:page=>{if(validate())list.setPage(page)}}}/></>
}
