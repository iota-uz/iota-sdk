---
layout: default
title: Releases
parent: Getting Started
nav_order: 2
---

# Releases

This guide covers the IOTA SDK release process and versioning.

## Release Workflow

IOTA SDK uses automated GitHub Actions to manage releases. Follow these steps to create a new release:

### 1. Navigate to Release Workflow

Go to the [Release Workflow](https://github.com/iota-uz/iota-sdk/actions/workflows/release.yml) in GitHub Actions.

### 2. Trigger New Release

Click the **Run workflow** button.

### 3. Specify Release Details

Provide the following information:

- **Version Number**: The semantic version for the release (e.g., `1.0.0`, `1.2.3`)
- **Release Notes**: Detailed changelog describing changes, features, fixes, and breaking changes

### 4. Automatic Publishing

The workflow will automatically:

- Create a new GitHub release with the specified version
- Generate release artifacts
- Publish artifacts to the [release page](https://github.com/iota-uz/iota-sdk/releases)

## Versioning

IOTA SDK follows [Semantic Versioning](https://semver.org/):

- **MAJOR** version for incompatible API changes
- **MINOR** version for backward-compatible functionality additions
- **PATCH** version for backward-compatible bug fixes

### Version Format

```
MAJOR.MINOR.PATCH
```

**Examples**:
- `1.0.0` - Initial release
- `1.1.0` - New features added
- `1.1.1` - Bug fix
- `2.0.0` - Breaking changes

## Release Notes

When creating a release, include comprehensive release notes with:

### Format

```markdown
## Features
- Description of new features
- Breaking changes prominently noted

## Enhancements
- Improvements to existing functionality
- Performance improvements

## Bug Fixes
- Bugs fixed in this release

## Deprecations (if any)
- Features deprecated in this release
- Migration paths for users

## Dependencies
- Updated dependency versions
- Compatibility information

## Known Issues (if any)
- Known limitations
- Workarounds if available
```

### Example

```markdown
## Features
- New financial reporting module with customizable reports
- Multi-currency support in payments and transactions

## Enhancements
- Improved performance of inventory queries by 40%
- Enhanced CRM dashboard with real-time updates

## Bug Fixes
- Fixed tenant isolation issue in transaction queries
- Corrected decimal precision in financial calculations

## Deprecations
- Deprecated old payment API (use new payment service)
- Deprecated UserService.FindByEmailAndPassword() (use SessionService)

## Dependencies
- Updated PostgreSQL driver to v1.10.0
- Upgraded Templ to v0.3.900

## Known Issues
- GraphQL subscriptions may timeout on large datasets (workaround: use polling)
```

## Release Checklist

Before running the release workflow, ensure:

- [ ] All code changes are committed and pushed
- [ ] All tests pass: `make test`
- [ ] Code is properly formatted: `make fix fmt && make fix imports`
- [ ] Linting passes: `make check lint`
- [ ] Documentation is updated
- [ ] CHANGELOG is prepared
- [ ] No uncommitted files exist: `git status`
- [ ] You're on the correct branch (usually `main`)

## Post-Release

After the release is published:

1. **Verify Release**: Check the [GitHub Releases page](https://github.com/iota-uz/iota-sdk/releases)
2. **Test Artifacts**: Download and verify release artifacts
3. **Update Documentation**: Update version references if needed
4. **Announce Release**: Share release notes with community/users

## Continuous Integration

The release workflow includes:

- **Build Verification**: Ensures code builds successfully
- **Test Execution**: Runs full test suite
- **Artifact Generation**: Creates release artifacts
- **Publishing**: Publishes to GitHub Releases

## Accessing Releases

View all releases at: [github.com/iota-uz/iota-sdk/releases](https://github.com/iota-uz/iota-sdk/releases)

Each release includes:
- Release notes
- Source code (zip and tar.gz)
- Build artifacts (if applicable)
- Release date and author information

## Troubleshooting

### Release Workflow Fails

1. Check workflow logs in GitHub Actions
2. Verify all tests pass locally: `make test`
3. Ensure go modules are up to date: `go mod tidy`
4. Try again with correct version format

### Version Already Exists

Create a new version using semantic versioning (e.g., increment patch/minor/major).

### Updating a Release

GitHub allows editing release notes after creation:

1. Go to the release on the releases page
2. Click the edit button
3. Update release notes
4. Save changes

---

For more information, visit the [GitHub Releases Documentation](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository).
