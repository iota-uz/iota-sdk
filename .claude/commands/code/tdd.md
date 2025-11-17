---
description: "Test-driven development: clarify requirements, write tests, implement code. RUN THIS COMMAND IN PLAN MODE ONLY"
model: sonnet
disable-model-invocation: true
---

You are in **TDD mode** - test-driven development where you write tests first, then implement code to make them pass.

## Workflow

### 1. Understand Requirements

Ask the user what they want to build as an open-ended question. Do not present predefined options.

**Critical**: NEVER make business logic decisions yourself. If requirements are unclear, ask for clarification.

### 2. Write Tests First

After investigating the current codebase state, use AskUserQuestion to clarify business requirements, validation rules, edge cases, and expected behavior. Then generate integration tests following ITF patterns from `.claude/guides/backend/testing.md`.

**Critical Principles:**
- NO mocks for internal services/repositories (use real instances)
- Use ITF's isolated databases (`itf.Setup(t)` or `itf.NewSuiteBuilder(t)`)
- Quality over quantity (3-5 comprehensive tests > 10 shallow tests)
- Happy path first, then 2-3 critical edge cases
- Use `t.Parallel()` for all tests
- Use XPath assertions for HTML validation in controller tests

### 3. Implement Code

Launch `editor` agent to implement the code that makes tests pass.

**Add Test Markers**: When implementing HTML templates, modify `.templ` files to include `data-test-id` and other data attributes on elements that tests need to target. This makes tests more reliable than XPath structure or CSS classes.

Examples:
- `<button data-test-id="submit-button">Submit</button>` - for action elements
- `<div data-test-section="client-details">...</div>` - for content sections
- `<input data-test-field="email" name="email" />` - for form fields
- `<span data-test-value="total-amount">$100</span>` - for dynamic values

Ensure all tests pass when implementation is complete.

### 4. Verify

Run tests to verify implementation. If tests fail, analyze failure output, fix implementation or tests (if requirements were misunderstood), and re-run until all tests pass.

### 5. Iterate

After tests pass, ask user what they want to do next: add more tests, refactor, or finish.

## Key Rules

1. **Trust user instructions** - If existing implementation exists, don't assume it's correct; rely on user requirements
2. **Always clarify business logic** - Use AskUserQuestion for requirements, validation rules, edge cases
3. **Tests first, code second** - Never write implementation before tests
4. **No mocks** - Use real repositories and services (ITF provides isolation)
5. **XPath for controllers** - Use `ExpectElement(xpath)`, `ExpectErrorFor(field)` for HTML validation
6. **Quality over quantity** - Few comprehensive tests > many shallow tests
7. **Reference testing guide** - See `.claude/guides/backend/testing.md` for patterns and APIs
8. **Happy path first** - Write success case, then critical failures
9. **Parallel tests** - Always use `t.Parallel()`
10. **Test markers** - Add `data-test-id` attributes to HTML elements during implementation for reliable test targeting
