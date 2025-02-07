package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	flaghelper "github.com/hashicorp/nomad/helper/flags"
	"github.com/posener/complete"
)

type VolumeSnapshotCreateCommand struct {
	Meta
}

func (c *VolumeSnapshotCreateCommand) Help() string {
	helpText := `
Usage: nomad volume snapshot create <volume id> [snapshot_name]

  Create a snapshot of an external storage volume. This command requires a
  volume ID or prefix. If there is an exact match based on the provided volume
  ID or prefix, then the specific volume is snapshotted. Otherwise, a list of
  matching volumes and information will be displayed. The volume must still be
  registered with Nomad in order to be snapshotted.

  If an optional snapshot name is provided, the argument will be passed to the
  CSI plugin to be used as the ID of the resulting snapshot. Not all plugins
  accept this name and it may be ignored.

  When ACLs are enabled, this command requires a token with the
  'csi-write-volume' capability for the volume's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Snapshot Create Options:

  -parameter
    Parameters to pass to the plugin to create a snapshot. Accepts multiple
    flags in the form -parameter key=value

  -secret
    Secrets to pass to the plugin to create snapshot. Accepts multiple
    flags in the form -secret key=value

  -verbose
    Display full information for the resulting snapshot.
`

	return strings.TrimSpace(helpText)
}

func (c *VolumeSnapshotCreateCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{})
}

func (c *VolumeSnapshotCreateCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Volumes, nil)
		if err != nil {
			return []string{}
		}
		matches := resp.Matches[contexts.Volumes]
		return matches
	})
}

func (c *VolumeSnapshotCreateCommand) Synopsis() string {
	return "Snapshot a volume"
}

func (c *VolumeSnapshotCreateCommand) Name() string { return "volume snapshot create" }

func (c *VolumeSnapshotCreateCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	var verbose bool
	var parametersArgs flaghelper.StringFlag
	var secretsArgs flaghelper.StringFlag
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.Var(&parametersArgs, "parameter", "parameters for snapshot, ex. -parameter key=value")
	flags.Var(&secretsArgs, "secret", "secrets for snapshot, ex. -secret key=value")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing arguments %s", err))
		return 1
	}

	// Check that we at least one argument
	args = flags.Args()
	if l := len(args); l == 0 {
		c.Ui.Error("This command takes at least one argument: <vol id> [snapshot name]")
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	volID := args[0]
	snapshotName := ""
	if len(args) == 2 {
		snapshotName = args[1]
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	secrets := api.CSISecrets{}
	for _, kv := range secretsArgs {
		s := strings.Split(kv, "=")
		if len(s) == 2 {
			secrets[s[0]] = s[1]
		} else {
			c.Ui.Error("Secret must be in the format: -secret key=value")
			return 1
		}
	}

	params := map[string]string{}
	for _, kv := range parametersArgs {
		p := strings.Split(kv, "=")
		if len(p) == 2 {
			params[p[0]] = p[1]
		}
	}

	snaps, _, err := client.CSIVolumes().CreateSnapshot(&api.CSISnapshot{
		SourceVolumeID: volID,
		Name:           snapshotName,
		Secrets:        secrets,
		Parameters:     params,
	}, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error snapshotting volume: %s", err))
		return 1
	}

	c.Ui.Output(csiFormatSnapshots(snaps.Snapshots, verbose))
	return 0
}
