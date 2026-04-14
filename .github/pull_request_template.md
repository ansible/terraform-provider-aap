<!-- 
Thank you for contributing to terraform-provider-aap!

Please review our contribution guidelines: 
https://github.com/ansible/terraform-provider-aap/blob/main/CONTRIBUTING.md
-->

## Description

<!-- What does this PR do? What issue does it fix? -->

**Related Issue:** <!-- Required: Link to GitHub issue (e.g., Fixes #123) or Jira ticket (internal) -->

<!-- 
⚠️ PRs without prior issue discussion may not be merged.
See CONTRIBUTING.md "Before You Start" section.
-->

## Type of Change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to change)
- [ ] Documentation update
- [ ] Code refactoring (no functional changes)

## Testing Checklist

### Unit Tests

- [ ] Unit tests added/updated for new code
- [ ] All unit tests pass locally (`make test`)
- [ ] Verified coverage locally (`make testcov`)

**Note:** SonarQube will automatically verify ≥80% coverage on new code in CI.

**Check your PR coverage:** [SonarCloud - terraform-provider-aap](https://sonarcloud.io/project/overview?id=ansible_terraform-provider-aap)

### Acceptance Tests

- [ ] Acceptance tests added/updated
- [ ] Acceptance tests pass (see output below)

<details>
<summary>Acceptance Test Output</summary>

```bash
# Replace with actual output from: make testacc
$ make testacc

...
```

</details>

### Code Quality

- [ ] Linting passes (`make lint`)
- [ ] Code follows existing patterns
- [ ] PR is focused on a single feature/fix

## Documentation

- [ ] Documentation updated (if user-facing changes)
- [ ] Examples updated (if applicable)
- [ ] Docs regenerated (`make generatedocs`)

## Testing Instructions

<!-- 
Provide step-by-step instructions for reviewers to test your changes.
For code changes, include commands and expected outcomes.
For documentation, explain what was changed and how to verify.
-->

**How to test this PR:**

1.
2.
3.

**Expected outcome:**

## Changelog

- [ ] Changelog entry added in `changelogs/fragments/` (if applicable)
- [ ] Format: `YYYYMMDD-description.yml`

**Changelog not required for:** documentation updates, test updates, code refactoring

## Additional Notes

<!-- Any additional context, breaking changes, migration notes, etc. -->

## AI Attribution

<!-- 
If you used AI code assistants (e.g., GitHub Copilot, Claude, ChatGPT) for substantial 
portions of this PR, please note it:
1. In your commit messages (add "Assisted-by: <tool>" to commit message body)
2. Below in this PR description

For detailed attribution statements, use: https://aiattribution.github.io/
-->

**Assisted-by:** (or **Co-Authored-By:**) <!-- e.g., GitHub Copilot, Claude, ChatGPT, or "None" -->
