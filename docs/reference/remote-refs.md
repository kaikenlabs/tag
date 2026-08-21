# Remote Template References

TAG supports fetching templates from various remote sources. This document covers all supported reference formats.

## Quick Reference

| Format | Example |
|--------|---------|
| GitHub | `gh:user/repo` |
| GitLab | `gl:user/repo` |
| Bitbucket | `bb:user/repo` |
| With version | `gh:user/repo@v1.0.0` |
| With subpath | `gh:user/repo/templates/go-api` |
| Git URL | `https://github.com/user/repo.git` |
| SSH URL | `git@github.com:user/repo.git` |
| Zip URL | `https://example.com/template.zip` |
| Local path | `./my-template` |
| Local zip | `./template.zip` |

## Shorthand Formats

### GitHub (`gh:`)

```bash
gh:user/repo
gh:organization/repo
gh:user/repo@v1.0.0
gh:user/repo@main
gh:user/repo/path/to/template
gh:user/repo@v1.0.0/path/to/template
```

**Examples:**
```bash
tag scaffold gh:kaikenlabs/go-api-template
tag scaffold gh:myorg/templates@v2.0.0
tag scaffold gh:myorg/templates/microservice
```

### GitLab (`gl:`)

```bash
gl:user/repo
gl:group/subgroup/repo
gl:user/repo@v1.0.0
gl:user/repo/path/to/template
```

**Examples:**
```bash
tag scaffold gl:mygroup/templates
tag scaffold gl:company/infra/templates@main
```

### Bitbucket (`bb:`)

```bash
bb:user/repo
bb:workspace/repo
bb:user/repo@v1.0.0
bb:user/repo/path/to/template
```

**Examples:**
```bash
tag scaffold bb:myteam/go-template
tag scaffold bb:myteam/templates@develop/python-api
```

## Version Specifiers

Use `@` to specify a version (tag, branch, or commit):

| Specifier | Example | Description |
|-----------|---------|-------------|
| Tag | `@v1.0.0` | Specific release tag |
| Branch | `@main` | Branch name |
| Branch | `@develop` | Feature branch |
| Commit | `@a1b2c3d` | Specific commit SHA |

**Examples:**
```bash
# Latest (default branch)
tag scaffold gh:user/template

# Specific version
tag scaffold gh:user/template@v1.2.3

# Development branch
tag scaffold gh:user/template@develop

# Specific commit
tag scaffold gh:user/template@abc123def
```

## Subpaths

Access templates in subdirectories:

```bash
# Template at repo-root/templates/go-api/
gh:user/repo/templates/go-api

# With version
gh:user/repo@v1.0.0/templates/go-api
```

This is useful for monorepos containing multiple templates:

```
templates-repo/
├── templates/
│   ├── go-api/
│   │   └── tag.template.json
│   ├── python-cli/
│   │   └── tag.template.json
│   └── react-app/
│       └── tag.template.json
```

```bash
tag scaffold gh:company/templates/templates/go-api
tag scaffold gh:company/templates/templates/python-cli
```

## Full Git URLs

### HTTPS URLs

```bash
https://github.com/user/repo.git
https://gitlab.com/user/repo.git
https://bitbucket.org/user/repo.git
https://git.example.com/user/repo.git
```

**With version:**
```bash
https://github.com/user/repo.git@v1.0.0
```

### SSH URLs

```bash
git@github.com:user/repo.git
git@gitlab.com:user/repo.git
git+ssh://git@github.com/user/repo.git
```

### Git Protocol URLs

```bash
git://github.com/user/repo.git
```

## Zip Files

### Remote Zip

```bash
tag scaffold https://example.com/template.zip
tag scaffold https://github.com/user/repo/archive/refs/tags/v1.0.0.zip
```

### Local Zip

```bash
tag scaffold ./my-template.zip
tag scaffold /path/to/template.zip
```

## Local Paths

```bash
# Relative paths
tag scaffold ./my-template
tag scaffold ../shared-templates/go-api

# Absolute paths
tag scaffold /home/user/templates/go-api
tag scaffold ~/templates/go-api
```

## Cache Behavior

Remote templates are cached locally to avoid repeated downloads.

### Cache Location

