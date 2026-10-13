import { test, expect } from '@playwright/test'
import path from 'node:path'

test('fresh real workspace offers setup and never returns configured secrets', async ({ page }) => {
  await page.goto('http://127.0.0.1:8091')
  await expect(page.getByRole('button', { name: 'Add time', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'Connect Zoho', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Zoho connection' })
  await expect(dialog.getByLabel('Authorized redirect URI')).toHaveValue(
    'http://127.0.0.1:8091/api/connection/callback',
  )
  await dialog.getByLabel('Client ID').fill('test-client')
  await dialog.getByLabel('Client secret').fill('acceptance-secret-never-return')
  await dialog.getByRole('button', { name: 'Save credentials' }).click()
  await expect(dialog.getByRole('button', { name: 'Connect to Zoho' })).toBeVisible()
  const response = await page.request.get('http://127.0.0.1:8091/api/bootstrap')
  expect(await response.text()).not.toContain('acceptance-secret-never-return')
  await dialog.getByRole('button', { name: 'Close dialog' }).click()
  await page.reload()
  await page.getByRole('button', { name: 'Connect Zoho', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Connect to Zoho' })).toBeVisible()
})

test('draft workflow, report, deliberate submission, and reference separation', async ({
  page,
}) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Your week, at a glance' })).toBeVisible()
  await expect(page.getByText('DEMO', { exact: true })).toBeVisible()
  await expect(page.locator('.entries-panel .entry-row')).toHaveCount(4)
  await expect
    .poll(async () => (await page.locator('.bar').first().boundingBox())?.height || 0)
    .toBeGreaterThan(10)
  await page.screenshot({
    path: path.resolve('../.cache/desktop-preview.png'),
    fullPage: true,
    animations: 'disabled',
  })
  await page.getByRole('button', { name: 'Add time', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Add time entry' })
  await dialog.getByLabel('Duration').fill('1:20')
  await dialog.getByLabel('Project', { exact: true }).selectOption('northstar')
  await dialog.getByLabel('Task', { exact: true }).selectOption('testing')
  await dialog
    .getByLabel('Work notes')
    .fill('Browser acceptance: reviewed the test cases.\nKept notes verbatim.')
  await dialog.getByRole('button', { name: 'Save draft' }).click()
  await expect(dialog).not.toBeVisible()
  await page.reload()
  await expect(
    page.getByText('Browser acceptance: reviewed the test cases.', { exact: false }),
  ).toBeVisible()
  await page.getByRole('button', { name: 'Edit Quality assurance' }).click()
  const edit = page.getByRole('dialog', { name: 'Edit time entry' })
  await edit.getByLabel('Duration').fill('1.5')
  await edit.getByRole('button', { name: 'Add comment' }).click()
  await edit.getByLabel('Comment 1 header').fill('Browser testing')
  await edit
    .getByRole('textbox', { name: 'Comment 1 brief point 1' })
    .fill('Reviewed the test cases')
  await edit.getByRole('button', { name: '+ Add brief point' }).click()
  await edit.getByRole('textbox', { name: 'Comment 1 brief point 2' }).fill('Checked the report')
  await edit.getByLabel('Comment 1 duration in hours').fill('1.5')
  await expect(edit.locator('.comment-preview')).toContainText(
    'Browser testing:\nBrief:\n- Reviewed the test cases\n- Checked the report\nDuration: 1.5',
  )
  await page.getByRole('button', { name: 'Save draft' }).click()
  await page.reload()
  await page.getByRole('button', { name: 'Edit Quality assurance' }).click()
  const again = page.getByRole('dialog', { name: 'Edit time entry' })
  await expect(again.getByLabel('Comment 1 header')).toHaveValue('Browser testing')
  await again
    .getByRole('textbox', { name: 'Comment 1 brief point 2' })
    .fill('Checked the project report')
  await again.getByRole('button', { name: 'Add comment' }).click()
  await again.getByLabel('Comment 2 header').fill('Follow-up')
  await again.getByRole('textbox', { name: 'Comment 2 brief point 1' }).fill('Verified the fix')
  await again.getByLabel('Comment 2 duration in hours').fill('0.5')
  await again.getByRole('button', { name: 'Save draft' }).click()
  await page.getByRole('button', { name: 'Duplicate Quality assurance' }).click()
  await page.getByRole('dialog').getByLabel('Work notes').fill('Temporary duplicate to remove.')
  await page.getByRole('button', { name: 'Save draft' }).click()
  const duplicate = page.locator('.entry-row').filter({ hasText: 'Temporary duplicate to remove.' })
  page.once('dialog', (d) => d.accept())
  await duplicate.getByRole('button', { name: 'Delete Quality assurance' }).click()
  await expect(duplicate).toHaveCount(0)
  await page.getByRole('button', { name: 'Generate report' }).click()
  const report = page.getByRole('dialog', { name: 'Weekly project report' })
  await report.getByLabel('Project', { exact: true }).selectOption('northstar')
  await expect(report.getByLabel('Report preview')).toHaveValue(/Kept notes verbatim\./)
  await expect(report.getByLabel('Report preview')).toHaveValue(/Checked the project report/)
  await expect(report.getByLabel('Report preview')).toHaveValue(
    /Follow-up:\nBrief:\n- Verified the fix\nDuration: 0.5/,
  )
  await expect(report.getByLabel('Report preview')).toHaveValue(/Total: 7:00 hours/)
  await page.context().grantPermissions(['clipboard-read', 'clipboard-write'])
  await report.getByRole('button', { name: 'Copy report' }).click()
  await expect(report.getByRole('button', { name: 'Copied' })).toBeVisible()
  expect(await page.evaluate(() => navigator.clipboard.readText())).toContain('Quality assurance')
  await report.getByRole('button', { name: 'Close dialog' }).click()
  await page.getByRole('button', { name: /^Review \d+ entries$/ }).click()
  const review = page.getByRole('dialog', { name: 'Review your time' })
  await expect(review.getByRole('button', { name: 'Simulate submission' })).toBeDisabled()
  await review.getByText('Simulate sending these entries in demo mode.').click()
  await review.getByRole('button', { name: 'Simulate submission' }).click()
  await expect(review).not.toBeVisible()
  await expect(page.getByText('Draft time this week').locator('..').locator('..')).toContainText(
    '0h',
  )
  await page.getByRole('button', { name: 'Submitted time' }).click()
  await expect(page.getByText('Simulated ID', { exact: false }).first()).toBeVisible()
  await page.getByRole('button', { name: 'Zoho time logs' }).click()
  await expect(page.getByText('Existing Zoho entry, shown for reference.')).toBeVisible()
  await page.reload()
  await expect(page.getByText('Draft time this week').locator('..').locator('..')).toContainText(
    '0h',
  )
})

test('week navigation and mobile layout stay usable', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Next week' }).click()
  await expect(page.getByText('A fresh page for your week')).toBeVisible()
  await page.getByRole('button', { name: 'Today', exact: true }).click()
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(page.getByRole('button', { name: 'Add time', exact: true })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  )
  await page.getByRole('button', { name: 'Add time', exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.getByRole('dialog').getByLabel('Duration').fill('0:45')
  await page.getByRole('dialog').getByLabel('Project', { exact: true }).selectOption('portal')
  await page.getByRole('dialog').getByLabel('Task', { exact: true }).selectOption('onboarding')
  await page
    .getByRole('dialog')
    .getByLabel('Work notes')
    .fill('Small screen check: reviewed onboarding notes.')
  await page.getByRole('dialog').getByRole('button', { name: 'Add comment' }).click()
  await page.getByRole('dialog').getByLabel('Comment 1 header').fill('Mobile review')
  await page
    .getByRole('dialog')
    .getByRole('textbox', { name: 'Comment 1 brief point 1' })
    .fill('Checked the narrow layout')
  await page.getByRole('dialog').getByLabel('Comment 1 duration in hours').fill('0.75')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  )
  await page.getByRole('button', { name: 'Save draft' }).click()
  await expect(page.getByText('Small screen check: reviewed onboarding notes.')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  )
  await page.screenshot({
    path: path.resolve('../.cache/mobile-preview.png'),
    fullPage: true,
    animations: 'disabled',
  })
  await page.getByRole('button', { name: 'Add time', exact: true }).click()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).not.toBeVisible()
})

test('past-week dashboard separates Zoho totals and renders HTML notes as text', async ({
  page,
}) => {
  await page.goto('/')
  await page.route('**/api/week?start=*', async (route) => {
    const response = await route.fetch()
    const week = await response.json()
    const log = (
      id: string,
      projectId: string,
      projectName: string,
      minutes: number,
      billable: boolean,
      notes: string,
    ) => ({
      id,
      date: week.start,
      projectId,
      projectName,
      taskId: 'api',
      taskName: 'API integration',
      userId: 'you',
      minutes,
      billable,
      notes,
      createdAt: '2026-10-01T12:00:00Z',
      approval: 'Approved',
    })
    week.logs = [
      log(
        'past-1',
        'northstar',
        'Northstar platform',
        90,
        true,
        '<p>API work:<br>Brief:<br>- Reviewed &amp; shipped<br>Duration: 1.5</p><script>unsafe()</script>',
      ),
      log(
        'past-2',
        'portal',
        'Client portal',
        30,
        false,
        '<ul><li>Planning</li><li>Review</li></ul>',
      ),
      log('past-3', 'portal', 'Client portal', 60, false, 'Plain notes stay plain.'),
    ]
    await route.fulfill({ response, json: week })
  })
  await page.getByRole('button', { name: 'Previous week' }).click()
  const summary = page.getByRole('region', { name: 'Zoho time summary' })
  await expect(summary).toContainText('3h')
  await expect(summary).toContainText(/2\s*projects/)
  await expect(summary.locator('.submitted-breakdown')).toContainText(
    /Billable\s*1h 30m\s*Non-billable\s*1h 30m/,
  )
  const logs = page.getByRole('region', { name: 'Submitted Zoho logs' })
  await expect(logs.locator('.entry-row')).toHaveCount(3)
  await expect(logs).toContainText(/API work:\s*Brief:\s*- Reviewed & shipped\s*Duration: 1.5/)
  await expect(logs).toContainText(/- Planning\s*- Review/)
  await expect(logs).not.toContainText('unsafe()')
  await expect(logs.locator('script')).toHaveCount(0)
  await page.getByRole('combobox', { name: 'Filter by project' }).selectOption('portal')
  await expect(summary).toContainText('1h 30m')
  await expect(summary).toContainText(/1\s*projects/)
  await expect(logs.locator('.entry-row')).toHaveCount(2)
})
