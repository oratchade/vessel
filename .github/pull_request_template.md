# Pull Request Template

## Description

Brief description of the changes and their purpose.

## Conventional Commits — Cheat Sheet

### **Types**

- **feat:** new feature
- **fix:** bug fix
- **docs:** documentation only
- **style:** formatting only (no logic)
- **refactor:** code restructuring (no behavior change)
- **perf:** performance improvements
- **test:** add or update tests
- **build:** build system or dependencies
- **ci:** CI configuration changes
- **chore:** maintenance tasks (no src or test changes)
- **revert:** undo a previous commit

### **Extras**

- **Scopes:** `feat(auth):`, `fix(ui):`
- **Breaking change:** `feat!:`, `refactor!:`
- **Issue reference:** `fix: handle null user #123`

## Type of Change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to change)
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Code refactoring
- [ ] Dependency update

## Related Issues

Closes #(issue number)

Fixes #(issue number)

Related to #(issue number)

## Changes Made

- [ ] Change 1
- [ ] Change 2
- [ ] Change 3

## Testing

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Tested against MySQL 8.0+
- [ ] Tested against PostgreSQL 15+
- [ ] Tested against SQLite
- [ ] Tested against MSSQL 2022
- [ ] All existing tests pass

## File Changes Summary

<!-- List the key files changed -->

<!-- List the key files changed -->

- `file1.go` - Brief description
- `file2.go` - Brief description

## Checklist

- [ ] Code follows Go style guidelines (gofumpt, golangci-lint)
- [ ] Comments follow Go effective comment standards
- [ ] Updated [README.md](../README.md) if needed
- [ ] Updated [ERROR_HANDLING.md](../ERROR_HANDLING.md) if error handling changed
- [ ] No new linting warnings generated
- [ ] Tests pass locally (`make test`)
- [ ] Code review comments addressed
- [ ] Commits follow the format specified in [CONTRIBUTING.md](../CONTRIBUTING.md)

## Performance Impact

- [ ] No performance impact
- [ ] Performance improvement (specify improvement %)
- [ ] Performance regression discussed and acceptable

## Database Compatibility

- [ ] MySQL 8.0+
- [ ] PostgreSQL 15+
- [ ] SQLite (all versions)
- [ ] MSSQL 2022

## Breaking Changes

<!-- If this is a breaking change, describe the migration path for users -->

None

## Screenshots / Diagrams (if applicable)

<!-- Add screenshots or diagrams if this PR includes UI changes or architectural changes -->

## Additional Notes

<!-- Any additional context or notes for reviewers -->