```
~/.tag/cache/
├── gh_user_repo/           # Latest version
├── gh_user_repo@v1.0.0/    # Pinned version
├── gl_org_template/
└── _url_a1b2c3d4e5f6/      # URL-based (hashed)
```

Override the cache directory with the `TAG_CACHE_DIR` environment variable — it must be an
absolute path, or TAG errors naming the variable. It's read before `$HOME` is resolved, so it
also works in containers/sandboxes where `$HOME` is unset or unwritable, and it isn't created
until the first cache write (constructing a resolver alone touches nothing on disk).

**Multi-tenant / shared deployments**: a missing `TAG_CACHE_DIR` silently restores the single
shared cache, which means one tenant's cached template can be served to another — cross-tenant
template disclosure. A multi-tenant caller (e.g. a Backstage scaffolder integration) must set
`TAG_CACHE_DIR` explicitly and should fail its own startup if it is unset.

### Cache Management

**Force refresh:**
```bash
tag scaffold gh:user/template --update
```

**Clear all cache:**
```bash
rm -rf ~/.tag/cache/
# or, if TAG_CACHE_DIR is set:
rm -rf "$TAG_CACHE_DIR"
```

`tag cache clear --all` only removes directories TAG itself wrote (identified by a
`_meta.json` file), so pointing `TAG_CACHE_DIR` at a directory that holds other data will not
delete that data.

### Cache TTL

- Non-pinned templates (no `@version`) expire after **24 hours**
- Version-pinned templates (`@v1.0.0`) never expire
- Use `--update` to force refresh regardless of TTL

## Authentication

### SSH Keys

For SSH URLs, TAG uses your SSH agent or `~/.ssh/` keys:

```bash
# Uses ~/.ssh/id_rsa or SSH agent
tag scaffold git@github.com:private/repo.git
```

### HTTPS Credentials

For private repositories over HTTPS, TAG uses provider-specific tokens via environment variables:

```bash
# GitHub
export GITHUB_TOKEN=your-personal-access-token

# GitLab
export GITLAB_TOKEN=your-personal-access-token

# Bitbucket
export BITBUCKET_TOKEN=your-app-password
```

TAG automatically uses these tokens when fetching from the corresponding provider.

### Personal Access Tokens

For GitHub/GitLab/Bitbucket private repos:

```bash
# Set the appropriate token for your provider
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx

# Then fetch normally
tag scaffold gh:myorg/private-template
```

## Error Handling

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| "empty reference" | No template specified | Provide a template path or reference |
| "missing repository path" | Invalid shorthand | Use format `gh:user/repo` |
| "invalid shorthand format" | Missing user or repo | Check reference format |
| "local path not found" | Path doesn't exist | Verify the path exists |
| "local zip file not found" | Zip file missing | Check file path |
| "network error" | No connectivity | Check internet connection |
| "authentication failed" | Invalid credentials | Verify SSH keys or tokens |

### Debugging

```bash
# Verbose output
ENV=DEV tag scaffold gh:user/template
```

## Best Practices

### 1. Use Version Pinning for Production

```bash
# Don't use in CI/CD (may change)
tag scaffold gh:company/template

# Do use in CI/CD (stable)
tag scaffold gh:company/template@v1.2.3
```

### 2. Use Shorthand When Possible

```bash
# Preferred
tag scaffold gh:user/repo

# More verbose
tag scaffold https://github.com/user/repo.git
```

### 3. Organize Monorepo Templates

```
company-templates/
├── README.md
├── go/
│   ├── api/
│   │   └── tag.template.json
│   └── cli/
│       └── tag.template.json
├── python/
│   └── flask/
│       └── tag.template.json
```

```bash
tag scaffold gh:company/templates/go/api
tag scaffold gh:company/templates/python/flask@v2.0.0
```

### 4. Document Your Template Reference

In your project's README:

```markdown
## Create New Service

```bash
tag scaffold gh:company/templates/microservice@v1.5.0 my-new-service
```
```

## See Also

- [Scaffold Command](../commands/scaffold.md) - Full command reference
- [Template Authoring](../templates/authoring.md) - Creating templates
- [Getting Started](../getting-started.md) - Quick start guide
