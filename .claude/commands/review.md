Run `git status` and `git diff --cached` & conduct a comprehensive code review

**Your Review Scope:**
- Analyze the entire package holistically, not just isolated files
- Focus on the changes in this PR, but consider their impact on the broader codebase
- Review architecture, code quality, security, performance, and maintainability
- Perform comprehensive security analysis including OWASP Top 10 vulnerabilities
- Check for secure coding practices and potential attack vectors
- Conduct semantic linting: check spelling, grammar, word choice, and terminology consistency
- Verify clear and professional technical communication in comments and documentation

**Feedback Categories:**
Provide feedback in exactly 3 categories with these specific emojis:

**Critical ❌** - Issues that MUST be fixed before merge:
- Security vulnerabilities (SQL injection, XSS, CSRF, insecure crypto, exposed secrets)
- Authentication/authorization bypass
- Input validation failures
- Potential panics or crashes
- Breaking changes to public APIs
- Data corruption risks
- Memory leaks
- Race conditions

**Minor 🟡** - Important improvements that should be addressed:
- Performance inefficiencies
- Error handling improvements
- Design pattern violations
- Missing tests for critical paths
- Documentation gaps for public APIs
- Potential bugs that don't cause crashes
- Spelling mistakes and typos in comments, docs, and strings
- Grammar and language clarity issues
- Inconsistent terminology usage

**Nits 🟢** - Style and best practice suggestions:
- Code style inconsistencies
- Naming improvements
- Minor refactoring opportunities
- Comment clarity and readability
- Test coverage for edge cases
- Word choice and semantic clarity improvements
- Technical writing style enhancements
- Consistent use of technical terms and domain language

**Review Principles to Apply:**
1. **Interfaces**: Keep interfaces minimal and consumer-defined; accept interfaces, return concrete types
2. **Dependencies**: Inject through constructors or functional options, never global vars or `init()`
3. **Composition**: Prefer struct embedding/composition over deep type hierarchies
4. **Error Handling**: Handle errors early, no hidden panics
5. **SOLID/GRASP**: Apply where they naturally map to Go idioms
6. **Dead Code**: Identify and flag unused functions or dead code
7. **DDD Architecture**: Ensure proper layer separation (domain, infrastructure, services, presentation)
8. **Go Idioms**: Follow standard Go conventions and best practices

**Output Format:**
Structure your review as follows:
```
## Code Review Summary

Brief overview of the changes and overall assessment.

### Critical ❌
- [Specific issue with file:line reference]
- [Another critical issue]

### Minor 🟡  
- [Important improvement needed]
- [Another minor issue]

### Nits 🟢
- [Style or best practice suggestion]
- [Another nit]

## Architecture Notes
[Any observations about DDD layer violations, design patterns, or architectural concerns]

## Security Considerations
[Security analysis covering: input validation, authentication/authorization, cryptography usage, secret management, SQL injection prevention, XSS/CSRF protection, and secure coding practices]

## Performance Notes
[Any performance-related observations]

## Language & Documentation Quality
[Spelling, grammar, terminology consistency, and technical writing quality assessment]
```

**Important**: 
- Always include file:line references for specific issues
- If no issues exist in a category, write "None identified"
- Focus on actionable feedback
- Be constructive and educational in your comments

