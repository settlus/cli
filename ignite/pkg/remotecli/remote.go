package remotecli

import (
	"fmt"
	"strings"

	"cosmossdk.io/client/v2/autocli"
	"cosmossdk.io/client/v2/autocli/flag"

	"github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	flagUpdate = "update"
	flagConfig = "config"
	flagOutput = "output"

	outputFormatText = "text"
	outputFormatJSON = "json"

	defaultKeyringBackend = "os"
)

func InitCmd(config *Config, configDir string) *cobra.Command {
	var insecure bool

	cmd := &cobra.Command{
		Use:   "init [chain]",
		Short: "Initialize a new chain",
		Long: `To configure a new chain, run this command using the --init flag and the name of the chain as it's listed in the chain registry (https://github.com/cosmos/chain-registry).
If the chain is not listed in the chain registry, you can use any unique name.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			chainName := strings.ToLower(args[0])
			return reconfigure(cmd, config, configDir, chainName)
		},
	}

	return cmd
}

func RemoteCommand(config *Config, configDir string) ([]*cobra.Command, error) {
	commands := make([]*cobra.Command, 0)
	for chain, chainConfig := range config.Chains {
		chain, chainConfig := chain, chainConfig

		// load chain info
		chainInfo := NewChainInfo(configDir, chain, chainConfig)
		if err := chainInfo.Load(false); err != nil {
			commands = append(commands, RemoteErrorCommand(config, configDir, chain, chainConfig, err))
			continue
		}

		// add comet commands
		cometCmds := cmtservice.NewCometBFTCommands()
		chainInfo.ModuleOptions[cometCmds.Name()] = cometCmds.AutoCLIOptions()

		appOpts := autocli.AppOptions{
			ModuleOptions: chainInfo.ModuleOptions,
		}

		builder := &autocli.Builder{
			Builder: flag.Builder{
				TypeResolver: &dynamicTypeResolver{chainInfo},
				FileResolver: chainInfo.ProtoFiles,
			},
			GetClientConn: func(command *cobra.Command) (grpc.ClientConnInterface, error) {
				return chainInfo.OpenClient()
			},
			AddQueryConnFlags: func(command *cobra.Command) {},
		}

		var (
			update   bool
			reconfig bool
			insecure bool
			output   string
		)

		chainCmd := &cobra.Command{
			Use:   chain,
			Short: fmt.Sprintf("Commands for the %s chain", chain),
			RunE: func(cmd *cobra.Command, args []string) error {
				switch {
				case reconfig:
					return reconfigure(cmd, config, configDir, chain)
				case update:
					cmd.Printf("Updating AutoCLI data for %s\n", chain)
					return chainInfo.Load(true)
				default:
					return cmd.Help()
				}
			},
		}
		chainCmd.Flags().BoolVar(&update, flagUpdate, false, "update the CLI commands for the selected chain (should be used after every chain upgrade)")
		chainCmd.Flags().BoolVar(&reconfig, flagConfig, false, "re-configure the selected chain (allows choosing a new gRPC endpoint and refreshes data")
		chainCmd.PersistentFlags().StringVar(&output, flagOutput, outputFormatJSON, fmt.Sprintf("output format (%s|%s)", outputFormatText, outputFormatJSON))

		if err := appOpts.EnhanceRootCommandWithBuilder(chainCmd, builder); err != nil {
			// when enriching the command with autocli fails, we add a command that
			// will print the error and allow the user to reconfigure the chain instead
			chainCmd.RunE = func(cmd *cobra.Command, args []string) error {
				cmd.Printf("Error while loading AutoCLI data for %s: %+v\n", chain, err)
				cmd.Printf("Attempt to reconfigure the chain using the %s flag\n", flagConfig)
				if cmd.Flags().Changed(flagConfig) {
					return reconfigure(cmd, config, configDir, chain)
				}

				return nil
			}
		}

		commands = append(commands, chainCmd)
	}

	return commands, nil
}

func RemoteErrorCommand(cfg *Config, configDir, chain string, chainConfig *ChainConfig, err error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   chain,
		Short: fmt.Sprintf("Unable to load %s data", chain),
		Long:  fmt.Sprintf("Unable to load %s data, reconfiguration needed.", chain),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Printf("Error loading chain data for %s: %+v\n", chain, err)
			return reconfigure(cmd, cfg, configDir, chain)
		},
	}

	return cmd
}

type dynamicTypeResolver struct {
	*ChainInfo
}

var (
	_ protoregistry.MessageTypeResolver   = dynamicTypeResolver{}
	_ protoregistry.ExtensionTypeResolver = dynamicTypeResolver{}
)

func (d dynamicTypeResolver) FindMessageByName(message protoreflect.FullName) (protoreflect.MessageType, error) {
	desc, err := d.ProtoFiles.FindDescriptorByName(message)
	if err != nil {
		return nil, err
	}

	return dynamicpb.NewMessageType(desc.(protoreflect.MessageDescriptor)), nil
}

func (d dynamicTypeResolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	if i := strings.LastIndexByte(url, '/'); i >= 0 {
		url = url[i+len("/"):]
	}

	return d.FindMessageByName(protoreflect.FullName(url))
}

func (d dynamicTypeResolver) FindExtensionByName(field protoreflect.FullName) (protoreflect.ExtensionType, error) {
	desc, err := d.ProtoFiles.FindDescriptorByName(field)
	if err != nil {
		return nil, err
	}

	return dynamicpb.NewExtensionType(desc.(protoreflect.ExtensionTypeDescriptor)), nil
}

func (d dynamicTypeResolver) FindExtensionByNumber(message protoreflect.FullName, field protoreflect.FieldNumber) (protoreflect.ExtensionType, error) {
	desc, err := d.ProtoFiles.FindDescriptorByName(message)
	if err != nil {
		return nil, err
	}

	messageDesc := desc.(protoreflect.MessageDescriptor)
	exts := messageDesc.Extensions()
	n := exts.Len()
	for i := 0; i < n; i++ {
		ext := exts.Get(i)
		if ext.Number() == field {
			return dynamicpb.NewExtensionType(ext), nil
		}
	}

	return nil, protoregistry.NotFound
}
