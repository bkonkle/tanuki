---
id: TANK-022
title: Role Management Commands
status: todo
priority: medium
estimate: S
depends_on: [TANK-023]
workstream: C
phase: 2
---

# Role Management Commands

## Summary

Implement commands for listing and inspecting roles.

## Acceptance Criteria

- [ ] `tanuki role list` - Show available roles
- [ ] `tanuki role show <name>` - Show role details
- [ ] `tanuki role init` - Create default role files

## Technical Details

### Commands

```go
var roleCmd = &cobra.Command{
    Use:   "role",
    Short: "Manage roles",
}

var roleListCmd = &cobra.Command{
    Use:   "list",
    Short: "List available roles",
    RunE:  runRoleList,
}

var roleShowCmd = &cobra.Command{
    Use:   "show <name>",
    Short: "Show role details",
    Args:  cobra.ExactArgs(1),
    RunE:  runRoleShow,
}

var roleInitCmd = &cobra.Command{
    Use:   "init",
    Short: "Create default role files",
    RunE:  runRoleInit,
}
```

### List Implementation

```go
func runRoleList(cmd *cobra.Command, args []string) error {
    cfg, _ := config.Load()
    roleMgr := role.NewManager(cfg)

    roles, err := roleMgr.List()
    if err != nil {
        return err
    }

    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "NAME\tDESCRIPTION\tSOURCE")
    fmt.Fprintln(w, "----\t-----------\t------")

    for _, r := range roles {
        source := "project"
        if r.Builtin {
            source = "builtin"
        }
        fmt.Fprintf(w, "%s\t%s\t%s\n", r.Name, r.Description, source)
    }

    return w.Flush()
}
```

### Show Implementation

```go
func runRoleShow(cmd *cobra.Command, args []string) error {
    roleName := args[0]

    cfg, _ := config.Load()
    roleMgr := role.NewManager(cfg)

    r, err := roleMgr.Get(roleName)
    if err != nil {
        return fmt.Errorf("role %q not found", roleName)
    }

    fmt.Printf("Name: %s\n", r.Name)
    fmt.Printf("Description: %s\n", r.Description)
    fmt.Println()

    fmt.Println("System Prompt:")
    fmt.Println("─────────────")
    fmt.Println(r.SystemPrompt)
    fmt.Println()

    if len(r.AllowedTools) > 0 {
        fmt.Printf("Allowed Tools: %s\n", strings.Join(r.AllowedTools, ", "))
    }

    if len(r.DisallowedTools) > 0 {
        fmt.Printf("Disallowed Tools: %s\n", strings.Join(r.DisallowedTools, ", "))
    }

    if len(r.ContextFiles) > 0 {
        fmt.Println("\nContext Files:")
        for _, f := range r.ContextFiles {
            fmt.Printf("  - %s\n", f)
        }
    }

    if r.Model != "" {
        fmt.Printf("\nModel: %s\n", r.Model)
    }

    if r.MaxTurns > 0 {
        fmt.Printf("Max Turns: %d\n", r.MaxTurns)
    }

    return nil
}
```

### Init Implementation

```go
func runRoleInit(cmd *cobra.Command, args []string) error {
    cfg, _ := config.Load()
    roleMgr := role.NewManager(cfg)

    if err := roleMgr.InitRoles(); err != nil {
        return err
    }

    fmt.Println("Created default roles in .tanuki/roles/")
    fmt.Println()
    fmt.Println("Available roles:")
    fmt.Println("  - backend")
    fmt.Println("  - frontend")
    fmt.Println("  - qa")
    fmt.Println("  - docs")
    fmt.Println()
    fmt.Println("Customize these files or create new roles.")
    fmt.Println("Use with: tanuki spawn <name> --role <role>")

    return nil
}
```

### Output

```
$ tanuki role list
NAME       DESCRIPTION                    SOURCE
----       -----------                    ------
backend    Backend development specialist builtin
frontend   Frontend development specialist builtin
qa         Quality assurance specialist   builtin
docs       Documentation specialist       builtin
custom     My custom role                 project
```

```
$ tanuki role show backend
Name: backend
Description: Backend development specialist

System Prompt:
─────────────
You are a backend development specialist. Focus on:
- API design and implementation
- Database operations and optimization
- Server-side business logic
- Security best practices

Always write tests for new functionality.

Allowed Tools: Read, Write, Edit, Bash, Glob, Grep, TodoWrite

Context Files:
  - docs/architecture.md
  - docs/api-conventions.md
```

```
$ tanuki role init
Created default roles in .tanuki/roles/

Available roles:
  - backend
  - frontend
  - qa
  - docs

Customize these files or create new roles.
Use with: tanuki spawn <name> --role <role>
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Role not found | Error with list of available roles |
| No roles directory | Suggest running `tanuki role init` |

## Out of Scope

- Role creation command (edit YAML directly)
- Role deletion
- Role validation command

## Notes

Keep these commands simple - roles are meant to be edited as YAML files, not managed through CLI.
