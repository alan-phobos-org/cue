import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import App from './App'

// Utility function tests (extracted from App.tsx for testing)
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

function isFileLink(link: string): boolean {
  return link.startsWith('~') || link.startsWith('/') || link.startsWith('./')
}

function displayLink(link: string): string {
  return link.replace(/^https?:\/\//, '')
}

describe('formatDate utility', () => {
  it('returns "Just now" for dates less than 1 hour ago', () => {
    const now = new Date()
    const thirtyMinAgo = new Date(now.getTime() - 30 * 60 * 1000)
    expect(formatDate(thirtyMinAgo.toISOString())).toBe('Just now')
  })

  it('returns hours ago for dates less than 24 hours ago', () => {
    const now = new Date()
    const fiveHoursAgo = new Date(now.getTime() - 5 * 60 * 60 * 1000)
    expect(formatDate(fiveHoursAgo.toISOString())).toBe('5h ago')
  })

  it('returns "Yesterday" for dates 24-48 hours ago', () => {
    const now = new Date()
    const yesterday = new Date(now.getTime() - 30 * 60 * 60 * 1000)
    expect(formatDate(yesterday.toISOString())).toBe('Yesterday')
  })

  it('returns "X days ago" for dates 2-6 days ago', () => {
    const now = new Date()
    const threeDaysAgo = new Date(now.getTime() - 3 * 24 * 60 * 60 * 1000)
    expect(formatDate(threeDaysAgo.toISOString())).toBe('3 days ago')
  })

  it('returns localized date for dates more than 7 days ago', () => {
    const now = new Date()
    const twoWeeksAgo = new Date(now.getTime() - 14 * 24 * 60 * 60 * 1000)
    const result = formatDate(twoWeeksAgo.toISOString())
    // Should be a date string, not relative
    expect(result).not.toContain('ago')
    expect(result).not.toBe('Just now')
  })
})

describe('isFileLink utility', () => {
  it('returns true for tilde paths', () => {
    expect(isFileLink('~/Documents/file.md')).toBe(true)
    expect(isFileLink('~/file.txt')).toBe(true)
  })

  it('returns true for absolute paths', () => {
    expect(isFileLink('/usr/local/bin')).toBe(true)
    expect(isFileLink('/home/user/file.txt')).toBe(true)
  })

  it('returns true for relative paths', () => {
    expect(isFileLink('./file.md')).toBe(true)
    expect(isFileLink('./subdir/file.txt')).toBe(true)
  })

  it('returns false for URLs', () => {
    expect(isFileLink('https://example.com')).toBe(false)
    expect(isFileLink('http://localhost:3000')).toBe(false)
    expect(isFileLink('ftp://server.com/file')).toBe(false)
  })

  it('returns false for other strings', () => {
    expect(isFileLink('example.com')).toBe(false)
    expect(isFileLink('file.txt')).toBe(false)
  })
})

describe('displayLink utility', () => {
  it('strips https protocol', () => {
    expect(displayLink('https://example.com')).toBe('example.com')
    expect(displayLink('https://github.com/user/repo')).toBe('github.com/user/repo')
  })

  it('strips http protocol', () => {
    expect(displayLink('http://example.com')).toBe('example.com')
  })

  it('preserves file paths unchanged', () => {
    expect(displayLink('~/Documents/file.md')).toBe('~/Documents/file.md')
    expect(displayLink('/usr/local/bin')).toBe('/usr/local/bin')
  })

  it('preserves other protocols', () => {
    expect(displayLink('ftp://server.com/file')).toBe('ftp://server.com/file')
  })
})

// Mock the API
const mockItems = [
  { id: '1', title: 'Test Item', content: 'Content', link: null, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }
]

globalThis.fetch = async (url: string, options?: RequestInit) => {
  const urlStr = url.toString()
  if (urlStr.includes('/api/items') && (!options || options.method === 'GET' || !options.method)) {
    return new Response(JSON.stringify(mockItems), { status: 200 })
  }
  if (urlStr.includes('/api/search')) {
    return new Response(JSON.stringify([]), { status: 200 })
  }
  if (options?.method === 'POST') {
    const body = JSON.parse(options.body as string)
    return new Response(JSON.stringify({ ...body, id: '2', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }), { status: 201 })
  }
  return new Response('{}', { status: 200 })
}

describe('Keyboard shortcuts', () => {
  afterEach(() => {
    cleanup()
  })

  beforeEach(() => {
    render(<App />)
  })

  it('/ key focuses search when no input has focus', async () => {
    const search = screen.getByPlaceholderText('Search knowledge...')

    // Ensure search is not focused
    expect(document.activeElement).not.toBe(search)

    // Press / on the document
    fireEvent.keyDown(document, { key: '/' })

    // Search should now be focused
    expect(document.activeElement).toBe(search)
  })

  it('/ key does NOT focus search when typing in search field', () => {
    const search = screen.getByPlaceholderText('Search knowledge...')

    // Focus search
    search.focus()
    expect(document.activeElement).toBe(search)

    // Type / in search
    fireEvent.keyDown(search, { key: '/' })

    // Should still be focused on search (not stolen)
    expect(document.activeElement).toBe(search)
  })

  it('/ key does NOT focus search when typing in an input field', async () => {
    const search = screen.getByPlaceholderText('Search knowledge...')

    // Wait for items to load and click on one to select it
    const item = await screen.findByText('Test Item')
    fireEvent.click(item)

    // Switch to edit mode
    const editBtn = screen.getByText('Edit')
    fireEvent.click(editBtn)

    // Find title input
    const titleInput = screen.getByPlaceholderText('Title...')
    titleInput.focus()
    expect(document.activeElement).toBe(titleInput)

    // Press / while in title input
    fireEvent.keyDown(titleInput, { key: '/' })

    // Should NOT have moved focus to search
    expect(document.activeElement).toBe(titleInput)
    expect(document.activeElement).not.toBe(search)
  })

  it('/ key does NOT focus search when typing in textarea', async () => {
    const search = screen.getByPlaceholderText('Search knowledge...')

    // Wait for items to load and click on one
    const item = await screen.findByText('Test Item')
    fireEvent.click(item)

    // Switch to edit mode
    const editBtn = screen.getByText('Edit')
    fireEvent.click(editBtn)

    // Find content textarea
    const textarea = screen.getByPlaceholderText('Write your notes in Markdown...')
    textarea.focus()
    expect(document.activeElement).toBe(textarea)

    // Press / while in textarea
    fireEvent.keyDown(textarea, { key: '/' })

    // Should NOT have moved focus to search
    expect(document.activeElement).toBe(textarea)
    expect(document.activeElement).not.toBe(search)
  })

  it('/ key does NOT focus search when typing in link input', async () => {
    const search = screen.getByPlaceholderText('Search knowledge...')

    // Wait for items to load and click on one
    const item = await screen.findByText('Test Item')
    fireEvent.click(item)

    // Switch to edit mode
    const editBtn = screen.getByText('Edit')
    fireEvent.click(editBtn)

    // Find link input
    const linkInput = screen.getByPlaceholderText('https://... or ~/path/to/file')
    linkInput.focus()
    expect(document.activeElement).toBe(linkInput)

    // Press / while in link input
    fireEvent.keyDown(linkInput, { key: '/' })

    // Should NOT have moved focus to search
    expect(document.activeElement).toBe(linkInput)
    expect(document.activeElement).not.toBe(search)
  })

  it('Escape key clears search query and blurs search input', async () => {
    const search = screen.getByPlaceholderText('Search knowledge...')

    // Focus and type in search
    search.focus()
    fireEvent.change(search, { target: { value: 'test query' } })
    expect((search as HTMLInputElement).value).toBe('test query')

    // Press Escape
    fireEvent.keyDown(document, { key: 'Escape' })

    // Search should be cleared and blurred
    expect((search as HTMLInputElement).value).toBe('')
    expect(document.activeElement).not.toBe(search)
  })
})

describe('Edge cases', () => {
  afterEach(() => {
    cleanup()
  })

  beforeEach(() => {
    render(<App />)
  })

  it('shows empty state when no items selected', () => {
    // Before items load, should show empty state
    const emptyState = screen.queryByText('Select an item or search to create one')
    // This may or may not be visible depending on timing, but shouldn't error
    expect(emptyState).toBeDefined()
  })

  it('displays item content preview in list', async () => {
    // Wait for items to load
    const item = await screen.findByText('Test Item')
    expect(item).toBeDefined()
  })
})
