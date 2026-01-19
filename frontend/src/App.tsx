import { useState, useEffect, useCallback, useRef } from 'react'
import { api, Item, SearchResult, WhoAmI, AuthRequiredError } from './api/client'
import { TokenSettings } from './components/TokenSettings'
import { AuthError } from './components/AuthError'

type Mode = 'preview' | 'edit'

function formatDate(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const hours = Math.floor(diff / (1000 * 60 * 60))
  const days = Math.floor(hours / 24)

  if (hours < 1) return 'Just now'
  if (hours < 24) return `${hours}h ago`
  if (days === 1) return 'Yesterday'
  if (days < 7) return `${days} days ago`
  return date.toLocaleDateString()
}

function formatFullDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric', month: 'short', day: 'numeric'
  })
}

function LinkIcon({ isFile }: { isFile: boolean }) {
  if (isFile) {
    return (
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/>
        <polyline points="13 2 13 9 20 9"/>
      </svg>
    )
  }
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/>
      <polyline points="15 3 21 3 21 9"/>
      <line x1="10" y1="14" x2="21" y2="3"/>
    </svg>
  )
}

function SearchIcon() {
  return (
    <svg className="search-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="11" cy="11" r="8"/>
      <path d="m21 21-4.35-4.35"/>
    </svg>
  )
}

function isFileLink(link: string): boolean {
  return link.startsWith('~') || link.startsWith('/') || link.startsWith('./')
}

// Detect if user is on macOS for keyboard shortcut display
function isMac(): boolean {
  return typeof navigator !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.platform)
}

const modKey = isMac() ? 'Cmd' : 'Ctrl'

function displayLink(link: string): string {
  // Strip protocol for display
  return link.replace(/^https?:\/\//, '')
}

function SettingsIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="12" cy="12" r="3"/>
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>
    </svg>
  )
}

