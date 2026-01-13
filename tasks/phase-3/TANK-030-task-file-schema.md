---
id: TANK-030
title: Task File Schema
status: todo
priority: high
estimate: M
depends_on: [TANK-020]
workstream: A
phase: 3
---

# Task File Schema

## Summary

Define the markdown-based task file format with YAML front matter. Tasks are designed for autonomous completion using Ralph-style objective criteria. This is the foundation that all other Phase 3 components build upon.

## Acceptance Criteria

- [ ] Markdown files with YAML front matter parser
- [ ] Task struct with all required fields
- [ ] Role assignment in front matter (links to Phase 2 roles)
- [ ] **Completion criteria**: verify commands and/or completion signals
- [ ] Priority levels (critical, high, medium, low)
- [ ] Dependency tracking (list of task IDs)
- [ ] Status tracking (pending, assigned, in_progress, review, complete, failed, blocked)
- [ ] Tags for categorization
- [ ] Validation on parse (required fields, valid status, valid priority)
- [ ] Unit tests with 80%+ coverage

## Technical Details

### Task File Format

Tasks use YAML front matter with **objective completion criteria** - either a verify command or completion signal (Ralph-style).

```markdown
---
id: TASK-001
title: Implement User Authentication
role: backend
priority: high
status: pending
depends_on: []

# Completion criteria (Ralph-style)
completion:
  verify: "npm test -- --grep 'auth'"  # Command that must pass (exit 0)
  signal: "AUTH_COMPLETE"               # Or look for this in output
  max_iterations: 20                    # For Ralph mode (default: 30)

tags:
  - auth
  - security
---

# Implement User Authentication

Add OAuth2-based authentication to the API.

## Requirements

1. **OAuth2 Flow** - Google as identity provider
2. **JWT Tokens** - 15min access, 7day refresh
3. **Security** - HTTP-only cookies, CSRF protection

## Done When

- `npm test -- --grep 'auth'` passes
- All auth endpoints return proper status codes
- Say AUTH_COMPLETE when finished
```

### Simpler Example (verify only)

```markdown
---
id: TASK-002
title: Fix ESLint Errors
role: frontend
priority: medium
completion:
  verify: "npm run lint"
---

# Fix ESLint Errors

Run `npm run lint` and fix all reported errors.
```

### Go Types

```go
// internal/task/types.go
package task

import "time"

// Task represents a work item to be assigned to an agent
type Task struct {
    // From front matter
    ID         string            `yaml:"id"`
    Title      string            `yaml:"title"`
    Role       string            `yaml:"role"`
    Priority   Priority          `yaml:"priority"`
    Status     Status            `yaml:"status"`
    DependsOn  []string          `yaml:"depends_on"`
    AssignedTo string            `yaml:"assigned_to,omitempty"`
    Completion *CompletionConfig `yaml:"completion,omitempty"`
    Tags       []string          `yaml:"tags,omitempty"`

    // Derived fields (not in YAML)
    FilePath    string     `yaml:"-"`
    Content     string     `yaml:"-"` // Markdown body (after front matter)
    CompletedAt *time.Time `yaml:"completed_at,omitempty"`
    StartedAt   *time.Time `yaml:"started_at,omitempty"`
}

// Priority levels for tasks
type Priority string

const (
    PriorityCritical Priority = "critical"
    PriorityHigh     Priority = "high"
    PriorityMedium   Priority = "medium"
    PriorityLow      Priority = "low"
)

// Status values for task lifecycle
type Status string

const (
    StatusPending    Status = "pending"
    StatusAssigned   Status = "assigned"
    StatusInProgress Status = "in_progress"
    StatusReview     Status = "review"
    StatusComplete   Status = "complete"
    StatusFailed     Status = "failed"
    StatusBlocked    Status = "blocked"
)

// CompletionConfig defines how to determine task completion (Ralph-style)
type CompletionConfig struct {
    // Verify is a command that must exit 0 for completion
    Verify string `yaml:"verify,omitempty"`

    // Signal is a string to detect in agent output
    Signal string `yaml:"signal,omitempty"`

    // MaxIterations for Ralph mode (default: 30)
    MaxIterations int `yaml:"max_iterations,omitempty"`
}

// IsRalphMode returns true if task should use Ralph-style iteration
func (t *Task) IsRalphMode() bool {
    return t.Completion != nil && (t.Completion.Verify != "" || t.Completion.Signal != "")
}

// GetMaxIterations returns max iterations with default fallback
func (c *CompletionConfig) GetMaxIterations() int {
    if c.MaxIterations <= 0 {
        return 30 // Default
    }
    return c.MaxIterations
}
```

