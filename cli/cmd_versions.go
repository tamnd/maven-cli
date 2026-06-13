package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) versionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions <groupId:artifactId>",
		Short: "List versions of an artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupID, artifactID, err := parseGA(args[0])
			if err != nil {
				return codeError(exitUsage, err)
			}
			n := a.effectiveLimit(10)
			a.progressf("fetching versions for %s:%s...", groupID, artifactID)
			versions, err := a.client.Versions(cmd.Context(), groupID, artifactID, n)
			if err != nil {
				return codeError(exitError, err)
			}
			return a.renderOrEmpty(versions, len(versions))
		},
	}
	return cmd
}
