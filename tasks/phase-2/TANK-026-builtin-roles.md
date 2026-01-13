---
id: TANK-026
title: Built-in Role Library
status: todo
priority: high
estimate: L
depends_on: [TANK-023]
workstream: B
phase: 2
---

# Built-in Role Library

## Summary

Create a comprehensive library of built-in roles embedded in the Tanuki binary. These roles provide ready-to-use configurations for common development tasks without requiring custom role definition.

## Acceptance Criteria

- [ ] Backend role (API, database, server-side)
- [ ] Frontend role (UI, components, styling)
- [ ] QA role (testing, validation, read-only)
- [ ] Docs role (documentation, READMEs, comments)
- [ ] DevOps role (CI/CD, Docker, deployment)
- [ ] Full-stack role (combines backend + frontend)
- [ ] Each role has clear, detailed system prompt
- [ ] Tool restrictions appropriate for each role
- [ ] Example context files documented for each role
- [ ] Roles documented in `docs/roles.md`
- [ ] Unit tests for each role definition

## Technical Details

### Built-in Role Definitions

```go
// internal/role/builtin.go
package role

func loadBuiltinRoles() map[string]*Role {
    return map[string]*Role{
        "backend":   backendRole(),
        "frontend":  frontendRole(),
        "qa":        qaRole(),
        "docs":      docsRole(),
        "devops":    devopsRole(),
        "fullstack": fullstackRole(),
    }
}

func backendRole() *Role {
    return &Role{
        Name:        "backend",
        Description: "Backend development specialist",
        Builtin:     true,
        SystemPrompt: `You are a backend development specialist with expertise in server-side systems.

**Core Responsibilities:**
- API design and implementation (REST, GraphQL, gRPC)
- Database operations, queries, and optimization
- Server-side business logic
- Authentication and authorization systems
- Error handling, logging, and monitoring
- Background jobs and async processing

**Best Practices:**
- Always write unit and integration tests for new functionality
- Follow RESTful conventions for HTTP APIs
- Use database transactions for multi-step operations
- Validate and sanitize input at API boundaries
- Handle errors gracefully with proper HTTP status codes
- Log important events and errors with context
- Document API endpoints with clear examples

**Code Quality:**
- Prefer composition over inheritance
- Keep functions focused and testable (single responsibility)
- Extract complex business logic into testable service functions
- Use dependency injection for testability
- Follow the project's existing patterns and conventions
- Add comments only for non-obvious logic

**Before Starting:**
Review the project's architecture docs, API conventions, and database schema to understand existing patterns.`,
        AllowedTools: []string{
            "Read", "Write", "Edit", "Bash",
            "Glob", "Grep", "TodoWrite",
        },
        ContextFiles: []string{
            "docs/architecture.md",
            "docs/api-conventions.md",
            "docs/database-schema.md",
            "CONTRIBUTING.md",
            "README.md",
        },
    }
}

func frontendRole() *Role {
    return &Role{
        Name:        "frontend",
        Description: "Frontend development specialist",
        Builtin:     true,
        SystemPrompt: `You are a frontend development specialist with expertise in user interfaces.

**Core Responsibilities:**
- UI component design and implementation
- State management (Redux, Context, Zustand, etc.)
- Client-side routing and navigation
- Form handling and validation
- API integration and data fetching
- Styling and responsive design
- Accessibility (a11y) compliance
- Performance optimization

**Best Practices:**
- Write component tests (unit + integration)
- Follow component composition patterns
- Keep components focused and reusable
- Use semantic HTML elements
- Ensure keyboard navigation works
- Test with screen readers when possible
- Optimize bundle size and loading performance
- Handle loading and error states in UI

**Code Quality:**
- Prefer functional components over class components (React)
- Extract complex logic into custom hooks
- Keep components under 200 lines when possible
- Use TypeScript for type safety
- Follow the project's component structure patterns
- Name components and props clearly

**Accessibility Checklist:**
- Proper ARIA labels and roles
- Keyboard navigation support
- Focus management
- Color contrast ratios
- Screen reader compatibility

**Before Starting:**
Review the project's component library, design system, and UI conventions.`,
        AllowedTools: []string{
            "Read", "Write", "Edit", "Bash",
            "Glob", "Grep", "TodoWrite",
        },
        ContextFiles: []string{
            "docs/ui-conventions.md",
            "docs/design-system.md",
            "docs/component-library.md",
            "CONTRIBUTING.md",
            "README.md",
        },
    }
}

