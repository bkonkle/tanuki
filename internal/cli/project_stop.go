package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var projectStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop all project agents",
	Long: `Gracefully stops all agents spawned for this project.

Tasks that are in progress will be set back to pending status so they
can be resumed later. Use --remove to also remove the agents after stopping.`,
	RunE: runProjectStop,
}

func init() {
	projectStopCmd.Flags().Bool("remove", false, "Remove agents after stopping")
	projectCmd.AddCommand(projectStopCmd)
}

func runProjectStop(cmd *cobra.Command, args []string) error {
	remove, _ := cmd.Flags().GetBool("remove")

	fmt.Println("Stopping project agents...")

	// TODO: Integrate with agent manager
	// agents, _ := agentMgr.List()
	// projectAgents := filterProjectAgents(agents)
	//
	// if len(projectAgents) == 0 {
	//     fmt.Println("No project agents running.")
	//     return nil
	// }
	//
	// for _, ag := range projectAgents {
	//     // Unassign any current task
	//     if ag.CurrentTask != "" {
	//         taskMgr.UpdateStatus(ag.CurrentTask, task.StatusPending)
	//         taskMgr.Unassign(ag.CurrentTask)
	//     }
	//
	//     if remove {
	//         fmt.Printf("  Removing %s...\n", ag.Name)
	//         if err := agentMgr.Remove(ag.Name, agent.RemoveOptions{Force: true}); err != nil {
	//             fmt.Printf("    Failed: %v\n", err)
	//             continue
	//         }
	//     } else {
	//         fmt.Printf("  Stopping %s...\n", ag.Name)
	//         if err := agentMgr.Stop(ag.Name); err != nil {
	//             fmt.Printf("    Failed: %v\n", err)
	//             continue
	//         }
	//     }
	// }

	// Placeholder output
	fmt.Println("  [placeholder] No agents to stop")

	fmt.Println()
	if remove {
		fmt.Println("Project agents removed.")
	} else {
		fmt.Println("Project stopped. Resume with: tanuki project resume")
	}

	return nil
}

// filterProjectAgents returns agents that have roles assigned (project agents).
// func filterProjectAgents(agents []*agent.Agent) []*agent.Agent {
//     var result []*agent.Agent
//     for _, ag := range agents {
//         if ag.Role != "" {
//             result = append(result, ag)
//         }
//     }
//     return result
// }
