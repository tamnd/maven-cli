package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func (a *App) artifactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact <groupId:artifactId>",
		Short: "Show latest info for a specific artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupID, artifactID, err := parseGA(args[0])
			if err != nil {
				return codeError(exitUsage, err)
			}
			a.progressf("fetching %s:%s...", groupID, artifactID)
			art, err := a.client.Artifact(cmd.Context(), groupID, artifactID)
			if err != nil {
				return codeError(exitError, err)
			}
			return a.render(art)
		},
	}
	return cmd
}

func parseGA(s string) (groupID, artifactID string, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid format %q: expected groupId:artifactId", s)
	}
	return parts[0], parts[1], nil
}
