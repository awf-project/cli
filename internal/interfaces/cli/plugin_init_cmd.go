package cli

import "github.com/spf13/cobra"

func newPluginInitCommand() *cobra.Command {
	var flags pluginInitFlags

	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Scaffold an AWF plugin repository",
		Long: `Scaffold an AWF plugin repository.

Supported kinds:
  operation  Generate an operation plugin scaffold.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			options, err := parsePluginInitOptions(args, flags)
			if err != nil {
				return err
			}

			if err := generatePluginRepository(cmd.Context(), options, cmd.OutOrStdout()); err != nil {
				return err
			}

			return ensurePluginInitGoFlags()
		},
	}

	cmd.Flags().StringArrayVar(
		&flags.kind,
		"kind",
		nil,
		"plugin scaffold kind (supported: operation)",
	)
	cmd.Flags().StringVar(&flags.output, "output", "", "output directory")
	cmd.Flags().BoolVar(&flags.force, "force", false, "overwrite an existing output directory")

	return cmd
}