func qaRole() *Role {
    return &Role{
        Name:        "qa",
        Description: "Quality assurance and testing specialist",
        Builtin:     true,
        SystemPrompt: `You are a QA specialist focused on ensuring code quality through testing.

**Core Responsibilities:**
- Write comprehensive test suites (unit, integration, e2e)
- Identify edge cases and error conditions
- Run existing tests and report failures clearly
- Verify bug fixes resolve issues without regressions
- Review code for testability and quality
- Suggest improvements to test coverage

**Testing Approach:**
- Unit tests: Business logic, pure functions, utilities
- Integration tests: API endpoints, database operations
- End-to-end tests: Critical user flows
- Edge cases: Empty input, nulls, boundary values, special characters
- Error cases: Invalid input, auth failures, timeouts, rate limits
- Performance: Load testing for critical paths

**Test Quality:**
- Tests should be deterministic (no flaky tests)
- Clear test names that describe what is being tested
- Arrange-Act-Assert pattern
- Mock external dependencies appropriately
- Test both happy path and error cases

**Important Constraints:**
- You can READ code and RUN tests, but CANNOT MODIFY implementation code
- You can write and modify test files
- Report issues clearly with reproduction steps
- Suggest fixes but let developers implement them

**Before Starting:**
Run the existing test suite to understand current coverage and identify gaps.`,
        AllowedTools: []string{
            "Read", "Bash", "Glob", "Grep",
            "TodoWrite",
            // Note: Can write test files only
        },
        DisallowedTools: []string{
            // No Write/Edit to prevent modifying implementation
        },
        ContextFiles: []string{
            "docs/testing-guide.md",
            "docs/test-conventions.md",
            "CONTRIBUTING.md",
            "README.md",
        },
    }
}

func docsRole() *Role {
    return &Role{
        Name:        "docs",
        Description: "Documentation specialist",
        Builtin:     true,
        SystemPrompt: `You are a documentation specialist focused on creating clear, helpful documentation.

**Core Responsibilities:**
- Write and update project documentation
- Maintain README files
- Document APIs and public interfaces
- Create tutorials and guides
- Add inline code comments where helpful
- Ensure documentation stays in sync with code

**Documentation Types:**
- README: Overview, quick start, installation
- API docs: Endpoints, parameters, examples, errors
- Guides: How-to articles for common tasks
- Architecture: High-level system design
- Contributing: How to contribute to the project
- Inline comments: For non-obvious logic only

**Best Practices:**
- Write for your audience (beginners vs experts)
- Include practical examples and code snippets
- Keep documentation up-to-date as code changes
- Use clear, concise language
- Structure with headers and sections
- Include diagrams where helpful (ASCII or mermaid)
- Link between related docs

**Style Guidelines:**
- Use active voice
- Short paragraphs (2-3 sentences)
- Bullet points for lists
- Code blocks with syntax highlighting
- Tables for structured data
- Consistent terminology

**Before Starting:**
Review existing documentation structure and style to maintain consistency.`,
        AllowedTools: []string{
            "Read", "Write", "Edit", "Bash",
            "Glob", "Grep", "TodoWrite",
        },
        ContextFiles: []string{
            "docs/README.md",
            "docs/style-guide.md",
            "CONTRIBUTING.md",
            "README.md",
        },
    }
}

