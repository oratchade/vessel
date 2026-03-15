# Documentation Linting Guide

This project includes comprehensive markdown and documentation linting tools to maintain consistent documentation quality across the fabric project.

## Available Tools

### 1. Markdownlint (Syntax & Style)

**Purpose:** Validates markdown syntax and enforces consistent styling

- **Config file:** `.markdownlint.yaml`
- **Focuses on:**
  - Heading consistency
  - List indentation
  - Line length (120 chars max)
  - Code block language specification
  - Bare URLs
  - HTML usage

**Installation:**

```bash
npm install -g markdownlint-cli2
```

**Usage:**

```bash
make lint-markdown
# or manually:
markdownlint-cli2 "**/*.md"
```

**Example violations:**

- ❌ Inconsistent heading styles
- ❌ Missing language tags in code blocks
- ❌ Lines exceeding 120 characters
- ❌ Inconsistent list formatting

---

### 2. Vale (Content & Style)

**Purpose:** Checks content quality, clarity, and voice consistency

- **Config file:** `.vale.yaml`
- **Styles directory:** `.vale/styles/`
- **Focuses on:**
  - Spelling and grammar
  - Simplicity and readability
  - Avoiding hedging language
  - Project-specific vocabulary (Project.yml)

**Installation:**

```bash
go install github.com/errata-ai/vale/v3@latest
```

**Usage:**

```bash
make lint-docs
# or manually:
vale --config .vale.yaml ./docs ./README.md
```

**Example violations:**

- ❌ Spelling mistakes
- ❌ Overly complex sentences
- ❌ Unnecessary hedging ("could", "might", "arguably")
- ❌ Undefined project-specific terms

---

## Project Vocabulary

Custom project terms are defined in `.vale/styles/Project.yml` to avoid false positives:

```yaml
- fabric
- DBActions
- PostgreSQL
- CockroachDB
- parameterized
- CRUD
```

Add terms to this file when introducing new project-specific vocabulary.

---

## Integration with CI/CD

Both tools are integrated into the `make check` target:

```bash
make check  # Runs all: fmt-check, lint, lint-markdown, lint-docs, mocks, coverage
```

---

## Configuration Files

### .markdownlint.yaml

Controls markdown syntax rules. Key settings:

```yaml
MD013:
  line_length: 120 # Maximum line length
  heading_line_length: 100
MD024:
  allow_different_nesting: true # Allow repeated headings in different nesting levels
MD033: false # Allow inline HTML
```

### .vale.yaml

Controls content quality rules. Key settings:

```yaml
MinAlertLevel: warning  # Treat warnings as failures
[*.md]
vale.Spelling = YES
vale.Simplicity = warning
```

---

## Best Practices

1. **Keep lines under 120 characters** - Better readability and diff visibility
2. **Use meaningful headings** - Clear structure helps readers navigate
3. **Include language tags in code blocks** - Enables syntax highlighting
4. **Write clearly** - Avoid hedging language; be direct
5. **Define new terms** - Add to `.vale/styles/Project.yml` if introducing new vocabulary

---

## Troubleshooting

### "markdownlint-cli2 not found"

```bash
npm install -g markdownlint-cli2
# or if using Node version manager:
npx markdownlint-cli2 "**/*.md"
```

### "Vale not found"

```bash
go install github.com/errata-ai/vale/v3@latest
```

### Too many warnings in check target

Vale is configured to only warn by default. To fail on Vale warnings, edit `.vale.yaml`:

```yaml
MinAlertLevel: error # Change from 'warning' to 'error'
```

---

## Editor Integration

### VS Code

**Markdownlint Extension:**

```json
{
  "markdownlint.config": {
    "extends": ".markdownlint.yaml"
  }
}
```

**Vale Extension:**

```json
{
  "vale.valeCLI.path": "/path/to/vale"
}
```

### Other Editors

Most editors support these tools via plugins. Install the appropriate extension and point it to your configuration files.

---

## Examples

### Good Markdown

````markdown
## Project Structure Overview

The fabric library provides multiple database drivers:

- **MySQL**: Use for relational data
- **PostgreSQL**: Advanced features and extensions
- **SQLite**: Development and testing
- **MSSQL**: Enterprise environments

### Code Example

```go
db, err := fabric.NewDB(config)
```
````

````

### Issues to Avoid
```markdown
# Poorly formatted markdown that violates multiple rules


This   has   inconsistent spacing and very long lines that exceed the recommended character limits set in the configuration.

```python
# Missing language tag - will trigger MD040
code here
````

```

---

## References

- [Markdownlint Rules](https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md)
- [Vale Documentation](https://vale.sh/docs)
- [Markdown Best Practices](https://www.markdownguide.org/)
```