function App() {
  const [items, setItems] = useState<Item[]>([])
  const [searchResults, setSearchResults] = useState<SearchResult[] | null>(null)
  const [selectedItem, setSelectedItem] = useState<Item | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [mode, setMode] = useState<Mode>('preview')
  const [editTitle, setEditTitle] = useState('')
  const [editContent, setEditContent] = useState('')
  const [editLink, setEditLink] = useState('')
  const [saving, setSaving] = useState(false)
  const [creating, setCreating] = useState(false)
  const [authState, setAuthState] = useState<WhoAmI | null>(null)
  const [authError, setAuthError] = useState(false)
  const [showTokenSettings, setShowTokenSettings] = useState(false)
  const [serverVersion, setServerVersion] = useState<string | null>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const saveTimeoutRef = useRef<number | null>(null)

  // Check auth state and load items on mount
  useEffect(() => {
    checkAuth()
    fetchVersion()
  }, [])

  const fetchVersion = async () => {
    try {
      const status = await api.status()
      setServerVersion(status.version)
    } catch (err) {
      console.error('Failed to fetch version:', err)
    }
  }

  const checkAuth = async () => {
    try {
      const auth = await api.whoami()
      setAuthState(auth)
      loadItems()
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        setAuthError(true)
      } else {
        console.error('Failed to check auth:', err)
        // Proceed without auth info in dev mode
        loadItems()
      }
    }
  }

  const loadItems = async () => {
    try {
      const data = await api.listItems()
      setItems(data)
      if (data.length > 0 && !selectedItem) {
        selectItem(data[0])
      }
    } catch (err) {
      console.error('Failed to load items:', err)
    }
  }

  const selectItem = (item: Item) => {
    setSelectedItem(item)
    setEditTitle(item.title)
    setEditContent(item.content)
    setEditLink(item.link || '')
    setMode('preview')
    setCreating(false)
  }

  const handleSearch = useCallback(async (query: string) => {
    setSearchQuery(query)
    if (query.trim()) {
      try {
        const results = await api.search(query)
        setSearchResults(results)
      } catch {
        // Invalid search query, show empty results
        setSearchResults([])
      }
    } else {
      setSearchResults(null)
    }
  }, [])

  const createNew = async (title: string) => {
    setSelectedItem(null)
    setEditTitle(title)
    setEditContent('')
    setEditLink('')
    setMode('edit')
    setCreating(true)
    setSearchQuery('')
    setSearchResults(null)
  }

  const saveItem = useCallback(async () => {
    if (!editTitle.trim()) return

    setSaving(true)
    try {
      const link = editLink.trim() || undefined
      if (creating) {
        const item = await api.createItem({ title: editTitle, content: editContent, link })
        setItems(prev => [item, ...prev])
        setSelectedItem(item)
        setCreating(false)
      } else if (selectedItem) {
        const item = await api.updateItem(selectedItem.id, { title: editTitle, content: editContent, link })
        setItems(prev => prev.map(i => i.id === item.id ? item : i))
        setSelectedItem(item)
      }
      setMode('preview')
    } catch (err) {
      console.error('Failed to save:', err)
      alert('Failed to save: ' + err)
    } finally {
      setSaving(false)
    }
  }, [editTitle, editContent, editLink, creating, selectedItem])

  const deleteItem = async () => {
    if (!selectedItem || !confirm('Delete this item?')) return

    try {
      await api.deleteItem(selectedItem.id)
      setItems(prev => prev.filter(i => i.id !== selectedItem.id))
      setSelectedItem(null)
    } catch (err) {
      console.error('Failed to delete:', err)
    }
  }

  const autoSave = useCallback(() => {
    if (saveTimeoutRef.current) {
      clearTimeout(saveTimeoutRef.current)
    }
    saveTimeoutRef.current = window.setTimeout(() => {
      if (mode === 'edit' && !creating && selectedItem) {
        saveItem()
      }
    }, 2000)
  }, [mode, creating, selectedItem, saveItem])

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === '/') {
        const active = document.activeElement
        const isTextInput = active instanceof HTMLInputElement ||
                           active instanceof HTMLTextAreaElement ||
                           (active instanceof HTMLElement && active.isContentEditable)
        if (!isTextInput) {
          e.preventDefault()
          searchRef.current?.focus()
        }
      }
      if (e.key === 'Escape') {
        searchRef.current?.blur()
        setSearchQuery('')
        setSearchResults(null)
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault()
        if (mode === 'edit') saveItem()
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'e') {
        e.preventDefault()
        setMode(m => m === 'preview' ? 'edit' : 'preview')
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [mode, saveItem])

  // Cleanup autosave timeout on unmount
  useEffect(() => {
    return () => {
      if (saveTimeoutRef.current) {
        clearTimeout(saveTimeoutRef.current)
      }
    }
  }, [])

  const displayedItems = searchResults
    ? searchResults.map(r => r.item)
    : items

  const showCreatePrompt = searchQuery.trim() &&
    !displayedItems.some(i => i.title.toLowerCase() === searchQuery.toLowerCase())

  // Show auth error page if auth is required but not provided
  if (authError) {
    return <AuthError />
  }

  return (
    <>
      <div className="ocean-bg" />
      <div className="app">
        {/* Sidebar */}
        <aside className="sidebar">
          <header className="header">
            <div className="header-top">
              <div className="logo">cue</div>
              {authState && (
                <div className="user-info">
                  <span className="user-name">{authState.user?.cn}</span>
                  {authState.mode === 'authenticated' && (
                    <button
                      className="settings-btn"
                      onClick={() => setShowTokenSettings(true)}
                      title="API Tokens"
                    >
                      <SettingsIcon />
                    </button>
                  )}
                </div>
              )}
            </div>
            <div className="search-container">
              <SearchIcon />
              <input
                ref={searchRef}
                type="text"
                className="search-input"
                placeholder="Search knowledge..."
                value={searchQuery}
                onChange={e => handleSearch(e.target.value)}
              />
              <span className="search-shortcut">/</span>
            </div>
          </header>

          <div className="item-list">
            {showCreatePrompt && (
              <div className="create-prompt" onClick={() => createNew(searchQuery)}>
                <div className="create-prompt-label">Create new item</div>
                <div className="create-prompt-title">{searchQuery}</div>
              </div>
            )}

            {displayedItems.length === 0 && !showCreatePrompt && (
              <div style={{ padding: '20px', textAlign: 'center', color: 'var(--text-dim)' }}>
                {searchQuery ? 'No results found' : 'No items yet. Search to create one.'}
              </div>
            )}

            {displayedItems.map(item => (
              <div
                key={item.id}
                className={`item-card ${selectedItem?.id === item.id ? 'active' : ''}`}
                onClick={() => selectItem(item)}
              >
                <div className="item-title">{item.title}</div>
                {item.link ? (
                  <a
                    className="item-link"
                    href={item.link}
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={e => e.stopPropagation()}
                  >
                    <LinkIcon isFile={isFileLink(item.link)} />
                    {displayLink(item.link)}
                  </a>
                ) : (
                  <div className="item-preview">
                    {item.content.slice(0, 100)}
                  </div>
                )}
                <div className="item-meta">
                  <span>Updated {formatDate(item.updatedAt)}</span>
                </div>
              </div>
            ))}
          </div>
        </aside>

        {/* Main content */}
        <main className="main">
          {(selectedItem || creating) ? (
            <>
              <header className="content-header">
                {mode === 'edit' ? (
                  <input
                    type="text"
                    className="content-title-input"
                    value={editTitle}
                    onChange={e => { setEditTitle(e.target.value); autoSave() }}
                    placeholder="Title..."
                    autoFocus={creating}
                  />
                ) : (
                  <h1 className="content-title">{selectedItem?.title}</h1>
                )}
                <div className="content-actions">
                  <div className="mode-toggle">
                    <button
                      className={`mode-btn ${mode === 'preview' ? 'active' : ''}`}
                      onClick={() => setMode('preview')}
                    >
                      Preview
                    </button>
                    <button
                      className={`mode-btn ${mode === 'edit' ? 'active' : ''}`}
                      onClick={() => setMode('edit')}
                    >
                      Edit
                    </button>
                  </div>
                  {mode === 'edit' && (
                    <button className="btn btn-primary" onClick={saveItem} disabled={saving}>
                      {saving ? 'Saving...' : 'Save'}
                    </button>
                  )}
                  {!creating && (
                    <button className="btn btn-danger" onClick={deleteItem}>
                      Delete
                    </button>
                  )}
                </div>
              </header>

              <div className="editor-container">
                {mode === 'edit' ? (
                  <div className="editor">
                    <div className="link-input-row">
                      <span className="link-label">Link</span>
                      <input
                        type="text"
                        className="link-input"
                        value={editLink}
                        onChange={e => { setEditLink(e.target.value); autoSave() }}
                        placeholder="https://... or ~/path/to/file"
                      />
                    </div>
                    <textarea
                      className="editor-content"
                      value={editContent}
                      onChange={e => { setEditContent(e.target.value); autoSave() }}
                      placeholder="Write your notes in Markdown..."
                    />
                  </div>
                ) : (
                  <div className="markdown-preview">
                    {selectedItem?.link && (
                      <div className="primary-link">
                        <LinkIcon isFile={isFileLink(selectedItem.link)} />
                        <a href={selectedItem.link} target="_blank" rel="noopener noreferrer">
                          {selectedItem.link}
                        </a>
                      </div>
                    )}
                    <div dangerouslySetInnerHTML={{ __html: renderMarkdown(selectedItem?.content || '') }} />
                    {selectedItem && (
                      <div className="timestamps">
                        <span>Created: {formatFullDate(selectedItem.createdAt)}</span>
                        <span>Updated: {formatFullDate(selectedItem.updatedAt)}</span>
                      </div>
                    )}
                  </div>
                )}
              </div>

              <div className="status-bar">
                <div className="status-indicator">
                  <span className={`status-dot ${saving ? 'saving' : ''}`} />
                  <span>{saving ? 'Saving...' : 'All changes saved'}</span>
                </div>
                <div className="status-right">
                  <span className="shortcuts"><kbd>{modKey}</kbd> + <kbd>S</kbd> save &middot; <kbd>{modKey}</kbd> + <kbd>E</kbd> edit</span>
                  {serverVersion && <span className="version">v{serverVersion}</span>}
                </div>
              </div>
            </>
          ) : (
            <div className="empty-state">
              <div className="empty-state-icon">📝</div>
              <div className="empty-state-text">Select an item or search to create one</div>
              <div className="empty-state-hint">Press <kbd>/</kbd> to search</div>
            </div>
          )}
        </main>
      </div>

      {/* Token settings modal */}
      {showTokenSettings && (
        <TokenSettings onClose={() => setShowTokenSettings(false)} />
      )}
    </>
  )
}