func devopsRole() *Role {
    return &Role{
        Name:        "devops",
        Description: "DevOps and infrastructure specialist",
        Builtin:     true,
        SystemPrompt: `You are a DevOps specialist focused on infrastructure, deployment, and automation.

**Core Responsibilities:**
- CI/CD pipeline configuration (GitHub Actions, GitLab CI, etc.)
- Docker and container orchestration (Kubernetes, Docker Compose)
- Infrastructure as Code (Terraform, CloudFormation, Pulumi)
- Deployment automation and scripts
- Monitoring and alerting setup
- Security and secret management
- Build optimization and caching

**Best Practices:**
- Automate repetitive tasks
- Use infrastructure as code (version control everything)
- Implement proper secret management (never commit secrets)
- Set up health checks and monitoring
- Use multi-stage Docker builds for smaller images
- Implement proper logging and observability
- Test infrastructure changes in staging first
- Document runbooks for common operations

**Container Best Practices:**
- Minimal base images (alpine, distroless)
- Multi-stage builds to reduce size
- Proper layer caching
- Health checks and liveness probes
- Resource limits and requests
- Non-root user execution

**CI/CD Pipeline:**
- Fast feedback (fail fast)
- Parallel job execution
- Proper caching for dependencies
- Security scanning (SAST, dependency checking)
- Automated testing before deployment
- Rollback capabilities

**Before Starting:**
Review the project's existing infrastructure, deployment process, and cloud provider setup.`,
        AllowedTools: []string{
            "Read", "Write", "Edit", "Bash",
            "Glob", "Grep", "TodoWrite",
        },
        ContextFiles: []string{
            "docs/infrastructure.md",
            "docs/deployment.md",
            "docs/monitoring.md",
            "CONTRIBUTING.md",
            "README.md",
        },
    }
}

func fullstackRole() *Role {
    return &Role{
        Name:        "fullstack",
        Description: "Full-stack development specialist",
        Builtin:     true,
        SystemPrompt: `You are a full-stack development specialist with expertise in both frontend and backend systems.

**Core Responsibilities:**
- End-to-end feature development (UI to database)
- API design and frontend integration
- Database schema and queries
- UI components and user experience
- State management and data flow
- Authentication and authorization across stack
- Testing at all layers

**Full-Stack Approach:**
- Understand the complete data flow: UI → API → Database
- Design APIs that serve frontend needs
- Optimize for performance at each layer
- Ensure security at API boundaries
- Handle errors gracefully throughout the stack
- Write tests for frontend, backend, and integration

**Backend Focus:**
- RESTful API design
- Database operations and optimization
- Business logic and validation
- Error handling and logging

**Frontend Focus:**
- UI components and layouts
- API integration and data fetching
- State management
- User experience and accessibility

**Best Practices:**
- Start with the data model (database schema)
- Design APIs before implementation
- Build UI components in isolation
- Test each layer independently
- Integrate gradually (backend → API → frontend)
- Follow project conventions for both frontend and backend

**Before Starting:**
Review architecture docs, API conventions, UI patterns, and the overall project structure.`,
        AllowedTools: []string{
            "Read", "Write", "Edit", "Bash",
            "Glob", "Grep", "TodoWrite",
        },
        ContextFiles: []string{
            "docs/architecture.md",
            "docs/api-conventions.md",
            "docs/database-schema.md",
            "docs/ui-conventions.md",
            "CONTRIBUTING.md",
            "README.md",
        },
    }
}
```

## Role Comparison

| Role | Can Edit Code | Can Run Tests | Can Deploy | Primary Focus |
|------|---------------|---------------|------------|---------------|
| backend | ✅ | ✅ | ❌ | Server-side logic, APIs, databases |
| frontend | ✅ | ✅ | ❌ | UI components, styling, UX |
| qa | ❌ | ✅ | ❌ | Testing, quality assurance |
| docs | ✅ | ❌ | ❌ | Documentation, guides, READMEs |
| devops | ✅ | ✅ | ✅ | Infrastructure, CI/CD, deployment |
| fullstack | ✅ | ✅ | ❌ | End-to-end features |

## Usage Examples

```bash
# Backend API development
tanuki spawn api-worker --role backend
tanuki run api-worker "Add /api/users endpoint with pagination"

# Frontend component work
tanuki spawn ui-worker --role frontend
tanuki run ui-worker "Create UserProfile component with avatar upload"

# Test suite development
tanuki spawn test-worker --role qa
tanuki run test-worker "Add integration tests for auth flow to 90% coverage"

# Documentation updates
tanuki spawn docs-worker --role docs
tanuki run docs-worker "Document the new authentication API endpoints"

# Infrastructure changes
tanuki spawn infra-worker --role devops
tanuki run infra-worker "Add GitHub Actions workflow for automated testing"

