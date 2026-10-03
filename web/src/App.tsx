import {PagedSelect} from './PagedList'
import {useEffect,useState,useRef,type ReactNode} from 'react'
import {LayoutDashboard,ArrowLeftRight,CheckCheck,Upload,Wallet,ChartNoAxesCombined,Tags,Settings,LogOut,Menu,ArrowUpRight,Plus,Search} from 'lucide-react'
import {api,setCSRF,money} from './api'
import {Button,Field,Form,Badge,Empty,Toast,Loading,Pagination,PageHeader} from './ui'
import {passwordError,usernameError} from './validation'
import {Transactions} from './Transactions'
import {Imports} from './Imports'
import {Budgets,Accounts,Categories,SettingsPage} from './Manage'
export type Row=Record<string,any>
export type Data={user:Row;accounts:Row[];categories:Row[];periods:Row[];rules:Row[];spendingGroups:Row[];branding:Row;next:Row}
export type PageProps={data:Data;revision:number;refresh:()=>void;notify:(message:string,error?:boolean)=>void}
export function useTask(notify:PageProps['notify']){
 const[busy,setBusy]=useState(false),pending=useRef(false)
 const run=async(fn:()=>Promise<unknown>,message?:string,onError?:(message:string)=>boolean)=>{if(pending.current)return false;pending.current=true;setBusy(true);try{await fn();if(message)notify(message);return true}catch(e){const message=(e as Error).message;if(!onError?.(message))notify(message,true);return false}finally{pending.current=false;setBusy(false)}}
 return{busy,run}
}
const nav=[{name:'Dashboard',icon:LayoutDashboard},{name:'Transactions',icon:ArrowLeftRight},{name:'Review',icon:CheckCheck},{name:'Imports',icon:Upload},{name:'Accounts',icon:Wallet},{name:'Budgets',icon:ChartNoAxesCombined},{name:'Categories',icon:Tags},{name:'Settings',icon:Settings}]
const descriptions:Record<string,string>={Dashboard:'Income, spending, and review for your selected period.',Transactions:'Search, categorize, and split account activity.',Review:'Check the details before approving.',Imports:'Fetch bank transactions or upload exports for review.',Accounts:'Shared household accounts and private accounts.',Budgets:'Category limits and the dates that work for you.',Categories:'Set up how your FNB transactions are categorized.',Settings:'Household preferences, access, and backups.'}
export default function App(){
 const[user,setUser]=useState<Row|null>(null),[ready,setReady]=useState(false),[data,setData]=useState<Data|null>(null),[revision,setRevision]=useState(0)
 const[refreshingData,setRefreshingData]=useState(false)
 const[settingsSection,setSettingsSection]=useState('general')
 const[view,setView]=useState('Dashboard'),[period,setPeriod]=useState(''),[account,setAccount]=useState(''),[more,setMore]=useState(false),[unassigned,setUnassigned]=useState(false)
 const[setup,setSetup]=useState<Row|null>(null),[startupError,setStartupError]=useState(''),[startupRetry,setStartupRetry]=useState(0)
 const noticeSequence=useRef(0)
 const[notice,setNotice]=useState<{id:number;message:string;error:boolean}|null>(null)
 const[theme,setTheme]=useState(()=>localStorage.getItem('finance-theme')||'system')
 const notify=(message:string,error=false)=>{setNotice({id:++noticeSequence.current,message,error});window.dispatchEvent(new CustomEvent('finance-notice',{detail:{message,error}}))}
 const{busy:signingOut,run:signOut}=useTask(notify)
 const refresh=()=>setRevision(v=>v+1)
 useEffect(()=>{
  let alive=true;setReady(false);setStartupError('')
  api('/setup').then(async status=>{
   if(!alive)return
   if(status.required){setSetup(status);setCSRF(status.csrf);return}
   setSetup(null)
   try{const v=await api('/me');if(alive){setUser(v.user);setCSRF(v.csrf)}}catch{}
  }).catch(e=>alive&&setStartupError(e.message)).finally(()=>alive&&setReady(true))
  return()=>{alive=false}
 },[startupRetry])
 useEffect(()=>{const handler=(event:Event)=>notify((event as CustomEvent<string>).detail,true);window.addEventListener('finance-request-error',handler);return()=>window.removeEventListener('finance-request-error',handler)},[])
 useEffect(()=>{const expired=()=>{setUser(null);setData(null);setCSRF('');setNotice({id:++noticeSequence.current,message:'Your session expired. Please sign in again.',error:true})};window.addEventListener('session-expired',expired);return()=>window.removeEventListener('session-expired',expired)},[])
 useEffect(()=>{const query=matchMedia('(prefers-color-scheme: dark)');const apply=()=>{document.documentElement.dataset.theme=theme==='system'?(query.matches?'dark':'light'):theme};apply();query.addEventListener('change',apply);localStorage.setItem('finance-theme',theme);return()=>query.removeEventListener('change',apply)},[theme])
 useEffect(()=>{
  if(!user)return
  let alive=true;setRefreshingData(true)

  api('/me').then(session=>{
   if(!alive)return null
   setCSRF(session.csrf)
   setUser(previous=>previous&&['id','username','admin','budget_member'].every(key=>previous[key]===session.user[key])?previous:session.user)
   return Promise.all([api('/accounts?page=0&page_size=100'),api('/categories?page=0&page_size=100'),session.user.budget_member?api('/periods?page=0&page_size=100'):Promise.resolve({items:[],next:{}}),Promise.resolve([]),api('/spending-groups?page=0&page_size=100'),api('/branding'),Promise.resolve(session.user)])
  }).then(async result=>{
   if(!alive||!result)return
   const[accounts,categories,periods,rules,spendingGroups,branding,latestUser]=result
   if(account&&!accounts.items.some((a:Row)=>String(a.id)===account)){const v=await api('/accounts?page=0&id='+account);accounts.items.push(...v.items)}
   if(period&&latestUser.budget_member&&!periods.items.some((p:Row)=>String(p.id)===period)){const v=await api('/periods?page=0&id='+period);periods.items.push(...v.items)}
   if(!alive)return
   setData({user:latestUser,accounts:accounts.items,categories:categories.items,periods:periods.items,next:periods.next,rules,spendingGroups:spendingGroups.items,branding})
   const today=new Intl.DateTimeFormat('en-CA',{timeZone:'Africa/Johannesburg',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date())
   const current=periods.items.find((p:Row)=>p.start_date<=today&&p.end_date>=today)
   setPeriod(old=>periods.items.some((p:Row)=>String(p.id)===old)?old:String(current?.id||periods.items[0]?.id||''))
   setAccount(old=>accounts.items.some((a:Row)=>String(a.id)===old)?old:'')
  }).catch(e=>alive&&notify(e.message,true)).finally(()=>alive&&setRefreshingData(false));return()=>{alive=false}
 },[user,revision])
 useEffect(()=>{document.title=user&&data?data.branding.display_name+' · Finance tracker':'Finance tracker'},[user,data?.branding.display_name])
 if(!ready)return <div className="initial"><Loading>Loading your workspace</Loading></div>
 if(startupError)return <main className="login"><div className="login-panel"><h1>Unable to load your workspace</h1><p role="alert" className="error-text">{startupError}</p><Button onClick={()=>setStartupRetry(v=>v+1)}>Retry</Button></div></main>
 if(setup)return <Setup onComplete={u=>{setSetup(null);setUser(u);setNotice(null)}} onClosed={()=>{setSetup(null);setCSRF('');setNotice({id:++noticeSequence.current,message:'Setup is already complete. Sign in with your administrator account.',error:false})}}/>
 if(!user)return <><Login onLogin={u=>{setUser(u);setNotice(null)}}/>{notice&&<Toast key={notice.id} message={notice.message} error={notice.error} onDismiss={()=>setNotice(null)}/>}</>
 if(!data)return <div className="initial">{notice?<><p role="alert">{notice.message}</p><Button onClick={refresh}>Retry</Button></>:<Loading>Loading accounts</Loading>}</div>
 const props={data,revision,refresh,notify}
 const go=(page:string)=>{setView(page);setMore(false);setUnassigned(false);setNotice(null)}
 const visibleNav=nav.filter(n=>user.budget_member||!['Dashboard','Budgets'].includes(n.name))
 const current=!user.budget_member&&view==='Dashboard'?'Accounts':view
 return <div className="shell"><a className="skip-link" href="#main-content">Skip to content</a>
  <aside className="sidebar"><div className="brand"><span className="brand-mark">{Array.from(data.branding.display_name as string)[0]}</span><div className="brand-text"><span title={data.branding.display_name}>{data.branding.display_name}</span><small>Finance tracker</small></div></div>
   <nav aria-label="Main navigation">{visibleNav.map(n=><button key={n.name} className={current===n.name?'nav-item active':'nav-item'} aria-current={current===n.name?'page':undefined} onClick={()=>go(n.name)}><n.icon size={19}/>{n.name}</button>)}</nav>
   <div className="sidebar-foot"><span className="dot"/>ZAR · South Africa<small>Self-hosted. Your data stays here.</small></div>
  </aside>
  <div className="workspace">
   <header className="topbar"><span className="mobile-brand" title={data.branding.display_name}>{data.branding.display_name}</span><span className="desktop-label">Personal finance</span><div className="top-actions"><span className="username">{user.username}</span><Button variant="quiet" aria-label="Sign out" loading={signingOut} onClick={()=>signOut(async()=>{await api('/logout','POST');setUser(null);setData(null);setCSRF('')})}><LogOut size={18}/></Button></div></header>
   <main id="main-content" tabIndex={-1}><PageHeader title={current} description={descriptions[current]} workspace={data.branding.display_name} loading={refreshingData}/>
    {user.budget_member&&['Dashboard','Transactions','Review'].includes(current)&&<div className="context-bar"><PagedSelect url="/periods" label="Budget period" optionLabel={p=>p.name+' · '+p.start_date+' – '+p.end_date} value={period} onChange={setPeriod} options={data.periods} empty={current==='Dashboard'?'Current period':'All periods'} revision={revision}/><PagedSelect url="/accounts" label="Account scope" optionLabel={a=>a.name+(a.household?'':' · Private')} value={account} onChange={setAccount} options={data.accounts} empty={current==='Dashboard'?'Household accounts':'All accessible accounts'} revision={revision}/></div>}
    {notice&&<Toast key={notice.id} message={notice.message} error={notice.error} onDismiss={()=>setNotice(null)}/>}
    {current==='Dashboard'&&<Dashboard {...props} period={period} account={account} onReview={()=>go('Review')} onUnassigned={()=>{go('Transactions');setUnassigned(true)}} onImport={()=>go('Imports')}/>}
    {(current==='Transactions'||current==='Review')&&<Transactions {...props} review={current==='Review'} period={period} account={account} unassigned={unassigned} onUnassignedChange={setUnassigned}/>}
    {current==='Imports'&&<Imports {...props}/>}
    {current==='Budgets'&&<Budgets {...props}/>}
    {current==='Accounts'&&<Accounts {...props} onManage={()=>{setSettingsSection('accounts');go('Settings')}}/>}
    {current==='Categories'&&<Categories {...props}/>}
    {current==='Settings'&&<SettingsPage {...props} theme={theme} onTheme={setTheme} section={settingsSection} onSectionChange={setSettingsSection}/>}
   </main>
  </div>
  <nav className="mobile-nav" aria-label="Mobile navigation">{visibleNav.filter(n=>['Dashboard','Transactions','Review'].includes(n.name)).map(n=><button key={n.name} className={current===n.name?'active':''} onClick={()=>go(n.name)} aria-current={current===n.name?'page':undefined}><n.icon size={21}/><span>{n.name}</span></button>)}<button onClick={()=>setMore(!more)} aria-expanded={more}><Menu size={21}/><span>More</span></button></nav>
  {more&&<div className="mobile-more"><nav aria-label="More pages">{visibleNav.filter(n=>!['Dashboard','Transactions','Review'].includes(n.name)).map(n=><button key={n.name} onClick={()=>go(n.name)}><n.icon size={19}/>{n.name}</button>)}</nav></div>}
 </div>
}
function Setup({onComplete,onClosed}:{onComplete:(user:Row)=>void;onClosed:()=>void}){
 const[usernameServerError,setUsernameServerError]=useState('')
 const[username,setUsername]=useState(''),[password,setPassword]=useState(''),[confirmation,setConfirmation]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState('')
 return <main className="login"><div className="login-brand"><span className="brand-mark">F</span>Finance tracker</div><div className="login-panel"><h1>Create admin account</h1><p className="muted">Set up your finance tracker.</p><Form onSubmit={async e=>{
  e.preventDefault();setError('')
  setBusy(true)
  try{const v=await api('/setup','POST',{username,password});setCSRF(v.csrf);onComplete(v.user)}
  catch(e){const message=(e as Error).message;if(message.toLowerCase().includes('username already exists'))setUsernameServerError(message);else setError(message);try{const status=await api('/setup');if(!status.required)onClosed();else setCSRF(status.csrf)}catch{}}
  finally{setBusy(false)}
 }}><Field label="Username" validate={usernameError} serverError={usernameServerError}><input autoFocus autoComplete="username" required minLength={2} maxLength={80} value={username} onChange={e=>{setUsername(e.target.value);setUsernameServerError('')}}/></Field><Field label="Password" validate={passwordError}><input type="password" autoComplete="new-password" required value={password} onChange={e=>setPassword(e.target.value)}/></Field><Field label="Confirm password" validate={value=>value!==password?'Passwords do not match.':''}><input type="password" autoComplete="new-password" required value={confirmation} onChange={e=>setConfirmation(e.target.value)}/></Field>{error&&<Toast message={error} error onDismiss={()=>setError('')}/>}<Button variant="primary" loading={busy} disabled={busy} type="submit">{busy?'Creating account':'Create account'}</Button></Form></div></main>
}
function Login({onLogin}:{onLogin:(user:Row)=>void}){
 const[username,setUsername]=useState(''),[password,setPassword]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState('')
 return <main className="login"><div className="login-brand"><span className="brand-mark">F</span>Finance tracker</div><div className="login-panel"><h1>Welcome back</h1><p className="muted">Sign in to your finance tracker.</p><Form onSubmit={async e=>{e.preventDefault();setBusy(true);setError('');try{const v=await api('/login','POST',{username,password});setCSRF(v.csrf);onLogin(v.user)}catch(e){setError((e as Error).message)}finally{setBusy(false)}}}><Field label="Username"><input autoComplete="username" required value={username} onChange={e=>setUsername(e.target.value)}/></Field><Field label="Password"><input type="password" autoComplete="current-password" required value={password} onChange={e=>setPassword(e.target.value)}/></Field>{error&&<Toast message={error} error onDismiss={()=>setError('')}/>}<Button variant="primary" loading={busy} disabled={busy} type="submit">{busy?'Signing in':'Sign in'}</Button></Form></div></main>
}
function Dashboard({data,period,account,revision,notify,onReview,onUnassigned,onImport}:PageProps&{period:string;account:string;onReview:()=>void;onUnassigned:()=>void;onImport:()=>void}){
 const[categoryPage,setCategoryPage]=useState(0),[balancePage,setBalancePage]=useState(0)
 useEffect(()=>{setCategoryPage(0);setBalancePage(0)},[period,account])
 const[d,setD]=useState<Row|null>(null),[loading,setLoading]=useState(true),[error,setError]=useState('')
 useEffect(()=>{let alive=true;setLoading(true);setError('');api('/dashboard?period='+period+'&account='+account+'&category_page='+categoryPage+'&balance_page='+balancePage).then(v=>alive&&setD(v)).catch(e=>{if(alive){setError(e.message);notify(e.message,true)}}).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[period,account,revision,categoryPage,balancePage])
 if(loading&&!d)return <Loading>Loading this period</Loading>
 if(error)return <Empty title="Dashboard unavailable">{error}</Empty>
 if(!d)return null
 const expense=d.categories.filter((c:Row)=>c.kind==='expense'&&(c.target_cents||c.spent_cents))
 return <>
 {loading&&<Loading>Loading this period</Loading>}
  <div className="period-note"><span>{d.period.start_date} — {d.period.end_date}</span><span>Pending transactions are included</span></div>
  <section className="stats" aria-label="Period totals"><Stat label="Income" value={money(d.income_cents)} hint="Transfers excluded"/><Stat label="Spending" value={money(d.spent_cents)} hint={'Includes '+money(d.pending_spend_cents)+' pending'}/><Stat label={d.has_targets?'Budget remaining':'Net movement'} value={money(d.has_targets?d.remaining_cents:d.income_cents-d.spent_cents)} hint={d.has_targets?'Of '+money(d.budget_cents)+' in limits':'Income less spending'} negative={d.has_targets&&d.remaining_cents<0}/><Stat label="Awaiting review" value={String(d.pending_count)} hint="Check categories and details"/></section>
  <div className="dashboard-grid"><section className="panel spending-panel"><div className="section-head"><div><h2>Spending by category</h2><p className="muted">Spending and category limits for the selected period.</p></div></div>
   {!expense.length?<Empty title="No category spending yet">Upload an FNB export and review your categories to get started.</Empty>:<><div className="category-rows">{expense.map((c:Row)=><div className="category-row" key={c.id}><div><strong>{c.name}</strong>{c.pending_cents>0&&<small>{money(c.pending_cents)} pending</small>}</div><div className="category-value"><strong>{money(c.spent_cents)}</strong>{d.has_targets&&<small>of {money(c.target_cents)}</small>}</div>{d.has_targets&&c.target_cents>0&&<progress aria-label={c.name+' budget used'} max={c.target_cents} value={Math.max(0,c.spent_cents)}/>}</div>)}</div></>}
   <Pagination page={categoryPage} total={d.category_total} loading={loading} onChange={setCategoryPage}/>
   {!!d.uncategorized_count&&<p className="footnote">{d.uncategorized_count} transactions still need categories; their amounts are included in totals.</p>}
  </section>
  <div className="stack"><section className="panel"><h2>Next steps</h2><button className="action-row" onClick={onReview}><CheckCheck size={20}/><span><strong>Review transactions</strong><small>{d.pending_count} awaiting your check</small></span><ArrowUpRight size={17}/></button><button className="action-row" onClick={onImport}><Upload size={20}/><span><strong>Upload FNB exports</strong><small>CSV, OFX, or their ZIP files</small></span><ArrowUpRight size={17}/></button>{d.unassigned_count>0&&<button className="action-row" onClick={onUnassigned}><ArrowLeftRight size={20}/><span><strong>Assign budget periods</strong><small>{d.unassigned_count} transactions outside date ranges</small></span><ArrowUpRight size={17}/></button>}</section>
  <section className="panel"><h2>Bank-reported balances</h2>{!d.balances.length?<Empty title="No accounts yet">Add your first account to begin.</Empty>:d.balances.map((a:Row)=><div className="balance-row" key={a.id}><div><strong>{a.name}</strong><small>{a.balance_date?'As of '+a.balance_date:'No balance imported'}{a.household?'':' · Private'}</small></div><strong>{a.balance_cents===null?'—':money(a.balance_cents)}</strong></div>)}<Pagination page={balancePage} total={d.balance_total} loading={loading} onChange={setBalancePage}/><p className="footnote">Balances reflect the date shown. Transaction history may be incomplete.</p></section></div></div>
  {!data.accounts.length&&<div className="notice">Start in Accounts: add the FNB account number shown in your export.</div>}
 </>
}
function Stat({label,value,hint,negative=false}:{label:string;value:string;hint:string;negative?:boolean}){return <div className="stat"><span>{label}</span><strong className={negative?'negative':''}>{value}</strong><small>{hint}</small></div>}