// Validate URL for safe protocols (prevents javascript: XSS attacks)
function isSafeUrl(url: string): boolean {
  // Allow http, https, and relative/file paths
  if (/^https?:\/\//i.test(url)) return true
  if (/^(\/|\.\/|~|#)/.test(url)) return true
  // Block everything else (javascript:, data:, vbscript:, etc.)
  return false
}

// Sanitize URL for use in href - returns safe URL or empty string
function sanitizeUrl(url: string): string {
  return isSafeUrl(url) ? url : ''
}

// Simple markdown renderer with XSS protection
function renderMarkdown(md: string): string {
  let html = md
    // Escape HTML
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    // Code blocks
    .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>')
    // Inline code
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    // Headers
    .replace(/^### (.+)$/gm, '<h3>$1</h3>')
    .replace(/^## (.+)$/gm, '<h2>$1</h2>')
    .replace(/^# (.+)$/gm, '<h1>$1</h1>')
    // Bold
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    // Italic
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
    // Links - with URL sanitization to prevent javascript: XSS
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_match, text, url) => {
      const safeUrl = sanitizeUrl(url)
      if (safeUrl) {
        return `<a href="${safeUrl}" target="_blank" rel="noopener noreferrer">${text}</a>`
      }
      return text // Just show the text if URL is unsafe
    })
    // Blockquotes
    .replace(/^> (.+)$/gm, '<blockquote>$1</blockquote>')
    // Unordered lists
    .replace(/^- (.+)$/gm, '<li>$1</li>')
    // Paragraphs
    .replace(/\n\n/g, '</p><p>')

  // Wrap in paragraph
  html = '<p>' + html + '</p>'

  // Fix list items
  html = html.replace(/(<li>.*<\/li>)/gs, '<ul>$1</ul>')
  html = html.replace(/<\/ul>\s*<ul>/g, '')

  // Clean up empty paragraphs
  html = html.replace(/<p>\s*<\/p>/g, '')
  html = html.replace(/<p>\s*(<h[123]>)/g, '$1')
  html = html.replace(/(<\/h[123]>)\s*<\/p>/g, '$1')
  html = html.replace(/<p>\s*(<pre>)/g, '$1')
  html = html.replace(/(<\/pre>)\s*<\/p>/g, '$1')
  html = html.replace(/<p>\s*(<ul>)/g, '$1')
  html = html.replace(/(<\/ul>)\s*<\/p>/g, '$1')
  html = html.replace(/<p>\s*(<blockquote>)/g, '$1')
  html = html.replace(/(<\/blockquote>)\s*<\/p>/g, '$1')

  return html
}

export default App
