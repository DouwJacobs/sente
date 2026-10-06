import {useState,useEffect,useRef} from 'react'
import {createPortal} from 'react-dom'
import {Search,X} from 'lucide-react'
import {api,money} from './api'
import {GroupDot} from './Choices'
import {MerchantAvatar} from './MerchantAvatar'
import {CategoryEditor,MerchantRuleEditor} from './Organisation'
import {RuleEditor} from './Rules'
import type {Row,PageProps} from './App'
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
  const dialogRef = useRef<HTMLDialogElement>(null)

  // Specific entity editors opened from search
  const [editingCategory, setEditingCategory] = useState<Row | null>(null)
  const [editingGroup, setEditingGroup] = useState<Row | null>(null)
  const [editingMerchantRule, setEditingMerchantRule] = useState<Row | null>(null)
  const [editingRule, setEditingRule] = useState<Row | null>(null)

  useEffect(() => {
    if (open) {
      dialogRef.current?.showModal()
      setQuery('')
      setSelectedIndex(0)
      setResults({categories: [], spending_groups: [], transactions: [], merchant_rules: [], rules: []})
      setTimeout(() => inputRef.current?.focus(), 30)
    } else {
      dialogRef.current?.close()
    }
  }, [open])

  // Debounced API search
  useEffect(() => {
    const trimmed = query.trim()
    if (trimmed.length < 2) {
      setResults({categories: [], spending_groups: [], transactions: [], merchant_rules: [], rules: []})
      setLoading(false)
      return
    }
    let active = true
    setLoading(true)
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
        .catch(() => {
          if (!active) return
          setResults({categories: [], spending_groups: [], transactions: [], merchant_rules: [], rules: []})
        })
        .finally(() => { if (active) setLoading(false) })
    }, 180)
    return () => { active = false; clearTimeout(timer) }
  }, [query])

  // Flatten into a single list grouped by section
  const flatItems: FlatItem[] = []

  for (const c of results.categories) {
    flatItems.push({
      id: `cat-${c.id}`, section: 'Categories',
      label: c.name,
      detail: [c.kind === 'expense' ? 'Expense' : 'Income', c.spending_group_name].filter(Boolean).join(' · '),
      groupDot: c.spending_group_color || undefined,
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
    {open && createPortal(
      <dialog
        ref={dialogRef}
        className="global-search-modal"
        aria-label="Search"
        onCancel={e => { e.preventDefault(); onClose() }}
        onClick={e => { if (e.target === dialogRef.current) onClose() }}
      >
        <div className="global-search-container" onKeyDown={handleKeyDown}>
          <div className="global-search-header">
            <div className="global-search-input-box">
              <Search size={15} className="search-icon-decor" aria-hidden="true" />
              <input
                ref={inputRef}
                type="text"
                className="global-search-input"
                placeholder="Search…"
                value={query}
                onChange={e => setQuery(e.target.value)}
                aria-autocomplete="list"
                aria-controls="global-search-results"
                aria-activedescendant={flatItems[selectedIndex]?.id}
              />
              {query && (
                <button
                  type="button"
                  className="search-clear-btn"
                  onClick={() => { setQuery(''); inputRef.current?.focus() }}
                  aria-label="Clear"
                >
                  <X size={14} />
                </button>
              )}
            </div>
          </div>

          <div id="global-search-results" ref={listRef} className="global-search-body" role="listbox">
            {!hasQuery ? (
              <div className="search-empty-state">
                <p className="search-shortcut-hint">Search categories, groups, transactions and rules.</p>
              </div>
            ) : loading ? (
              <div className="search-empty-state">
                <p className="search-shortcut-hint">Searching…</p>
              </div>
            ) : flatItems.length === 0 ? (
              <div className="search-empty-state">
                <p className="search-shortcut-hint">No results for "{query}"</p>
              </div>
            ) : (
              sections.map(section => (
                <div key={section.name} className="search-section">
                  <div className="search-section-label">{section.name}</div>
                  {section.items.map(item => {
                    const isSelected = item.globalIndex === selectedIndex
                    return (
                      <div
                        key={item.id}
                        id={item.id}
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
                      </div>
                    )
                  })}
                </div>
              ))
            )}
          </div>
        </div>
      </dialog>,
      document.body
    )}

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

function SpendingGroupEditor({group, notify, onClose, onDone}: {group: Row; notify: PageProps['notify']; onClose: () => void; onDone: () => void}) {
  const [name, setName] = useState(group.name)
  const [color, setColor] = useState(group.color)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  return createPortal(
    <dialog
      className="modal-dialog"
      ref={el => { if (el && !el.open) el.showModal() }}
      onCancel={e => { e.preventDefault(); onClose() }}
    >
      <div className="modal-panel">
        <div className="modal-head">
          <h2>Edit spending group · {group.name}</h2>
          <button type="button" className="modal-close" aria-label="Close" onClick={onClose}><X size={18} /></button>
        </div>
        <form className="form" onSubmit={async e => {
          e.preventDefault()
          if (busy) return
          setBusy(true)
          try {
            await api('/spending-groups/' + group.id, 'PUT', {name, color, version: group.version})
            notify('Spending group saved')
            onDone()
          } catch (err) {
            const message = (err as Error).message
            if (message === 'Spending group already exists') setError(message)
            else notify(message, true)
          } finally {
            setBusy(false)
          }
        }}>
          <div className="field">
            <label className="field-label">Name</label>
            <input className="field-input" autoFocus required minLength={2} maxLength={80} value={name} onChange={e => { setName(e.target.value); setError('') }} />
            {error && <span className="field-error" role="alert">{error}</span>}
          </div>
          <div className="field">
            <label className="field-label">Color</label>
            <select className="field-input" value={color} onChange={e => setColor(e.target.value)}>
              {['blue', 'amber', 'purple', 'orange', 'teal', 'slate', 'rose'].map(c => (
                <option value={c} key={c}>{c[0].toUpperCase() + c.slice(1)}</option>
              ))}
            </select>
          </div>
          <div className="editor-actions">
            <button type="submit" className="button primary" disabled={busy}>{busy ? 'Saving…' : 'Save spending group'}</button>
            <button type="button" className="button secondary" onClick={onClose}>Cancel</button>
          </div>
        </form>
      </div>
    </dialog>,
    document.body
  )
}
