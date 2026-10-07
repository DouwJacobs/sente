import {GlobalSearch} from './GlobalSearch'
import {IncomeBucket} from './IncomeBucket'
import {PeriodNavigation,DailyGuide} from './CoreWorkflows'
import {SpendingTransactions} from './DashboardTransactions'
import {TransactionAccess,useTransactionAccess,type TransactionScope} from './TransactionAccess'
import {SpendingBucket} from './SpendingBucket'
import {LimitEditor} from './LimitEditor'
import {OAuthConsent} from './OAuthConsent'
import {PagedSelect} from './PagedList'
import {PixelMark} from './PixelScene'
import {useEffect,useState,useRef,type ReactNode} from 'react'
import {LayoutDashboard,ArrowLeftRight,CheckCheck,Upload,Wallet,ChartNoAxesCombined,Tags,Settings,LogOut,Menu,ArrowUpRight,Plus,Search,TrendingUp,TrendingDown,Target,Sun,Moon} from 'lucide-react'
import {api,setCSRF,money} from './api'
import {Button,Field,Form,Badge,Empty,Toast,Loading,Pagination,PageHeader,Tabs,Modal} from './ui'
import {passwordError,usernameError} from './validation'
import {Transactions} from './Transactions'
import {Imports} from './Imports'
import {TransactionFilters,emptyTransactionFilters,transactionFilterQuery} from './TransactionFilters'
import {Budgets,Accounts,Categories,SettingsPage} from './Manage'
export type Row=Record<string,any>
export type Data={user:Row;accounts:Row[];categories:Row[];periods:Row[];rules:Row[];spendingGroups:Row[];branding:Row;next:Row}
export type PageProps={data:Data;revision:number;refresh:()=>void;notify:(message:string,error?:boolean)=>void}
export function useTask(notify:PageProps['notify']){
 const[busy,setBusy]=useState(false),pending=useRef(false)
 const run=async(fn:()=>Promise<unknown>,message?:string,onError?:(message:string)=>boolean)=>{if(pending.current)return false;pending.current=true;setBusy(true);try{await fn();if(message)notify(message);return true}catch(e){const message=(e as Error).message;if(!onError?.(message))notify(message,true);return false}finally{pending.current=false;setBusy(false)}}
 return{busy,run}
}
const nav=[{name:'Dashboard',icon:LayoutDashboard},{name:'Transactions',icon:ArrowLeftRight},{name:'Accounts',icon:Wallet},{name:'Budgets',icon:ChartNoAxesCombined},{name:'Categories',icon:Tags},{name:'Settings',icon:Settings}]
const descriptions:Record<string,string>={Dashboard:'Income, spending, and review for your selected period.',Transactions:'Import, categorize, and review your transactions.',Review:'Check the details before approving.',Imports:'Get bank transactions or upload a statement for review.',Accounts:'Shared household accounts and private accounts.',Budgets:'Category limits and the dates that work for you.',Categories:'Organize categories, spending groups, and automatic rules.',Settings:'Household preferences, access, and backups.'}
export default function App(){
 const consent=window.location.pathname==='/mcp/authorize'
 const[user,setUser]=useState<Row|null>(null),[ready,setReady]=useState(false),[data,setData]=useState<Data|null>(null),[revision,setRevision]=useState(0)
 const[refreshingData,setRefreshingData]=useState(false),periodInitialized=useRef(false)
 const[transactionTab,setTransactionTab]=useState('all'),[importScope,setImportScope]=useState<number[]>([]),[workCounts,setWorkCounts]=useState({review:0,imports:0})
 const[reviewFilters,setReviewFilters]=useState({seen:'',acceptance:'needs_category'})
 const[transactionFilters,setTransactionFilters]=useState(emptyTransactionFilters),[transactionQuery,setTransactionQuery]=useState('')
 useEffect(()=>{const timer=setTimeout(()=>setTransactionQuery(transactionFilters.query),250);return()=>clearTimeout(timer)},[transactionFilters.query])
 const[settingsSection,setSettingsSection]=useState('general')
 const[categoryTab,setCategoryTab]=useState('categories')
 const[searchOpen,setSearchOpen]=useState(false)
 useEffect(()=>{
  const handler=(e:KeyboardEvent)=>{
   if((e.metaKey||e.ctrlKey)&&e.key.toLowerCase()==='k'){
    e.preventDefault();setSearchOpen(v=>!v)
   }else if(e.key==='/'&&!searchOpen){
    const el=document.activeElement
    if(el&&(el.tagName==='INPUT'||el.tagName==='TEXTAREA'||el.getAttribute('contenteditable')==='true'))return
    e.preventDefault();setSearchOpen(true)
   }
  }
  window.addEventListener('keydown',handler)
  return()=>window.removeEventListener('keydown',handler)
 },[searchOpen])
 const[view,setView]=useState('Dashboard'),[period,setPeriod]=useState(''),[account,setAccount]=useState(''),[more,setMore]=useState(false),[unassigned,setUnassigned]=useState(false)
 const[setup,setSetup]=useState<Row|null>(null),[startupError,setStartupError]=useState(''),[startupRetry,setStartupRetry]=useState(0)
 const noticeSequence=useRef(0)
 const[notice,setNotice]=useState<{id:number;message:string;error:boolean}|null>(null)
 const[theme,setTheme]=useState(()=>localStorage.getItem('finance-theme')||'system')
 const[resolvedTheme,setResolvedTheme]=useState(()=>matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light')
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
 useEffect(()=>{const query=matchMedia('(prefers-color-scheme: dark)');const apply=()=>{const resolved=theme==='system'?(query.matches?'dark':'light'):theme;document.documentElement.dataset.theme=resolved;setResolvedTheme(resolved)};apply();query.addEventListener('change',apply);localStorage.setItem('finance-theme',theme);return()=>query.removeEventListener('change',apply)},[theme])
 useEffect(()=>{
  if(!user||consent)return
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
   if(!periodInitialized.current){periodInitialized.current=true;setPeriod(String(current?.id||periods.items[0]?.id||''))}else setPeriod(old=>!old||periods.items.some((p:Row)=>String(p.id)===old)?old:'')
   setAccount(old=>accounts.items.some((a:Row)=>String(a.id)===old)?old:'')
  }).catch(e=>alive&&notify(e.message,true)).finally(()=>alive&&setRefreshingData(false));return()=>{alive=false}
 },[user,revision])
 useEffect(()=>{if(!user||consent)return;let alive=true;Promise.all([api('/transactions?pending=1'),api('/imports?page=0')]).then(([t,i])=>{if(alive)setWorkCounts({review:t.total,imports:i.pending_total||0})}).catch(()=>{});return()=>{alive=false}},[user,revision])
 useEffect(()=>{document.title=user&&data?data.branding.display_name+' · Sente':'Sente'},[user,data?.branding.display_name])
 if(!ready)return <div className="initial"><Loading>Loading your workspace</Loading></div>
 if(startupError)return <main className="login"><div className="login-panel"><h1>Unable to load your workspace</h1><p role="alert" className="error-text">{startupError}</p><Button onClick={()=>setStartupRetry(v=>v+1)}>Retry</Button></div></main>
 if(setup)return <Setup onComplete={u=>{setSetup(null);setUser(u);setNotice(null)}} onClosed={()=>{setSetup(null);setCSRF('');setNotice({id:++noticeSequence.current,message:'Setup is already complete. Sign in with your administrator account.',error:false})}}/>
 if(!user)return <><Login onAttempt={()=>setNotice(null)} onError={message=>notify(message,true)} onLogin={u=>{setUser(u);setNotice(null)}}/>{notice&&<Toast key={notice.id} message={notice.message} error={notice.error} autoDismiss={false} onDismiss={()=>setNotice(null)}/>}</>
 if(consent)return <><OAuthConsent notify={notify} onSignOut={async()=>{await api('/logout','POST');setUser(null);setData(null);setCSRF('')}}/>{notice&&<Toast key={notice.id} message={notice.message} error={notice.error} autoDismiss={false} onDismiss={()=>setNotice(null)}/>}</>
 if(!data)return <div className="initial">{notice?<><p role="alert">{notice.message}</p><Button onClick={refresh}>Retry</Button></>:<Loading>Loading accounts</Loading>}</div>
 const props={data,revision,refresh,notify}
 const activeTransactionFilters=transactionTab==='review'?{...transactionFilters,...reviewFilters}:transactionFilters
 const changeTransactionFilters=(value:typeof transactionFilters)=>{if(transactionTab==='review'){setReviewFilters({seen:value.seen||'',acceptance:value.acceptance||''});setTransactionFilters({...value,seen:transactionFilters.seen,acceptance:transactionFilters.acceptance})}else setTransactionFilters(value)}
 const filterQuery=transactionFilterQuery({...activeTransactionFilters,query:transactionQuery})
 const clearTransactionFilters=()=>{setTransactionFilters(emptyTransactionFilters);setReviewFilters({seen:'',acceptance:''});setTransactionQuery('');setPeriod('');setAccount('');setUnassigned(false);setImportScope([])}
 const go=(page:string)=>{setView(page);setMore(false);setUnassigned(false);if(page==='Transactions'&&view!=='Transactions'){setPeriod('');setAccount('')}window.scrollTo({top:0});}
 const openTransactions=(tab:string,ids:number[]=[])=>{setTransactionTab(tab);setImportScope(ids);if(tab==='review'||ids.length){setPeriod('');setAccount('')}if(tab==='review'){setTransactionFilters(emptyTransactionFilters);setTransactionQuery('');setReviewFilters({seen:'',acceptance:'needs_category'})}go('Transactions')}
 const openBanking=()=>{setSettingsSection('banking');go('Settings')}
 const viewTransactions=(scope:TransactionScope)=>{setTransactionTab('all');setImportScope([]);setTransactionFilters({...emptyTransactionFilters,category:scope.category||'',group:scope.group||'',date_from:scope.dateFrom||'',date_to:scope.dateTo||'',excluded:scope.excluded?'1':''});setTransactionQuery('');go('Transactions');setPeriod(scope.period||'');setAccount(scope.account||'');setUnassigned(false)}
 const navigateFromSearch=(page:string,tab?:string)=>{if(page==='Categories'&&tab)setCategoryTab(tab);go(page)}
 const visibleNav=nav.filter(n=>user.budget_member||!['Dashboard','Budgets'].includes(n.name))
 const current=!user.budget_member&&view==='Dashboard'?'Accounts':view
 return <TransactionAccess {...props} viewTransactions={viewTransactions}>
  <GlobalSearchWrapper open={searchOpen} onClose={()=>setSearchOpen(false)} onNavigate={navigateFromSearch} notify={notify} refresh={refresh} data={data} revision={revision}/>
  <div className="shell"><a className="skip-link" href="#main-content">Skip to content</a>
  <aside className="sidebar"><div className="brand"><span className="brand-mark"><PixelMark/></span><div className="brand-text"><span title={data.branding.display_name}>{data.branding.display_name}</span><small>Sente</small></div></div>
   <nav aria-label="Main navigation">{visibleNav.map(n=><button key={n.name} className={current===n.name?'nav-item active':'nav-item'} aria-current={current===n.name?'page':undefined} onClick={()=>go(n.name)}><n.icon size={19}/>{n.name}</button>)}</nav>
   <div className="sidebar-foot"><span className="dot"/>ZAR · South Africa<small>Self-hosted. Your data stays here.</small></div>
  </aside>
  <div className="workspace">
   <header className="topbar"><span className="mobile-brand" title={data.branding.display_name}>{data.branding.display_name}</span><span className="desktop-label">Personal finance</span><div className="top-actions"><Button variant="secondary" className="topbar-search-btn" onClick={()=>setSearchOpen(true)} aria-label="Search workspace"><Search size={15} aria-hidden="true"/><span>Search</span></Button><span className="username">{user.username}</span><Button variant="quiet" className="topbar-icon" aria-label={resolvedTheme==='dark'?'Switch to light theme':'Switch to dark theme'} title={resolvedTheme==='dark'?'Switch to light theme':'Switch to dark theme'} onClick={()=>setTheme(resolvedTheme==='dark'?'light':'dark')}>{resolvedTheme==='dark'?<Sun size={18} aria-hidden="true"/>:<Moon size={18} aria-hidden="true"/>}</Button><Button variant="quiet" aria-label="Sign out" loading={signingOut} onClick={()=>signOut(async()=>{await api('/logout','POST');setUser(null);setData(null);setCSRF('')})}><LogOut size={18}/></Button></div></header>
   <main id="main-content" tabIndex={-1}><PageHeader title={current} description={descriptions[current]} workspace={data.branding.display_name} loading={refreshingData}/>
    {current==='Transactions'&&<Tabs id="transactions" label="Transaction workspace" items={[{id:'all',label:'All transactions'},{id:'review',label:'Needs review ('+workCounts.review+')'},{id:'imports',label:'Import activity ('+workCounts.imports+')'}]} value={transactionTab} onChange={tab=>{setTransactionTab(tab);setImportScope([]);setUnassigned(false);if(tab==='review'){setPeriod('');setAccount('');setTransactionFilters(emptyTransactionFilters);setTransactionQuery('');setReviewFilters({seen:'',acceptance:'needs_category'})}}}/>}
    {current==='Dashboard'&&user.budget_member&&<div className="context-bar">
     {user.budget_member&&<div className="context-filter"><PagedSelect url="/periods" label="Budget period" hint={data.periods.find(p=>String(p.id)===period)?data.periods.find(p=>String(p.id)===period)!.start_date+' – '+data.periods.find(p=>String(p.id)===period)!.end_date:undefined} optionLabel={p=>p.name} value={period} onChange={setPeriod} options={data.periods} empty={current==='Dashboard'?'Current period':'All periods'} revision={revision}/></div>}
     <div className="context-filter"><PagedSelect url="/accounts" label="Accounts" optionLabel={a=>a.name+(a.household?'':' · Private')} value={account} onChange={setAccount} options={data.accounts} empty={current==='Dashboard'?'Household accounts':'All accessible accounts'} revision={revision}/></div>
     <PeriodNavigation period={period} revision={revision} notify={notify} onChange={setPeriod}/>
    </div>}
    {current==='Budgets'&&user.budget_member&&<PeriodNavigation period={period} revision={revision} notify={notify} onChange={setPeriod}/>}
    {current==='Transactions'&&<TransactionFilters data={data} revision={revision} value={activeTransactionFilters} onChange={changeTransactionFilters} onClear={clearTransactionFilters} active={!!(transactionFilterQuery(activeTransactionFilters)||account||period||unassigned||importScope.length)} imports={transactionTab==='imports'} account={account} period={period} onAccountChange={setAccount} onPeriodChange={setPeriod} unassigned={unassigned} onUnassignedChange={setUnassigned}/>}
    {notice&&<Toast key={notice.id} message={notice.message} error={notice.error} onDismiss={()=>setNotice(null)}/>}
    {current==='Dashboard'&&<Dashboard {...props} period={period} account={account} onReview={()=>openTransactions('review')} onUnassigned={()=>{openTransactions('all');setUnassigned(true)}} onImport={()=>openTransactions('imports')} stagedCount={workCounts.imports} onAccounts={()=>go('Accounts')}/>}
    {current==='Transactions'&&<div role="tabpanel" id={'transactions-panel-'+transactionTab} aria-labelledby={'transactions-tab-'+transactionTab}>
     {transactionTab==='imports'?<Imports {...props} filterQuery={transactionFilterQuery({...transactionFilters,query:transactionQuery,seen:'',acceptance:''},account)} onClearFilters={clearTransactionFilters} onReview={ids=>openTransactions('review',ids)} onTransactions={ids=>openTransactions('all',ids)} onBanking={openBanking}/>:<>{importScope.length>0&&<div className="toolbar"><Button onClick={()=>setImportScope([])}>Show all imports</Button></div>}<Transactions {...props} filterQuery={filterQuery} review={transactionTab==='review'} period={period} account={account} importIds={importScope} onClearFilters={clearTransactionFilters} onImport={()=>openTransactions('imports')} stagedCount={workCounts.imports} unassigned={unassigned}/></>}
    </div>}
    {current==='Budgets'&&<Budgets {...props} period={period} account={account} onDashboard={id=>{setPeriod(String(id));setAccount('');go('Dashboard')}}/>}
    {current==='Accounts'&&<Accounts {...props} onManage={openBanking} onTransactions={()=>openTransactions('imports')}/>}
    {current==='Categories'&&<Categories {...props} onAccounts={()=>go('Accounts')}/>}
    {current==='Settings'&&<SettingsPage {...props} theme={theme} onTheme={setTheme} section={settingsSection} onSectionChange={setSettingsSection} onAccounts={()=>go('Accounts')} onTransactions={()=>openTransactions('imports')}/>}
   </main>
  </div>
  <nav className="mobile-nav" aria-label="Mobile navigation">{visibleNav.filter(n=>['Dashboard','Transactions','Accounts'].includes(n.name)).map(n=><button key={n.name} className={current===n.name?'active':''} onClick={()=>go(n.name)} aria-current={current===n.name?'page':undefined}><n.icon size={21}/><span>{n.name}</span></button>)}<button onClick={()=>setMore(!more)} aria-expanded={more}><Menu size={21}/><span>More</span></button></nav>
  {more&&<div className="mobile-more"><nav aria-label="More pages">{visibleNav.filter(n=>!['Dashboard','Transactions','Accounts'].includes(n.name)).map(n=><button key={n.name} onClick={()=>go(n.name)}><n.icon size={19}/>{n.name}</button>)}</nav></div>}
 </div></TransactionAccess>
}
function Setup({onComplete,onClosed}:{onComplete:(user:Row)=>void;onClosed:()=>void}){
 const[usernameServerError,setUsernameServerError]=useState('')
 const[username,setUsername]=useState(''),[password,setPassword]=useState(''),[confirmation,setConfirmation]=useState(''),[busy,setBusy]=useState(false),[error,setError]=useState('')
 return <main className="login"><div className="login-brand"><span className="brand-mark"><PixelMark/></span>Sente</div><div className="login-panel"><h1>Create admin account</h1><p className="muted">Set up Sente for your household.</p><Form onSubmit={async e=>{
  e.preventDefault();setError('')
  setBusy(true)
  try{const v=await api('/setup','POST',{username,password});setCSRF(v.csrf);onComplete(v.user)}
  catch(e){const message=(e as Error).message;if(message.toLowerCase().includes('username already exists'))setUsernameServerError(message);else setError(message);try{const status=await api('/setup');if(!status.required)onClosed();else setCSRF(status.csrf)}catch{}}
  finally{setBusy(false)}
 }}><Field label="Username" validate={usernameError} serverError={usernameServerError}><input autoFocus autoComplete="username" required minLength={2} maxLength={80} value={username} onChange={e=>{setUsername(e.target.value);setUsernameServerError('')}}/></Field><Field label="Password" validate={passwordError}><input type="password" autoComplete="new-password" required value={password} onChange={e=>setPassword(e.target.value)}/></Field><Field label="Confirm password" validate={value=>value!==password?'Passwords do not match.':''}><input type="password" autoComplete="new-password" required value={confirmation} onChange={e=>setConfirmation(e.target.value)}/></Field>{error&&<Toast message={error} error onDismiss={()=>setError('')}/>}<Button variant="primary" loading={busy} disabled={busy} type="submit">{busy?'Creating account':'Create account'}</Button></Form></div></main>
}
function Login({onLogin,onAttempt,onError}:{onLogin:(user:Row)=>void;onAttempt:()=>void;onError:(message:string)=>void}){
 const[username,setUsername]=useState(''),[password,setPassword]=useState(''),[busy,setBusy]=useState(false),pending=useRef(false)
 return <main className="login"><div className="login-brand"><span className="brand-mark"><PixelMark/></span>Sente</div><div className="login-panel"><h1>Welcome back</h1><p className="muted">Sign in to Sente.</p><Form onSubmit={async e=>{e.preventDefault();if(pending.current)return;pending.current=true;setBusy(true);onAttempt();try{const v=await api('/login','POST',{username,password});setCSRF(v.csrf);onLogin(v.user)}catch(e){onError((e as Error).message)}finally{pending.current=false;setBusy(false)}}}><Field label="Username"><input autoComplete="username" required value={username} onChange={e=>setUsername(e.target.value)}/></Field><Field label="Password"><input type="password" autoComplete="current-password" required value={password} onChange={e=>setPassword(e.target.value)}/></Field><Button variant="primary" loading={busy} disabled={busy} type="submit">{busy?'Signing in':'Sign in'}</Button></Form></div></main>
}
function Dashboard({data,period,account,revision,notify,onReview,onUnassigned,onImport,stagedCount,onAccounts,refresh}:PageProps&{period:string;account:string;onReview:()=>void;onUnassigned:()=>void;onImport:()=>void;stagedCount:number;onAccounts:()=>void}){
 const {viewTransactions}=useTransactionAccess()
 const[budgetSort,setBudgetSort]=useState('alphabetical')
 const[categoryPage,setCategoryPage]=useState(0),[balancePage,setBalancePage]=useState(0),[groupPage,setGroupPage]=useState(0),[editingLimits,setEditingLimits]=useState(false)
 useEffect(()=>{setCategoryPage(0);setBalancePage(0);setGroupPage(0)},[period,account])
 const[d,setD]=useState<Row|null>(null),[loading,setLoading]=useState(true),[error,setError]=useState('')
 useEffect(()=>{let alive=true;setLoading(true);setError('');api('/dashboard?period='+period+'&account='+account+'&category_page='+categoryPage+'&balance_page='+balancePage+'&group_page='+groupPage+'&sort='+budgetSort).then(v=>alive&&setD(v)).catch(e=>{if(alive){setError(e.message);notify(e.message,true)}}).finally(()=>alive&&setLoading(false));return()=>{alive=false}},[period,account,revision,categoryPage,balancePage,groupPage,budgetSort])
 if(loading&&!d)return <Loading>Loading this period</Loading>
 if(error)return <Empty title="Dashboard unavailable">{error}</Empty>
 if(!d)return null
 const expense=d.categories.filter((c:Row)=>c.kind==='expense'&&(c.target_cents||c.spent_cents))
 return <>
 {loading&&<Loading>Loading this period</Loading>}
  {editingLimits&&<Modal title={'Group budgets · '+d.period.name} onClose={()=>setEditingLimits(false)}><LimitEditor period={d.period} notify={notify} refresh={refresh} revision={revision} onDone={()=>setEditingLimits(false)}/></Modal>}
  <section className="stats dashboard-overview" aria-label="Period totals"><Stat tone="budget" label={d.has_targets?'Budget remaining':'Net movement'} value={money(d.has_targets?d.remaining_cents:d.income_cents-d.spent_cents)} hint={d.has_targets?'Of '+money(d.budget_cents)+' in limits':'Income less spending'} negative={d.has_targets&&d.remaining_cents<0}/><Stat tone="income" label="Income" value={money(d.income_cents)} hint="Transfers excluded"/><Stat tone="spending" label="Spending" value={money(d.spent_cents)} hint={'Includes '+money(d.pending_spend_cents)+' pending'}/><Stat tone="review" label="Needs categories" value={String(d.pending_count)} hint="Categorize missing allocations"/></section>
  <DailyGuide period={d.period} remaining={d.remaining_cents} hasTargets={d.has_targets&&d.budget_cents>0} account={account}/>
  <div className="dashboard-grid"><section className="panel spending-panel" aria-label="Spending by group"><div className="section-head"><div><h2>Spending by group</h2><p className="muted">Budget, spending and what is left.</p></div><div className="toolbar-actions spending-controls"><Field label="Sort budgets"><select value={budgetSort} onChange={e=>{setBudgetSort(e.target.value);setCategoryPage(0);setGroupPage(0)}}><option value="alphabetical">Alphabetical</option><option value="spending">Spending: highest first</option><option value="remaining">Remaining: lowest first</option></select></Field>{d.has_targets&&<Button onClick={()=>setEditingLimits(true)}>Edit budgets</Button>}</div></div>
   {!d.spending_groups?.length?<Empty title="No spending yet">Import transactions to see your spending groups here.</Empty>:d.spending_groups.map((g:Row)=><SpendingBucket key={period+':'+account+':'+groupPage+':'+g.id} group={g} period={String(d.period.id)} periodName={d.period.name} account={account} groupPage={groupPage} sort={budgetSort} hasTargets={d.has_targets} notify={notify} revision={revision} canEditBudget={data.user.budget_member} refresh={refresh}/>)}
   <IncomeBucket key={String(d.period.id)+':'+account} value={d} account={account} revision={revision} notify={notify}/>
   <Pagination page={groupPage} total={d.group_total||0} loading={loading} onChange={setGroupPage}/>
   <details className="category-limits-summary"><summary>{d.has_targets?'Category totals across all groups':'Category totals across all groups'}</summary><p className="footnote">{d.has_targets?'Category totals combine the separate budgets and spending in each group.':'Totals cover the selected account only.'}</p>
   {!expense.length?<Empty title="No category spending yet">Import transactions and review your categories to get started.</Empty>:<><div className="category-rows">{expense.map((c:Row)=><div className="category-row" key={c.id}><div><strong>{c.name}</strong>{c.pending_cents>0&&<small>{money(c.pending_cents)} pending</small>}</div><div className="category-value"><strong>{money(c.spent_cents)}</strong>{d.has_targets&&<small>of {money(c.target_cents)}</small>}{d.has_targets&&c.target_cents>0&&c.spent_cents>c.target_cents&&<small className="negative">{money(c.spent_cents-c.target_cents)} over limit</small>}</div>{d.has_targets&&c.target_cents>0&&<progress className={c.spent_cents>c.target_cents?'over-budget':undefined} aria-label={c.name+' budget used'} max={c.target_cents} value={Math.max(0,c.spent_cents)}/>}<SpendingTransactions label={c.name+' transactions across all groups'} scope={{category:String(c.id),period:String(d.period.id),account}} revision={revision} notify={notify}/></div>)}</div></>}
   <div className="panel-footer"><Pagination page={categoryPage} total={d.category_total} loading={loading} onChange={setCategoryPage}/>
   </div></details><div className="panel-footer">
   {!!d.uncategorized_count&&<p className="footnote">{d.uncategorized_count} {d.uncategorized_count===1?'transaction still needs':'transactions still need'} categories; their amounts are included in totals.</p>}</div>
  </section>
  <div className="dashboard-support"><section className="panel" aria-label="Next steps"><h2>Next steps</h2>
   {!data.accounts.length&&<button className="action-row" onClick={onAccounts}><Wallet size={20}/><span><strong>{data.user.admin?'Set up accounts':'View account access'}</strong><small>{data.user.admin?'Connect FNB, discover accounts, or add one manually':'Ask an administrator to grant account access'}</small></span><ArrowUpRight size={17}/></button>}
   {stagedCount>0&&<button className="action-row" onClick={onImport}><Upload size={20}/><span><strong>Resolve import issues</strong><small>{stagedCount} {stagedCount===1?'account import needs':'account imports need'} attention</small></span><ArrowUpRight size={17}/></button>}
   {d.pending_count>0&&<button className="action-row" onClick={onReview}><CheckCheck size={20}/><span><strong>Review transactions</strong><small>{d.pending_count} need categories</small></span><ArrowUpRight size={17}/></button>}
   {data.accounts.length>0&&!stagedCount&&<button className="action-row" onClick={onImport}><Upload size={20}/><span><strong>Import transactions</strong><small>Get transactions from FNB or upload a statement</small></span><ArrowUpRight size={17}/></button>}
   {d.unassigned_count>0&&<button className="action-row" onClick={onUnassigned}><ArrowLeftRight size={20}/><span><strong>Assign budget periods</strong><small>{d.unassigned_count} {d.unassigned_count===1?'transaction':'transactions'} outside date ranges</small></span><ArrowUpRight size={17}/></button>}
  </section>
  <section className="panel" aria-label="Bank-reported balances"><h2>Bank-reported balances</h2>{!d.balances.length?<Empty title={data.accounts.length?'No balances for these accounts':'No accounts yet'}>{data.accounts.length?'Choose another account or refresh balances on the Accounts page.':'Set up an account to begin.'}<Button onClick={onAccounts}>Open Accounts</Button></Empty>:d.balances.map((a:Row)=><div className="balance-row" key={a.id}><div><button type="button" className="transaction-link" onClick={()=>viewTransactions({account:String(a.id)})}><strong>{a.name}</strong></button><small>{a.balance_date?'As of '+a.balance_date:'No balance imported'}{a.household?'':' · Private'}</small></div><strong>{a.balance_cents===null?'—':money(a.balance_cents)}</strong></div>)}<Pagination page={balancePage} total={d.balance_total} loading={loading} onChange={setBalancePage}/><p className="footnote">Balances reflect the date shown. Transaction history may be incomplete.</p></section></div></div>
 </>
}
function Stat({label,value,hint,negative=false,tone}:{label:string;value:string;hint:string;negative?:boolean;tone:'income'|'spending'|'budget'|'review'}){
 const Icon={income:TrendingUp,spending:TrendingDown,budget:Target,review:CheckCheck}[tone]
 return <div className={'stat '+tone}><div className="stat-heading"><span>{label}</span><span className="stat-icon" aria-hidden="true"><Icon size={18}/></span></div><strong className={negative?'negative':''}>{value}</strong><small>{hint}</small></div>
}
function GlobalSearchWrapper({open,onClose,onNavigate,notify,refresh,data,revision}:{open:boolean;onClose:()=>void;onNavigate:(page:string,tab?:string)=>void;notify:PageProps['notify'];refresh:()=>void;data:PageProps['data'];revision?:number}){
 const{openTransaction,viewTransactions}=useTransactionAccess()
 return <GlobalSearch open={open} onClose={onClose} onNavigate={onNavigate} openTransaction={openTransaction} viewTransactions={viewTransactions} notify={notify} refresh={refresh} data={data} revision={revision}/>
}