### Parser Implementation

```go
// internal/task/parser.go
package task

import (
    "fmt"
    "os"
    "strings"

    "gopkg.in/yaml.v3"
)

// ParseFile reads and parses a task file
func ParseFile(path string) (*Task, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read file: %w", err)
    }

    return Parse(string(content), path)
}

// Parse parses task content with front matter
func Parse(content string, filePath string) (*Task, error) {
    // Split front matter and content
    // Format: ---\nyaml\n---\nmarkdown
    parts := strings.SplitN(content, "---", 3)
    if len(parts) < 3 {
        return nil, fmt.Errorf("invalid task file format: missing front matter delimiters")
    }

    // Parse YAML front matter
    var task Task
    if err := yaml.Unmarshal([]byte(parts[1]), &task); err != nil {
        return nil, fmt.Errorf("parse front matter: %w", err)
    }

    // Set derived fields
    task.FilePath = filePath
    task.Content = strings.TrimSpace(parts[2])

    // Validate
    if err := Validate(&task); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    return &task, nil
}

// Validate checks required fields and values
func Validate(t *Task) error {
    if t.ID == "" {
        return fmt.Errorf("id is required")
    }

    if t.Title == "" {
        return fmt.Errorf("title is required")
    }

    if t.Role == "" {
        return fmt.Errorf("role is required")
    }

    // Validate priority
    switch t.Priority {
    case PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow, "":
        // Valid (empty defaults to medium)
        if t.Priority == "" {
            t.Priority = PriorityMedium
        }
    default:
        return fmt.Errorf("invalid priority %q: must be critical, high, medium, or low", t.Priority)
    }

    // Validate status
    switch t.Status {
    case StatusPending, StatusAssigned, StatusInProgress, StatusReview,
         StatusComplete, StatusFailed, StatusBlocked, "":
        // Valid (empty defaults to pending)
        if t.Status == "" {
            t.Status = StatusPending
        }
    default:
        return fmt.Errorf("invalid status %q", t.Status)
    }

    // Validate completion config
    if t.Completion != nil {
        if t.Completion.Verify == "" && t.Completion.Signal == "" {
            return fmt.Errorf("completion must have verify or signal")
        }
    }

    return nil
}
```

### Serialization (for status updates)

```go
// internal/task/serialize.go
package task

import (
    "fmt"
    "os"
    "strings"

    "gopkg.in/yaml.v3"
)

// WriteFile writes task back to file, preserving markdown content
func WriteFile(t *Task) error {
    // Marshal front matter
    frontMatter, err := yaml.Marshal(t)
    if err != nil {
        return fmt.Errorf("marshal front matter: %w", err)
    }

    // Combine with markdown content
    content := fmt.Sprintf("---\n%s---\n\n%s\n", string(frontMatter), t.Content)

    return os.WriteFile(t.FilePath, []byte(content), 0644)
}
```

### Status Values

| Status | Description | Next States |
|--------|-------------|-------------|
| `pending` | Not yet started | assigned, blocked |
| `assigned` | Agent assigned, not yet started | in_progress, pending |
| `in_progress` | Agent actively working | review, complete, failed |
| `review` | Work complete, needs human review | complete, in_progress |
| `complete` | Verified and done | (terminal) |
| `failed` | Failed, needs attention | pending, in_progress |
| `blocked` | Waiting on dependencies | pending |

