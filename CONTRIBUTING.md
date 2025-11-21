# Contributing to terraform-provider-aap

Thank you for contributing to the Ansible Automation Platform Terraform Provider!

These guidelines document the standards for contributions. Whether you're an internal team member or external contributor, following these guidelines ensures consistent, high-quality contributions.

## Before You Start

**⚠️ Open an issue first before investing time in a pull request.**

We want to prevent wasted effort. Before writing code:

1. **Open a GitHub issue** describing the bug fix or feature you want to contribute
2. **Wait for maintainer feedback** to confirm the contribution would be accepted
3. **Get agreement on the approach** before starting implementation
4. **Then create your PR** referencing the issue

**Important:** Pull requests submitted without prior discussion may not be merged, even if the code is high quality. This helps ensure your time is well spent on contributions we can accept.

## Quick Start

| Step | Command | Purpose |
|------|---------|---------|
| 1. Fork & clone | `git clone https://github.com/<your-username>/terraform-provider-aap` | Get the code |
| 2. Create branch from main | `git checkout main && git checkout -b feature-name` | Isolate your work |
| 3. Make changes | Edit code, add tests | Implement feature |
| 4. Run checks | `make lint && make test && make testacc` | Verify quality |
| 5. Commit | `git commit -m "description"` | Save changes |
| 6. Push | `git push origin feature-name` | Share work |
| 7. Open PR | Create pull request on GitHub | Request review |

