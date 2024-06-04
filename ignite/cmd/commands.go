package ignitecmd

import (
	"github.com/spf13/cobra"

	"github.com/ignite/cli/v29/ignite/pkg/remotecli"
)

// NewCommands returns a command that groups all commands from the current chain folder.
func NewCommands() (*cobra.Command, error) {
	c := &cobra.Command{
		Use: "commands [command]",
		//Short: "Build, init and start a blockchain node",
		//Long: `Commands in this namespace let you to build, initialize, and start your
		//blockchain node locally for development purposes.
		//`,
	}
	configDir, err := remotecli.GetConfigDir()
	if err != nil {
		return nil, err
	}

	cfg, err := remotecli.Load(configDir)
	if err != nil {
		return nil, err
	}

	// add commands
	commands, err := remotecli.RemoteCommand(cfg, configDir)
	if err != nil {
		return nil, err
	}
	commands = append(
		commands,
		remotecli.InitCmd(cfg, configDir),
	)

	c.AddCommand(commands...)
	return c, nil
}
