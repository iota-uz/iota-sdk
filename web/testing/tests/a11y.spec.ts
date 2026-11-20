import { test, expect } from '@playwright/test'
import { A11yViolationError, expectNoA11yViolations } from '../src/index.js'

const accessible = '<!doctype html><html lang="en"><head><title>Fixture</title></head><body><main><h1>Fixture</h1><button type="button">Save</button></main></body></html>'

// Characterisation: falsely green if the helper ignored critical violations or ran on an empty scope.
test('fails on a button without an accessible name (critical) and names the rule', async ({ page }) => {
  await page.setContent('<!doctype html><html lang="en"><head><title>Fixture</title></head><body><main><h1>Fixture</h1><button type="button" id="unnamed"></button></main></body></html>')
  const error = await expectNoA11yViolations(page).catch(e => e)
  expect(error).toBeInstanceOf(A11yViolationError)
  expect(error.violations.map((violation: { id: string }) => violation.id)).toContain('button-name')
  expect(error.message).toContain('[critical] button-name')
  expect(error.message).toContain('#unnamed')
})

test('passes an accessible page and attaches the full JSON report', async ({ page }, testInfo) => {
  await page.setContent(accessible)
  await expectNoA11yViolations(page)
  const report = testInfo.attachments.find(attachment => attachment.name === 'a11y-report.json')
  expect(report?.contentType).toBe('application/json')
  expect(JSON.parse(report!.body!.toString())).toHaveProperty('violations')
})

test('include, exclude and disableRules narrow the check', async ({ page }) => {
  await page.setContent('<!doctype html><html lang="en"><head><title>Fixture</title></head><body><main><h1>Fixture</h1><section id="ok"><button type="button">Save</button></section><section id="broken"><button type="button"></button></section></main></body></html>')
  await expectNoA11yViolations(page, { include: ['#ok'] })
  await expectNoA11yViolations(page, { exclude: ['#broken'] })
  // The fixture's unnamed button is the subject of the rule being switched off.
  await expectNoA11yViolations(page, { disableRules: ['button-name'] })
  await expect(expectNoA11yViolations(page, { include: ['#broken'] })).rejects.toThrow(A11yViolationError)
})

test('moderate and minor findings are reported but do not fail', async ({ page }, testInfo) => {
  // Two main landmarks is a moderate "landmark-no-duplicate-main" finding.
  await page.setContent('<!doctype html><html lang="en"><head><title>Fixture</title></head><body><main><h1>One</h1></main><main><p>Two</p></main></body></html>')
  await expectNoA11yViolations(page)
  const report = JSON.parse(testInfo.attachments.find(attachment => attachment.name === 'a11y-report.json')!.body!.toString())
  expect(report.violations.length).toBeGreaterThan(0)
  expect(report.violations.every((violation: { impact: string }) => !['serious', 'critical'].includes(violation.impact))).toBe(true)
})