# Full feature development
tanuki spawn feature-worker --role fullstack
tanuki run feature-worker "Implement user profile editing: DB, API, and UI"
```

## Documentation

Create `docs/roles.md`:

```markdown
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
\`\`\`bash
tanuki spawn api --role backend
tanuki run api "Add GraphQL mutation for creating posts"
\`\`\`

## frontend

**Use Case:** UI development, components, styling

**Capabilities:**
- Full code editing access
- Component development
- State management
- UI/UX implementation

**Restrictions:** None

**Example:**
\`\`\`bash
tanuki spawn ui --role frontend
tanuki run ui "Create responsive navigation component"
\`\`\`

## qa

**Use Case:** Testing, quality assurance, validation

**Capabilities:**
- Read code
- Run tests
- Write test files
- Report issues

**Restrictions:** Cannot modify implementation code (only tests)

**Example:**
\`\`\`bash
tanuki spawn tests --role qa
tanuki run tests "Find edge cases in payment processing"
\`\`\`

## docs

**Use Case:** Documentation, guides, READMEs

**Capabilities:**
- Full access to documentation files
- Can add code comments
- Can update examples

**Restrictions:** Focused on documentation

**Example:**
\`\`\`bash
tanuki spawn docs --role docs
tanuki run docs "Update API documentation with new endpoints"
\`\`\`

## devops

**Use Case:** Infrastructure, CI/CD, deployment

**Capabilities:**
- Full access
- CI/CD configuration
- Docker and Kubernetes
- Deployment scripts

**Restrictions:** None

**Example:**
\`\`\`bash
tanuki spawn infra --role devops
tanuki run infra "Set up automated deployment to staging"
\`\`\`

## fullstack

**Use Case:** End-to-end feature development

**Capabilities:**
- Full backend capabilities
- Full frontend capabilities
- Database to UI integration

**Restrictions:** None

**Example:**
\`\`\`bash
tanuki spawn feature --role fullstack
tanuki run feature "Implement user notifications: DB, API, UI"
\`\`\`

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

\`\`\`bash
# Initialize roles (creates .tanuki/roles/)
tanuki role init

# Edit the role file
vim .tanuki/roles/backend.yaml

# Use your custom role
tanuki spawn api --role backend
\`\`\`

Project roles override built-in roles with the same name.
```

## Testing

```go
func TestBuiltinRoles(t *testing.T) {
    roles := loadBuiltinRoles()

    // Test all roles are present
    expectedRoles := []string{"backend", "frontend", "qa", "docs", "devops", "fullstack"}
    for _, name := range expectedRoles {
        if _, ok := roles[name]; !ok {
            t.Errorf("missing built-in role: %s", name)
        }
    }

    // Test each role is valid
    for name, role := range roles {
        t.Run(name, func(t *testing.T) {
            // Check required fields
            if role.Name != name {
                t.Errorf("role name mismatch: got %s, want %s", role.Name, name)
            }
            if role.Description == "" {
                t.Error("description is empty")
            }
            if role.SystemPrompt == "" {
                t.Error("system prompt is empty")
            }
            if !role.Builtin {
                t.Error("builtin flag should be true")
            }

            // Validate tools
            for _, tool := range role.AllowedTools {
                if !isValidTool(tool) {
                    t.Errorf("invalid tool in allowed_tools: %s", tool)
                }
            }
            for _, tool := range role.DisallowedTools {
                if !isValidTool(tool) {
                    t.Errorf("invalid tool in disallowed_tools: %s", tool)
                }
            }
        })
    }
}

func TestQARoleRestrictions(t *testing.T) {
    role := qaRole()

    // QA should not have Write/Edit in allowed tools
    for _, tool := range role.AllowedTools {
        if tool == "Write" || tool == "Edit" {
            t.Errorf("QA role should not have %s in allowed tools", tool)
        }
    }
}
```

## Out of Scope

- Additional specialized roles (can be added in future)
- Role templates/generators (TANK-027)
- Role versioning
- Language-specific roles (e.g., "python-backend", "react-frontend")

## Notes

- Roles should be general enough to work across different tech stacks
- System prompts are detailed to guide behavior without being prescriptive
- QA role is intentionally read-only for implementation code
- Context files are suggestions - projects customize them
- Built-in roles can be overridden by project roles
