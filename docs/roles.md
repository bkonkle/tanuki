# Built-in Roles

Tanuki includes six built-in roles optimized for common development tasks.

## backend

**Use Case:** Server-side development, APIs, databases

**Capabilities:**
- Full code editing access
- Database operations
- API development
- Business logic implementation

**Restrictions:** None

**Example:**
```bash
tanuki spawn api --role backend
tanuki run api "Add GraphQL mutation for creating posts"
```

## frontend

**Use Case:** UI development, components, styling

**Capabilities:**
- Full code editing access
- Component development
- State management
- UI/UX implementation

**Restrictions:** None

**Example:**
```bash
tanuki spawn ui --role frontend
tanuki run ui "Create responsive navigation component"
```

## qa

**Use Case:** Testing, quality assurance, validation

**Capabilities:**
- Read code
- Run tests
- Write test files
- Report issues

**Restrictions:** Cannot modify implementation code (only tests)

**Example:**
```bash
tanuki spawn tests --role qa
tanuki run tests "Find edge cases in payment processing"
```

## docs

**Use Case:** Documentation, guides, READMEs

**Capabilities:**
- Full access to documentation files
- Can add code comments
- Can update examples

**Restrictions:** Focused on documentation

**Example:**
```bash
tanuki spawn docs --role docs
tanuki run docs "Update API documentation with new endpoints"
```

## devops

**Use Case:** Infrastructure, CI/CD, deployment

**Capabilities:**
- Full access
- CI/CD configuration
- Docker and Kubernetes
- Deployment scripts

**Restrictions:** None

**Example:**
```bash
tanuki spawn infra --role devops
tanuki run infra "Set up automated deployment to staging"
```

## fullstack

**Use Case:** End-to-end feature development

**Capabilities:**
- Full backend capabilities
- Full frontend capabilities
- Database to UI integration

**Restrictions:** None

**Example:**
```bash
tanuki spawn feature --role fullstack
tanuki run feature "Implement user notifications: DB, API, UI"
```

## Choosing a Role

| Task | Recommended Role |
|------|------------------|
| Adding API endpoint | backend |
| Creating UI component | frontend |
| Writing tests | qa |
| Updating README | docs |
| Configuring CI/CD | devops |
| Complete feature (DB→UI) | fullstack |
| Bug fix (backend) | backend |
| Bug fix (frontend) | frontend |
| Bug fix (full-stack) | fullstack |

## Customizing Roles

To customize a built-in role:

```bash
# Initialize roles (creates .tanuki/roles/)
tanuki role init

# Edit the role file
vim .tanuki/roles/backend.yaml

# Use your custom role
tanuki spawn api --role backend
```

Project roles override built-in roles with the same name.
