import {useState,useEffect,useRef,useId} from 'react'
import {Button,Loading,Modal} from './ui'
import {SpendingGroupEditor} from './SpendingGroupEditor'
import {Search,X} from 'lucide-react'
import {api,money} from './api'
import {GroupDot} from './Choices'
import {MerchantAvatar} from './MerchantAvatar'
import {CategoryEditor,MerchantRuleEditor} from './Organisation'
import {RuleEditor} from './Rules'
import type {Row,PageProps} from './shared/types'
import type {TransactionScope} from './TransactionAccess'

export interface SearchResults {
  categories: Row[]
  spending_groups: Row[]
  transactions: Row[]
  merchant_rules: Row[]
  rules: Row[]
}

interface GlobalSearchProps {
  open: boolean
  onClose: () => void
  onNavigate: (page: string, tab?: string) => void
  openTransaction: (id: number) => void
  viewTransactions: (scope: TransactionScope) => void
  notify: PageProps['notify']
  refresh: () => void
  data: PageProps['data']
  revision?: number
}

interface FlatItem {
  id: string
  section: string
  label: string
  detail?: string
  amount?: number
  groupDot?: string
  merchantName?: string
  merchantLogo?: string
  onSelect: () => void
}

export function GlobalSearch({open, onClose, openTransaction, notify, refresh, data, revision}: GlobalSearchProps) {
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(false)
  const [results, setResults] = useState<SearchResults>({
    categories: [], spending_groups: [], transactions: [], merchant_rules: [], rules: [],
  })
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const resultId = useId()
  const [error,setError] = useState('')
  const [retry,setRetry] = useState(0)

  // Specific entity editors opened from search
  const [editingCategory, setEditingCategory] = useState<Row | null>(null)
  const [editingGroup, setEditingGroup] = useState<Row | null>(null)
  const [editingMerchantRule, setEditingMerchantRule] = useState<Row | null>(null)
  const [editingRule, setEditingRule] = useState<Row | null>(null)

  useEffect(() => {
    if (open) {
      setQuery('')
      setSelectedIndex(0)
      setResults({categories: [], spending_groups: [], transactions: [], merchant_rules: [], rules: []})
      setError('')
      setLoading(false)
    }
  }, [open])

  // Debounced API search
  useEffect(() => {
    const trimmed = query.trim()
    if (!open || trimmed.length < 2) {
      setError('')
      setResults({categories: [], spending_groups: [], transactions: [], merchant_rules: [], rules: []})
      setLoading(false)
      return
    }
    let active = true
    setLoading(true)
    setError('')
    setResults({categories: [], spending_groups: [], transactions: [], merchant_rules: [], rules: []})
    const timer = setTimeout(() => {
      api('/search?q=' + encodeURIComponent(trimmed))
        .then((res: SearchResults) => {
          if (!active) return
          setResults({
            categories: res.categories || [],
            spending_groups: res.spending_groups || [],
            transactions: res.transactions || [],
            merchant_rules: res.merchant_rules || [],
            rules: res.rules || [],
          })
          setSelectedIndex(0)
        })
        .catch((err:Error) => {
          if (!active) return
          setError(err.message)
          notify(err.message,true)
        })
        .finally(() => { if (active) setLoading(false) })
    }, 180)
    return () => { active = false; clearTimeout(timer) }
  }, [open,query,retry])

  // Flatten into a single list grouped by section
  const flatItems: FlatItem[] = []

  for (const c of results.categories) {
    flatItems.push({
      id: `cat-${c.id}`, section: 'Categories',
      label: c.name,
      detail: c.kind === 'expense' ? 'Expense' : 'Income',
      onSelect: () => { onClose(); setEditingCategory(c) },
    })
  }

  for (const g of results.spending_groups) {
    flatItems.push({
      id: `group-${g.id}`, section: 'Spending groups',
      label: g.name,
      groupDot: g.color,
      onSelect: () => { onClose(); setEditingGroup(g) },
    })
  }

  for (const t of results.transactions) {
    flatItems.push({
      id: `tx-${t.id}`, section: 'Transactions',
      label: t.description,
      detail: [t.date, t.account_name, t.category_name || 'Needs category'].filter(Boolean).join(' · '),
      amount: t.amount_cents,
      merchantName: t.merchant_name || undefined,
      merchantLogo: t.merchant_logo || undefined,
      onSelect: () => { onClose(); openTransaction(t.id) },
    })
  }

  for (const m of results.merchant_rules) {
    flatItems.push({
      id: `mrule-${m.id}`, section: 'Merchant rules',
      label: m.merchant_name,
      detail: [m.pattern ? `"${m.pattern}"` : '', m.category_name, m.account_name || 'All accounts'].filter(Boolean).join(' · '),
      merchantName: m.merchant_name,
      merchantLogo: m.merchant_logo || undefined,
      onSelect: () => { onClose(); setEditingMerchantRule(m) },
    })
  }

  for (const r of results.rules) {
    flatItems.push({
      id: `rule-${r.id}`, section: 'Rules',
      label: `"${r.pattern}"`,
      detail: [r.category_name || 'Unassigned', r.account_name || 'All accounts', r.builtin ? 'Built-in' : ''].filter(Boolean).join(' · '),
      onSelect: () => { onClose(); setEditingRule(r) },
    })
  }

  // Keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (flatItems.length === 0) return
      setSelectedIndex(prev => (prev + 1) % flatItems.length)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (flatItems.length === 0) return
      setSelectedIndex(prev => (prev - 1 + flatItems.length) % flatItems.length)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      const selected = flatItems[selectedIndex]
      if (selected) selected.onSelect()
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onClose()
    }
  }

  // Scroll active item into view
  useEffect(() => {
    if (!listRef.current) return
    const el = listRef.current.querySelector(`[data-index="${selectedIndex}"]`)
    if (el) el.scrollIntoView({block: 'nearest'})
  }, [selectedIndex])

  const hasQuery = query.trim().length >= 2

  // Group items by section for rendering with headers
  const sections: {name: string; items: (FlatItem & {globalIndex: number})[]}[] = []
  let lastSection = ''
  flatItems.forEach((item, idx) => {
    if (item.section !== lastSection) {
      sections.push({name: item.section, items: []})
      lastSection = item.section
    }
    sections[sections.length - 1].items.push({...item, globalIndex: idx})
  })

  return <>
    {open && <Modal title="Search workspace" onClose={onClose}>
        <div className="global-search-container">
          <div className="global-search-header">
            <div className="global-search-input-box">
              <Search size={15} className="search-icon-decor" aria-hidden="true" />
              <input
                ref={inputRef}
                autoFocus
                data-autofocus="true"
                role="combobox"
                aria-label="Search workspace"
                aria-expanded={hasQuery&&!loading&&!error}
                onKeyDown={handleKeyDown}
                type="text"
                className="global-search-input"
                placeholder="Search…"
                value={query}
                onChange={e => setQuery(e.target.value)}
                aria-autocomplete="list"
                aria-controls={resultId}
                aria-activedescendant={!loading&&!error&&flatItems[selectedIndex]?resultId+flatItems[selectedIndex].id:undefined}
              />
              {query && (
                <Button variant="quiet"
                  type="button"
                  className="search-clear-btn"
                  onClick={() => { setQuery(''); inputRef.current?.focus() }}
                  aria-label="Clear search"
                >
                  <X size={14} />
                </Button>
              )}
            </div>
          </div>

          <div id={resultId} ref={listRef} className="global-search-body" role="listbox" aria-label="Search results" aria-busy={loading}>
            {!hasQuery ? (
              <div className="search-empty-state">
                <p className="search-shortcut-hint">Search categories, groups, transactions and rules.</p>
              </div>
            ) : loading ? (
              <div className="search-empty-state">
                <Loading>Searching…</Loading>
              </div>
            ) : error ? (
              <div className="search-empty-state"><p>Search could not be completed.</p><Button onClick={()=>setRetry(v=>v+1)}>Retry search</Button></div>
            ) : flatItems.length === 0 ? (
              <div className="search-empty-state">
                <p role="status" className="search-shortcut-hint">No results for "{query}"</p>
              </div>
            ) : (
              sections.map(section => (
                <div key={section.name} className="search-section">
                  <div className="search-section-label">{section.name}</div>
                  {section.items.map(item => {
                    const isSelected = item.globalIndex === selectedIndex
                    return (
                      <button type="button" tabIndex={-1}
                        key={item.id}
                        id={resultId+item.id}
                        data-index={item.globalIndex}
                        role="option"
                        aria-selected={isSelected}
                        className={'search-item' + (isSelected ? ' selected' : '')}
                        onClick={item.onSelect}
                        onMouseEnter={() => setSelectedIndex(item.globalIndex)}
                      >
                        {item.merchantName && (
                          <MerchantAvatar name={item.merchantName} logo={item.merchantLogo} />
                        )}
                        {item.groupDot && !item.merchantName && (
                          <GroupDot color={item.groupDot} />
                        )}
                        <div className="search-item-content">
                          <span className="search-item-title">{item.label}</span>
                          {item.detail && <span className="search-item-sub">{item.detail}</span>}
                        </div>
                        {item.amount !== undefined && (
                          <strong className={item.amount < 0 ? 'negative-amount' : 'positive-amount'}>
                            {money(item.amount)}
                          </strong>
                        )}
                      </button>
                    )
                  })}
                </div>
              ))
            )}
          </div>
        </div>
      </Modal>}

    {editingCategory && (
      <CategoryEditor
        category={editingCategory}
        notify={notify}
        onClose={() => setEditingCategory(null)}
        onDone={() => { setEditingCategory(null); refresh() }}
      />
    )}

    {editingGroup && (
      <SpendingGroupEditor
        group={editingGroup}
        notify={notify}
        onClose={() => setEditingGroup(null)}
        onDone={() => { setEditingGroup(null); refresh() }}
      />
    )}

    {editingMerchantRule && (
      <MerchantRuleEditor
        rule={editingMerchantRule}
        data={data}
        notify={notify}
        refresh={refresh}
        onClose={() => setEditingMerchantRule(null)}
        onDone={() => { setEditingMerchantRule(null); refresh() }}
      />
    )}

    {editingRule && (
      <RuleEditor
        rule={editingRule}
        data={data}
        revision={revision}
        notify={notify}
        refresh={refresh}
        onClose={() => setEditingRule(null)}
        onDone={() => { setEditingRule(null); refresh() }}
      />
    )}
  </>
}
