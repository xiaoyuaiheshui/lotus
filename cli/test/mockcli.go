package test

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
	lcli "github.com/urfave/cli/v2"
)

type MockCLI struct {
	t    *testing.T
	cmds []*lcli.Command
	cctx *lcli.Context
	out  *bytes.Buffer
}

func NewMockCLI(t *testing.T, cmds []*lcli.Command) *MockCLI {
	// Create a CLI App with an --api-url flag so that we can specify which node
	// the command should be executed against
	app := &lcli.App{
		Flags: []lcli.Flag{
			&lcli.StringFlag{
				Name:   "api-url",
				Hidden: true,
			},
		},
		Commands: cmds,
	}

	var out bytes.Buffer
	app.Writer = &out
	app.Setup()

	cctx := lcli.NewContext(app, &flag.FlagSet{}, nil)
	return &MockCLI{t: t, cmds: cmds, cctx: cctx, out: &out}
}

func (c *MockCLI) Client(addr multiaddr.Multiaddr) *mockCLIClient {
	return &mockCLIClient{t: c.t, cmds: c.cmds, addr: addr, cctx: c.cctx, out: c.out}
}

// mockCLIClient runs commands against a particular node
type mockCLIClient struct {
	t    *testing.T
	cmds []*lcli.Command
	addr multiaddr.Multiaddr
	cctx *lcli.Context
	out  *bytes.Buffer
}

func (c *mockCLIClient) run(cmd []string, params []string, args []string) string {
	// Add parameter --api-url=<node api listener address>
	apiFlag := "--api-url=" + c.addr.String()
	params = append([]string{apiFlag}, params...)

	err := c.cctx.App.Run(append(append(cmd, params...), args...))
	require.NoError(c.t, err)

	// Get the output
	str := strings.TrimSpace(c.out.String())
	c.out.Reset()
	return str
}

func (c *mockCLIClient) RunCmd(input []string) string {
	out, err := c.RunCmdRaw(input)
	require.NoError(c.t, err)

	return out
}

// Given an input, find the corresponding command or sub-command.
// eg "paych add-funds"
func (c *mockCLIClient) cmdByNameSub(input []string) (*lcli.Command, []string) {
	name := input[0]
	for _, cmd := range c.cmds {
		if cmd.Name == name {
			return c.findSubcommand(cmd, input[1:])
		}
	}
	return nil, []string{}
}

func (c *mockCLIClient) findSubcommand(cmd *lcli.Command, input []string) (*lcli.Command, []string) {
	// If there are no sub-commands, return the current command
	if len(cmd.Subcommands) == 0 {
		return cmd, input
	}

	// Check each sub-command for a match against the name
	subName := input[0]
	for _, subCmd := range cmd.Subcommands {
		if subCmd.Name == subName {
			// Found a match, recursively search for sub-commands
			return c.findSubcommand(subCmd, input[1:])
		}
	}
	return nil, []string{}
}

func (c *mockCLIClient) RunCmdRaw(input []string) (string, error) {
	cmd, input := c.cmdByNameSub(input)
	if cmd == nil {
		panic("Could not find command " + input[0] + " " + input[1])
	}

	// prepend --api-url=<node api listener address>
	apiFlag := "--api-url=" + c.addr.String()
	input = append([]string{apiFlag}, input...)

	fs := c.flagSet(cmd)
	err := fs.Parse(input)
	require.NoError(c.t, err)

	err = cmd.Action(lcli.NewContext(c.cctx.App, fs, c.cctx))

	// Get the output
	str := strings.TrimSpace(c.out.String())
	c.out.Reset()
	return str, err
}

func (c *mockCLIClient) flagSet(cmd *lcli.Command) *flag.FlagSet {
	// Apply app level flags (so we can process --api-url flag)
	fs := &flag.FlagSet{}
	for _, f := range c.cctx.App.Flags {
		err := f.Apply(fs)
		if err != nil {
			c.t.Fatal(err)
		}
	}
	// Apply command level flags
	for _, f := range cmd.Flags {
		err := f.Apply(fs)
		if err != nil {
			c.t.Fatal(err)
		}
	}
	return fs
}

func (c *mockCLIClient) RunInteractiveCmd(cmd []string, interactive []string) string {
	c.toStdin(strings.Join(interactive, "\n") + "\n")
	return c.RunCmd(cmd)
}

func (c *mockCLIClient) toStdin(s string) {
	c.cctx.App.Metadata["stdin"] = bytes.NewBufferString(s)
}