For branching strategy details, see the [README](README.md#branching-model).

## Requirements

| Tool | Version | Installation Guide |
|------|---------|-------------------|
| Go | 1.24 | <https://go.dev/doc/install> |
| Terraform | 1.0+ | <https://developer.hashicorp.com/terraform/install> |
| golangci-lint | 2.5.0 | <https://golangci-lint.run/usage/install/> |
| AAP/AWX instance | 2.4+ | <https://github.com/ansible/awx/blob/devel/INSTALL.md> |

## Development Setup

| Action | Command |
|--------|---------|
| Build provider | `make build` |
| Configure local dev | See [README - Local Development](README.md#installation-for-local-development) |

For detailed local development setup including Terraform dev_overrides configuration, see the [README](README.md#installation-for-local-development).

## Testing Requirements

### Unit Tests

| Action | Command | Required? |
|--------|---------|-----------|
| Run all unit tests | `make test` | ✅ Yes |
| Run with coverage | `make testcov` | ✅ Yes |
| CI must pass | Automatic | ✅ Yes |

**Requirements:**

- All PRs must include unit tests for new code
- New code must have minimum 80% test coverage (enforced by SonarQube)

**Check coverage:**

- Locally: Run `make testcov` → generates `./unit-testing.cov`
- On your PR: View results at [SonarCloud - terraform-provider-aap](https://sonarcloud.io/project/overview?id=ansible_terraform-provider-aap)

### Acceptance Tests

| Action | Command | Required? |
|--------|---------|-----------|
| Run acceptance tests | `make testacc` | ✅ Yes (for code changes) |

**Requirement**: PRs with code changes must include acceptance test output in PR description.

**Setup**: For detailed acceptance test setup instructions, see the [README Testing section](README.md#acceptance-tests).

⚠️ **WARNING**: Acceptance tests modify real AAP resources. Use a dedicated test instance.

### Linting

| Action | Command | Required? |
|--------|---------|-----------|
| Run linters | `make lint` | ✅ Yes |
| CI must pass | Automatic | ✅ Yes |

**Requirement**: All linting errors must be fixed before merge.

## Code Standards

### Pull Request Checklist

- [ ] PR references related GitHub issue (if applicable)
- [ ] PR has appropriate labels (`bug`, `enhancement`, `documentation`, etc.)
- [ ] PR is focused on single feature/fix
- [ ] Unit tests added/updated
- [ ] Code coverage ≥80% for new code (`make testcov`)
- [ ] Acceptance tests pass (include output in PR)
- [ ] Testing instructions provided in PR description
- [ ] Linting passes (`make lint`)
- [ ] Documentation updated (if applicable)
- [ ] Changelog entry added (if applicable)
- [ ] Examples updated (if applicable)
- [ ] AI attribution noted (if applicable)

### PR Title Format

Include the related issue number in your PR title:

```text
[#123] Brief description of changes
```

**Examples:**

- ✅ `[#45] Add retry logic for API calls`
- ✅ `Fix workflow job timeout bug` (if no issue number)

**Note:** Internal contributors may use Jira ticket numbers (e.g., `[AAP-12345]`), but external contributors should use GitHub issue numbers.

### Focused PRs

**Keep PRs focused on one feature or fix.** If you find yourself making multiple unrelated changes, split them into separate PRs.

**Good examples:**

- ✅ New resource + related changes to same resource
- ✅ Bug fix across multiple related resources
- ✅ Documentation updates for one feature

**Should be split:**

- ❌ New feature + unrelated bug fixes
- ❌ Changes to multiple unrelated resources
- ❌ Refactoring + new features

**Why?** Focused PRs are easier to review, test, and revert if needed.

### Responding to Review Comments

**Keep the conversation alive:**

- **Acknowledge all comments** - Reply explicitly to each review comment, even if just to say "Will look into this"
- **Avoid emoji-only responses** - These can be interpreted multiple ways; use clear text
- **Add new commits for review changes** - Don't rebase/squash during review; incremental commits show what changed
- **Resolve conversations** - Mark conversations as resolved once addressed

**Why?** Clear communication helps reviewers understand your changes and speeds up approval.

### PR Labels

**Recommended:** Add labels to your PR to help categorize it.

| Label | When to Use |
|-------|-------------|
| `bug` | Fixes a defect or issue |
| `enhancement` | Adds new functionality or improves existing features |
| `documentation` | Updates to documentation only |

### Autogenerated Code

**DO NOT manually edit autogenerated files.**

| File Pattern | Generator | How to Regenerate |
|-------------|-----------|-------------------|
| `mock_*.go` | mockgen | `go generate ./...` |

Files with `// Code generated by MockGen. DO NOT EDIT.` should never be edited manually. Modify the source file and regenerate instead.

### New Resources/Data Sources

| Requirement | Location | Notes |
|-------------|----------|-------|
| Implementation | `internal/provider/` | Use Plugin Framework |
| Unit tests | `*_test.go` | Test CRUD operations, edge cases, error handling |
| Acceptance test | `*_test.go` | Full resource lifecycle with real AAP instance |
| Documentation | `templates/{resources,data-sources}/` | Use `.md.tmpl` |
| Example | `examples/{resources,data-sources}/` | Working `.tf` file |
| Schema validation | In schema definition | Client-side validation |

### Documentation Updates

| When? | What to update | Command |
|-------|----------------|---------|
| New resource/data source | Add template + example | `make generatedocs` |
| Change resource schema | Update template | `make generatedocs` |
| Add/modify examples | Edit `.tf` files | `make generatedocs` |

**Requirement**: Run `make generatedocs` and commit generated docs.

Validate docs at: <https://registry.terraform.io/tools/doc-preview>

### Changelog Entries

| Change Type | When Required | Format |
|------------|---------------|--------|
| New feature | New resource/data source/field | `changelogs/fragments/YYYYMMDD-description.yml` |
| Bug fix | Fixes existing behavior | `changelogs/fragments/YYYYMMDD-description.yml` |
| Breaking change | Incompatible changes | Required + highlighted |
| Internal refactoring | No user-facing change | Not required |

**Tool:** This project uses [antsibull-changelog](https://github.com/ansible-community/antsibull-changelog) for changelog management.

**Format:** Use existing fragments in `changelogs/fragments/` as examples.

**Example fragment** (`changelogs/fragments/20251114-new-feature.yml`):

```yaml
minor_changes:
  - "Added support for workflow job templates as a data source"
```

## CI Expectations

| Check | Must Pass? | What it does |
|-------|------------|--------------|
| Unit tests | ✅ Yes | Runs `go test` |
| Code coverage | ✅ Yes (enforced by SonarQube) | Automatically blocks PRs with <80% coverage on new code |
| Linting | ✅ Yes | Runs `golangci-lint` |
| Build | ✅ Yes | Compiles provider |

**Policy**: PRs with failing CI will not be reviewed until fixed.

**View CI results:** Check [SonarCloud](https://sonarcloud.io/project/overview?id=ansible_terraform-provider-aap) for detailed coverage analysis on your PR.

## Review Process

| Stage | Timeline | Action |
|-------|----------|--------|
| PR opened | Within 2 business days | Initial review |
| Changes requested | N/A | Address feedback |
| Approved | Within 1 business day | Merge to main |

### What Reviewers Check

- [ ] Tests pass and cover new code
- [ ] Testing instructions provided (for code changes)
- [ ] Linting passes (coverage enforced by SonarQube)
- [ ] Documentation is clear and complete
- [ ] Code follows existing patterns
- [ ] PR scope is focused
- [ ] Changelog entry (if needed)

## AI Code Assistant Usage

If you use AI code assistants, please follow these guidelines:

| Guideline | Details |
|-----------|---------|
| **Human verification** | Always review, test, and understand AI-generated code before submitting |
| **When to attribute** | Note substantial AI assistance in commit messages AND PR description using `Assisted-by:` |
| **Security** | Never input sensitive data (API keys, credentials, customer data) into AI tools |
| **Code quality** | AI-generated code must meet the same standards (tests, coverage, linting) |
| **License compliance** | Ensure AI suggestions don't introduce incompatible licenses |

**Create attribution statements:** Use <https://aiattribution.github.io/> to generate detailed AI attribution.

**Example commit message:**

```text
Add retry logic for API calls

Implements exponential backoff for transient failures.

Assisted-by: GitHub Copilot
```

**Example PR description:**

```text
Implemented retry logic for API calls

Assisted-by: GitHub Copilot
```

## Reporting Security Issues

**DO NOT** open public issues for security vulnerabilities.

Report security issues to: <security@ansible.com>

For more information, see: <https://www.ansible.com/security>

## Code of Conduct

This project follows the [Ansible Community Code of Conduct](https://docs.ansible.com/ansible/latest/community/code_of_conduct.html).

For broader Ansible community contribution guidelines, see the [Ansible Contribution Guide](https://docs.ansible.com/ansible/latest/community/index.html).

## Getting Help

| Question Type | Where to Ask |
|--------------|--------------|
| Bug reports | [GitHub Issues](https://github.com/ansible/terraform-provider-aap/issues) |
| Feature requests | [GitHub Issues](https://github.com/ansible/terraform-provider-aap/issues) |
| Usage questions | [Ansible Community Forum](https://forum.ansible.com/) |
| Security issues | <security@ansible.com> |

## License

GNU General Public License v3.0 - See [LICENSE](LICENSE)
