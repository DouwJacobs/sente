import {Button,Field} from './ui'
import {useState,useRef,useId} from 'react'
import {Search,SlidersHorizontal,RotateCcw} from 'lucide-react'
import {cents} from './api'
import {moneyError} from './validation'
import {PagedSelect} from './PagedList'
import {ChoiceField} from './Choices'
import {type PageProps} from './shared/types'
export type TransactionFilterValues={category:string;group:string;direction:string;query:string;seen?:string;acceptance?:string;date_from?:string;date_to?:string;minimum?:string;maximum?:string;merchant?:string;tag?:string;excluded?:string}
export const emptyTransactionFilters:TransactionFilterValues={category:'',group:'',direction:'',query:'',seen:'',acceptance:''}
export function transactionFilterQuery(filters:TransactionFilterValues,account=''){
 const params=new URLSearchParams()
 if(filters.category)params.set('category',filters.category)
 if(filters.group)params.set('spending_group',filters.group)
 if(filters.direction)params.set('direction',filters.direction)
 if(filters.query.trim())params.set('q',filters.query.trim())
 if(filters.seen)params.set('seen',filters.seen)
 if(filters.acceptance)params.set('acceptance',filters.acceptance)
 for(const key of ['date_from','date_to','merchant','tag','excluded'] as const)if(filters[key])params.set(key,filters[key]!)
 for(const [key,value] of [['min_amount_cents',filters.minimum],['max_amount_cents',filters.maximum]])if(value){try{params.set(key!,String(cents(value)))}catch{}}
 if(account)params.set('account',account)
 return params.toString()
}
type TransactionFiltersProps={
 data:PageProps['data'];revision:number;value:TransactionFilterValues
 onChange:(value:TransactionFilterValues)=>void;onClear:()=>void;active:boolean;imports:boolean
 account:string;period:string;onAccountChange:(value:string)=>void;onPeriodChange:(value:string)=>void
 unassigned:boolean;onUnassignedChange:(value:boolean)=>void
}
export function TransactionFilters({data,revision,value,onChange,onClear,active,imports,account,period,onAccountChange,onPeriodChange,unassigned,onUnassignedChange}:TransactionFiltersProps){
 const[expanded,setExpanded]=useState(false),toggle=useRef<HTMLButtonElement>(null),panelId=useId()
 const summaries=[value.category&&(value.category==='uncategorized'?'Uncategorized':'Category: '+(data.categories.find(c=>String(c.id)===value.category)?.name||'#'+value.category)),value.group&&(value.group==='unassigned'?'No spending group':'Group: '+(data.spendingGroups.find(g=>String(g.id)===value.group)?.name||'#'+value.group)),value.direction&&(value.direction==='out'?'Money out':'Money in'),!imports&&value.acceptance&&(value.acceptance==='needs_category'?'Needs category':'Accepted'),!imports&&value.seen&&(value.seen==='0'?'Unseen':'Seen'),!imports&&value.date_from&&'From '+value.date_from,!imports&&value.date_to&&'To '+value.date_to,!imports&&value.minimum&&'Minimum '+value.minimum,!imports&&value.maximum&&'Maximum '+value.maximum,!imports&&value.merchant&&'Merchant selected',!imports&&value.tag&&'Tag selected'].filter(Boolean)
 const advancedCount=[value.date_from,value.date_to,value.minimum,value.maximum,value.merchant,value.tag].filter(Boolean).length
 const selectedPeriod=unassigned?undefined:data.periods.find(p=>String(p.id)===period)
 const scopeSummary=[account?(data.accounts.find(a=>String(a.id)===account)?.name||'Selected account'):'All accounts',!imports&&(unassigned?'No budget period':selectedPeriod?.name||'All periods')].filter(Boolean).join(' · ')
 return <section className="transaction-filters transaction-controls" aria-label="Transaction filters" onKeyDown={e=>{if(expanded&&e.key==='Escape'&&!(e.target as HTMLElement).closest('dialog')){e.preventDefault();setExpanded(false);toggle.current?.focus()}}}>
 <div className="transaction-control-bar">
  <div className="transaction-filter-search compact-search">
   <Search size={17} aria-hidden="true"/>
   <Field label="Search transactions"><input type="search" placeholder="Search transactions" value={value.query} onChange={e=>onChange({...value,query:e.target.value})}/></Field>
  </div>
  <Button variant="quiet" ref={toggle} aria-label={'Filters'+(summaries.length?` (${summaries.length})`:'')} aria-expanded={expanded} aria-controls={panelId} onClick={()=>setExpanded(!expanded)}><SlidersHorizontal size={18} aria-hidden="true"/><span className="filter-button-text">Filters{summaries.length?` (${summaries.length})`:''}</span></Button>
  {active&&!expanded&&<Button variant="quiet" aria-label="Clear filters" title="Clear filters" onClick={onClear}><RotateCcw size={18} aria-hidden="true"/></Button>}
 </div>
 <div className="transaction-scope-summary" aria-label="Current transaction scope">{scopeSummary}{summaries.length>0&&<span> · {summaries.length} {summaries.length===1?'filter':'filters'} active</span>}</div>
 <div id={panelId} className={"transaction-filter-reveal"+(expanded?" is-open":"")} inert={!expanded} aria-hidden={!expanded}>
 <div className="transaction-filter-reveal-inner"><div className="transaction-filter-disclosure">
 <div className="filter-card-head"><h3>Filters</h3><Button variant="quiet" aria-label="Clear filters" title="Clear filters" disabled={!active} onClick={onClear}><RotateCcw size={18} aria-hidden="true"/></Button></div>
 <section className="filter-section" aria-label="Account and period"><h3>Account &amp; period</h3><div className="transaction-search-grid">
  <div className="context-filter"><PagedSelect url="/accounts" label="Accounts" optionLabel={a=>a.name+(a.household?'':' · Private')} value={account} onChange={onAccountChange} options={data.accounts} empty="All accounts" revision={revision}/></div>
  {!imports&&data.user.budget_member&&<div className="context-filter"><PagedSelect url="/periods" label="Budget period" hint={unassigned?'Household transactions with no budget period; excludes entries marked outside budgeting.':selectedPeriod?selectedPeriod.start_date+' – '+selectedPeriod.end_date:undefined} optionLabel={p=>p.name} specialOptions={[{value:'unassigned',label:'Not assigned to a budget period'}]} value={unassigned?'unassigned':period} onChange={value=>{onUnassignedChange(value==='unassigned');onPeriodChange(value==='unassigned'?'':value)}} options={data.periods} empty="All periods" revision={revision}/></div>}
 </div></section>
 <section className="filter-section" aria-label="Classification filters"><h3>Classification</h3>
 <div className="filter-classification-grid">
  <div className="context-filter"><PagedSelect url="/categories" label="Filter by category" empty="All categories" specialOptions={[{value:'uncategorized',label:'Uncategorized'}]} options={data.categories} revision={revision} value={value.category} onChange={category=>onChange({...value,category})}/></div>
  <div className="context-filter"><PagedSelect url="/spending-groups" label="Filter by spending group" empty="All groups" specialOptions={[{value:'unassigned',label:'Not assigned'}]} options={data.spendingGroups} revision={revision} value={value.group} onChange={group=>onChange({...value,group})}/></div>
 </div></section>
 <section className="filter-section" aria-label="Transaction status filters"><h3>{imports?'Direction':'Direction & review'}</h3><div className="filter-status-grid">
  <Field label="Money direction"><select value={value.direction} onChange={e=>onChange({...value,direction:e.target.value})}><option value="">Money in and out</option><option value="in">Money in</option><option value="out">Money out</option></select></Field>
  {!imports&&<Field label="Acceptance"><select value={value.acceptance||''} onChange={e=>onChange({...value,acceptance:e.target.value})}><option value="">All statuses</option><option value="needs_category">Needs category</option><option value="accepted">Accepted</option></select></Field>}
  {!imports&&<Field label="Seen by you"><select value={value.seen||''} onChange={e=>onChange({...value,seen:e.target.value})}><option value="">All states</option><option value="0">Unseen</option><option value="1">Seen</option></select></Field>}
 </div></section>
 {!imports&&<details className="details advanced-transaction-filters">
  <summary>More filters{advancedCount ? ` (${advancedCount} active)` : ''}</summary>
  <div className="advanced-filter-grid">
  <section className="filter-section" aria-label="Date range filters"><h3>Date range</h3><div className="filter-pair-grid"><Field label="From date"><input type="date" value={value.date_from||''} onChange={e=>onChange({...value,date_from:e.target.value})}/></Field>
  <Field label="To date" validate={v=>v&&value.date_from&&v<value.date_from?'End date must follow start date.':''}><input type="date" value={value.date_to||''} onChange={e=>onChange({...value,date_to:e.target.value})}/></Field>
  </div></section><section className="filter-section" aria-label="Amount range filters"><h3>Amount range</h3><div className="filter-pair-grid">
  <Field label="Minimum signed amount" validate={v=>v?moneyError(v):''}><input aria-describedby={panelId+"-amount-hint"} inputMode="decimal" value={value.minimum||''} onChange={e=>onChange({...value,minimum:e.target.value})}/></Field>
  <Field label="Maximum signed amount" validate={v=>v?moneyError(v):''}><input aria-describedby={panelId+"-amount-hint"} inputMode="decimal" value={value.maximum||''} onChange={e=>onChange({...value,maximum:e.target.value})}/></Field>
  </div><p className="filter-guidance" id={panelId+"-amount-hint"}>Use negative amounts for expenses and positive amounts for income.</p></section>
  <section className="filter-section" aria-label="Merchant and tag filters"><h3>Merchant &amp; tag</h3><div className="filter-pair-grid"><div className="context-filter"><ChoiceField onError={message=>window.dispatchEvent(new CustomEvent('finance-request-error',{detail:message}))} source={'/labels?kind=merchant'+(account?'&account='+account:'')} label="Merchant" options={[]} value={value.merchant?Number(value.merchant):null} onChange={id=>onChange({...value,merchant:id===null?'':String(id)})} empty="All merchants" optionLabel={m=>m.name+(account?'':' · '+m.account_name)}/></div><div className="context-filter"><ChoiceField onError={message=>window.dispatchEvent(new CustomEvent('finance-request-error',{detail:message}))} source={'/labels?kind=tag'+(account?'&account='+account:'')} label="Tag" options={[]} value={value.tag?Number(value.tag):null} onChange={id=>onChange({...value,tag:id===null?'':String(id)})} empty="All tags" optionLabel={t=>t.name+(account?'':' · '+t.account_name)}/></div></div></section>
  </div>
 </details>}
 {summaries.length>0&&<div className="active-filter-summary" aria-label="Active filters">{summaries.map(label=><span key={String(label)}>{label}</span>)}</div>}

 </div></div></div>
 {value.excluded&&<p className="footnote">Showing household transactions unassigned or marked outside budgets. Clear filters to broaden.</p>}
 {imports&&<p className="footnote">Shows imports containing matching transactions. Import issues and original import details show every row; use View transactions to see only matches.</p>}
 </section>
}
