import { test, expect } from '@playwright/test'

test.describe('Keyboard shortcuts', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
  })

  test('/ key focuses search when no input has focus', async ({ page }) => {
    const search = page.locator('.search-input')

    // Click elsewhere to ensure no input has focus
    await page.locator('.app').click()

    // Press /
    await page.keyboard.press('/')

    // Search should be focused
    await expect(search).toBeFocused()
  })

  test('/ key types into link input field instead of focusing search', async ({ page }) => {
    const search = page.locator('.search-input')

    // Create an item first so we can edit it
    await search.fill('Test Item')
    await page.locator('.create-prompt').click()

    // Now in edit mode, find the link input
    const linkInput = page.locator('.link-input')
    await expect(linkInput).toBeVisible()

    // Focus the link input and type a URL with /
    await linkInput.click()
    await linkInput.fill('https:')
    await page.keyboard.press('/')
    await page.keyboard.type('/example.com')

    // Should contain the full URL, not have focus stolen
    await expect(linkInput).toHaveValue('https://example.com')
    await expect(linkInput).toBeFocused()
    await expect(search).not.toBeFocused()
  })

  test('/ key types into content textarea instead of focusing search', async ({ page }) => {
    const search = page.locator('.search-input')

    // Create an item
    await search.fill('Test Item')
    await page.locator('.create-prompt').click()

    // Find the textarea
    const textarea = page.locator('.editor-content')
    await expect(textarea).toBeVisible()

    // Focus and type content with /
    await textarea.click()
    await textarea.fill('path: ')
    await page.keyboard.press('/')
    await page.keyboard.type('usr/bin')

    // Should contain the slash
    await expect(textarea).toHaveValue('path: /usr/bin')
    await expect(textarea).toBeFocused()
    await expect(search).not.toBeFocused()
  })

  test('/ key types into title input instead of focusing search', async ({ page }) => {
    const search = page.locator('.search-input')

    // Create an item
    await search.fill('Test')
    await page.locator('.create-prompt').click()

    // Find the title input
    const titleInput = page.locator('.content-title-input')
    await expect(titleInput).toBeVisible()

    // Focus and type with /
    await titleInput.click()
    await titleInput.fill('Q')
    await page.keyboard.press('/')
    await page.keyboard.type('A')

    // Should contain the slash
    await expect(titleInput).toHaveValue('Q/A')
    await expect(titleInput).toBeFocused()
  })

  test('/ key works normally in search input itself', async ({ page }) => {
    const search = page.locator('.search-input')

    // Focus search and type with /
    await search.click()
    await search.fill('path')
    await page.keyboard.press('/')
    await page.keyboard.type('to')

    // Should contain the slash
    await expect(search).toHaveValue('path/to')
    await expect(search).toBeFocused()
  })
})
