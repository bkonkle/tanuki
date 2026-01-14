package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/bkonkle/tanuki/internal/role"
	"github.com/spf13/cobra"
)

// validRoleNamePattern enforces role name constraints:
// - Must be lowercase alphanumeric with hyphens
var validRoleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z][a-z0-9]?$`)

var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage roles",
	Long: `Manage role configurations for agents.

Roles define specialized agent behavior through system prompts, tool restrictions,
and context files. Use built-in roles (backend, frontend, qa, docs, devops, fullstack)
or create custom roles.`,
}

var roleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available roles",
	Long: `List all available roles including both built-in and project-specific roles.

Project roles in .tanuki/roles/ override built-in roles with the same name.`,
	RunE: runRoleList,
}

var roleShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show role details",
	Long: `Display detailed information about a specific role.

Shows the system prompt, allowed/disallowed tools, context files, and other configuration.`,
	Args: cobra.ExactArgs(1),
	RunE: runRoleShow,
}

var roleInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default role files",
	Long: `Initialize the .tanuki/roles/ directory with default role configuration files.

This creates editable YAML files for all built-in roles, which you can customize
for your project's needs. Existing files are not overwritten.`,
	RunE: runRoleInit,
}

var (
	roleCreateBasedOn     string
	roleCreateDescription string
)

var roleCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new role from template",
	Long: `Create a new custom role from a template.

Examples:
  # Create a blank role template
  tanuki role create ml-engineer

  # Base new role on existing role
  tanuki role create ml-engineer --based-on backend

  # Create with description
  tanuki role create ml-engineer --description "ML model development"
`,
	Args: cobra.ExactArgs(1),
	RunE: runRoleCreate,
}

func init() {
	roleCreateCmd.Flags().StringVar(&roleCreateBasedOn, "based-on", "", "Base new role on existing role")
	roleCreateCmd.Flags().StringVarP(&roleCreateDescription, "description", "d", "", "Role description")

	roleCmd.AddCommand(roleListCmd)
	roleCmd.AddCommand(roleShowCmd)
	roleCmd.AddCommand(roleInitCmd)
	roleCmd.AddCommand(roleCreateCmd)
	rootCmd.AddCommand(roleCmd)
}

func runRoleList(_ *cobra.Command, _ []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	roleMgr := role.NewManager(projectRoot)
	roles, err := roleMgr.List()
	if err != nil {
		return fmt.Errorf("failed to list roles: %w", err)
	}

	if len(roles) == 0 {
		fmt.Println("No roles available.")
		fmt.Println("\nUse 'tanuki role init' to create default role files.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION\tSOURCE")
	_, _ = fmt.Fprintln(w, "----\t-----------\t------")

	for _, r := range roles {
		source := "project"
		if r.Builtin {
			source = "builtin"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", r.Name, r.Description, source)
	}

	return w.Flush()
}

func runRoleShow(_ *cobra.Command, args []string) error {
	roleName := args[0]

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	roleMgr := role.NewManager(projectRoot)
	r, err := roleMgr.Get(roleName)
	if err != nil {
		// Suggest available roles
		roles, _ := roleMgr.List()
		roleNames := make([]string, len(roles))
		for i, role := range roles {
			roleNames[i] = role.Name
		}
		return fmt.Errorf("role %q not found\n\nAvailable roles: %s", roleName, strings.Join(roleNames, ", "))
	}

	fmt.Printf("Name: %s\n", r.Name)
	fmt.Printf("Description: %s\n", r.Description)

	source := "project"
	if r.Builtin {
		source = "builtin"
	}
	fmt.Printf("Source: %s\n", source)
	fmt.Println()

	fmt.Println("System Prompt:")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println(r.SystemPrompt)
	fmt.Println(strings.Repeat("-", 60))
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

func runRoleInit(_ *cobra.Command, _ []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	roleMgr := role.NewManager(projectRoot)
	if err := roleMgr.InitRoles(); err != nil {
		return fmt.Errorf("failed to initialize roles: %w", err)
	}

	rolesDir := filepath.Join(projectRoot, ".tanuki", "roles")
	fmt.Printf("Created default roles in %s\n", rolesDir)
	fmt.Println()
	fmt.Println("Available roles:")
	for _, name := range role.ListBuiltinRoleNames() {
		fmt.Printf("  - %s\n", name)
	}
	fmt.Println()
	fmt.Println("Customize these files or create new roles.")
	fmt.Println("Use with: tanuki spawn <name> --role <role>")

	return nil
}

func runRoleCreate(_ *cobra.Command, args []string) error {
	name := args[0]

	// Validate role name
	if !isValidRoleName(name) {
		return fmt.Errorf("invalid role name: %q (must be lowercase alphanumeric with hyphens)", name)
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	roleMgr := role.NewManager(projectRoot)

	// Check if role already exists
	if _, err := roleMgr.Get(name); err == nil {
		return fmt.Errorf("role %q already exists\nUse 'tanuki role show %s' to view it", name, name)
	}

	var template *role.Role

	if roleCreateBasedOn != "" {
		// Load base role
		base, err := roleMgr.Get(roleCreateBasedOn)
		if err != nil {
			// Suggest available roles
			roles, _ := roleMgr.List()
			roleNames := make([]string, len(roles))
			for i, r := range roles {
				roleNames[i] = r.Name
			}
			return fmt.Errorf("base role %q not found\n\nAvailable roles: %s", roleCreateBasedOn, strings.Join(roleNames, ", "))
		}

		template = base.Clone()
		template.Name = name
		template.Builtin = false
		fmt.Printf("Creating role %q based on %q\n", name, roleCreateBasedOn)
	} else {
		// Create default template
		template = role.DefaultTemplate(name)
		fmt.Printf("Creating role %q\n", name)
	}

	// Override description if provided
	if roleCreateDescription != "" {
		template.Description = roleCreateDescription
	}

	// Write role file
	roleFile := filepath.Join(projectRoot, ".tanuki", "roles", name+".yaml")
	if err := role.WriteRoleTemplate(roleFile, template); err != nil {
		return fmt.Errorf("write role file: %w", err)
	}

	fmt.Printf("\nCreated role file: %s\n", roleFile)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit the role file to customize behavior:")
	fmt.Printf("     vim %s\n", roleFile)
	fmt.Println("  2. Use with: tanuki spawn <agent> --role", name)

	return nil
}

// isValidRoleName checks if a role name is valid.
func isValidRoleName(name string) bool {
	if len(name) < 1 || len(name) > 63 {
		return false
	}
	return validRoleNamePattern.MatchString(name)
}