### Priority Values

| Priority | Description | Queue Order |
|----------|-------------|-------------|
| `critical` | Must be done immediately | 1 (highest) |
| `high` | Important, do soon | 2 |
| `medium` | Normal priority (default) | 3 |
| `low` | Nice to have | 4 (lowest) |

### Directory Structure

```
project/
├── .tanuki/
│   └── tasks/
│       ├── TASK-001-user-auth.md
│       ├── TASK-002-api-refactor.md
│       └── TASK-003-test-coverage.md
└── ...
```

## Testing

### Unit Tests

```go
func TestParse(t *testing.T) {
    tests := []struct {
        name    string
        content string
        want    *Task
        wantErr bool
    }{
        {
            name: "valid task with completion",
            content: `---
id: TASK-001
title: Test Task
role: backend
priority: high
completion:
  verify: "npm test"
---

# Test Task

Do the thing.
`,
            want: &Task{
                ID:       "TASK-001",
                Title:    "Test Task",
                Role:     "backend",
                Priority: PriorityHigh,
                Status:   StatusPending, // Default
                Completion: &CompletionConfig{
                    Verify: "npm test",
                },
                Content: "# Test Task\n\nDo the thing.",
            },
            wantErr: false,
        },
        {
            name: "missing id",
            content: `---
title: Test Task
role: backend
---

Content
`,
            wantErr: true,
        },
        {
            name: "invalid priority",
            content: `---
id: TASK-001
title: Test Task
role: backend
priority: urgent
---

Content
`,
            wantErr: true,
        },
        {
            name: "minimal valid task",
            content: `---
id: TASK-001
title: Minimal
role: backend
---

Do it.
`,
            want: &Task{
                ID:       "TASK-001",
                Title:    "Minimal",
                Role:     "backend",
                Priority: PriorityMedium, // Default
                Status:   StatusPending,  // Default
                Content:  "Do it.",
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Parse(tt.content, "test.md")
            if (err != nil) != tt.wantErr {
                t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if tt.want != nil {
                // Compare fields...
                if got.ID != tt.want.ID {
                    t.Errorf("ID = %v, want %v", got.ID, tt.want.ID)
                }
                // ... more assertions
            }
        })
    }
}

func TestValidate(t *testing.T) {
    tests := []struct {
        name    string
        task    *Task
        wantErr bool
        errMsg  string
    }{
        {
            name:    "nil task",
            task:    nil,
            wantErr: true,
        },
        {
            name:    "empty id",
            task:    &Task{Title: "Test", Role: "backend"},
            wantErr: true,
            errMsg:  "id is required",
        },
        {
            name:    "empty completion config",
            task:    &Task{ID: "T1", Title: "T", Role: "r", Completion: &CompletionConfig{}},
            wantErr: true,
            errMsg:  "completion must have verify or signal",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.task)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
            if tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
                t.Errorf("Validate() error = %v, want containing %v", err, tt.errMsg)
            }
        })
    }
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| File not found | Error with path |
| Invalid YAML | Error with parse details |
| Missing front matter delimiters | Error: "missing front matter delimiters" |
| Missing required fields | Error with field name |
| Invalid priority/status | Error with valid options |
| Empty completion config | Error: "must have verify or signal" |

## Out of Scope

- Task creation wizard/command (create files manually)
- Task templates
- Time estimates in schema
- Subtasks / hierarchical tasks
- Custom fields

## Notes

Tasks are just markdown files. This keeps them human-readable and version-controllable. The system reads them; humans write them.

The `completion` field is optional - tasks without it will need manual verification. Tasks with `completion.verify` or `completion.signal` enable Ralph-style autonomous completion.
