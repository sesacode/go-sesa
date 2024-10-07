// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package launcher

import (
	"fmt"
	"strings"

	"gopkg.in/urfave/cli.v1"

	"github.com/sesanetwork/go-sesa/cmd/utils"
	"github.com/sesanetwork/go-sesa/console"
	"github.com/sesanetwork/go-sesa/node"
	"github.com/sesanetwork/go-sesa/rpc"
)

var (
	consoleFlags = []cli.Flag{utils.JSpathFlag, utils.ExecFlag, utils.PreloadJSFlag}

	consoleCommand = cli.Command{
		Action:   utils.MigrateFlags(localConsole),
		Name:     "console",
		Usage:    "Start an interactive JavaScript environment",
		Flags:    append(append(append(nodeFlags, rpcFlags...), consoleFlags...), testFlags...),
		Category: "CONSOLE COMMANDS",
		Description: `
The sesa console is an interactive shell for the JavaScript runtime environment
which exposes a node admin interface as well as the Ðapp JavaScript API.
See https://github.com/sesanetwork/go-sesa/libs/wiki/JavaScript-Console.`,
	}

	attachCommand = cli.Command{
		Action:    utils.MigrateFlags(remoteConsole),
		Name:      "attach",
		Usage:     "Start an interactive JavaScript environment (connect to node)",
		ArgsUsage: "[endpoint]",
		Flags:     append(consoleFlags, DataDirFlag),
		Category:  "CONSOLE COMMANDS",
		Description: `
The sesa console is an interactive shell for the JavaScript runtime environment
which exposes a node admin interface as well as the Ðapp JavaScript API.
See https://github.com/sesanetwork/go-sesa/libs/wiki/JavaScript-Console.
This command allows to open a console on a running sesa node.`,
	}

	javascriptCommand = cli.Command{
		Action:    utils.MigrateFlags(ephemeralConsole),
		Name:      "js",
		Usage:     "Execute the specified JavaScript files",
		ArgsUsage: "<jsfile> [jsfile...]",
		Flags:     append(append(nodeFlags, consoleFlags...), testFlags...),
		Category:  "CONSOLE COMMANDS",
		Description: `
The JavaScript VM exposes a node admin interface as well as the Ðapp
JavaScript API. See https://github.com/sesanetwork/go-sesa/libs/wiki/JavaScript-Console`,
	}
)

// localConsole starts a new sesa node, attaching a JavaScript console to it at the
// same time.
func localConsole(ctx *cli.Context) error {
	// Create and start the node based on the CLI flags
	cfg := makeAllConfigs(ctx)
	genesisStore := mayGetGenesisStore(ctx)
	node, _, nodeClose := makeNode(ctx, cfg, genesisStore)
	startNode(ctx, node)
	defer nodeClose()

	// Attach to the newly started node and start the JavaScript console
	client, err := node.Attach()
	if err != nil {
		utils.Fatalf("Failed to attach to the inproc sesa: %v", err)
	}
	config := console.Config{
		DataDir: utils.MakeDataDir(ctx),
		DocRoot: ctx.GlobalString(utils.JSpathFlag.Name),
		Client:  client,
		Preload: utils.MakeConsolePreloads(ctx),
	}

	console, err := console.New(config)
	if err != nil {
		utils.Fatalf("Failed to start the JavaScript console: %v", err)
	}
	defer console.Stop(false)

	// If only a short execution was requested, evaluate and return
	if script := ctx.GlobalString(utils.ExecFlag.Name); script != "" {
		console.Evaluate(script)
		return nil
	}
	// Otherwise print the welcome screen and enter interactive mode
	console.Welcome()
	console.Interactive()

	return nil
}

// remoteConsole will connect to a remote sesa instance, attaching a JavaScript
// console to it.
func remoteConsole(ctx *cli.Context) error {
	// Attach to a remotely running sesa instance and start the JavaScript console
	endpoint := ctx.Args().First()
	if endpoint == "" {
		path := DefaultDataDir()
		if ctx.GlobalIsSet(DataDirFlag.Name) {
			path = ctx.GlobalString(DataDirFlag.Name)
		}
		endpoint = fmt.Sprintf("%s/sesa.ipc", path)
	}
	client, err := dialRPC(endpoint)
	if err != nil {
		utils.Fatalf("Unable to attach to remote sesa: %v", err)
	}
	config := console.Config{
		DataDir: utils.MakeDataDir(ctx),
		DocRoot: ctx.GlobalString(utils.JSpathFlag.Name),
		Client:  client,
		Preload: utils.MakeConsolePreloads(ctx),
	}

	console, err := console.New(config)
	if err != nil {
		utils.Fatalf("Failed to start the JavaScript console: %v", err)
	}
	defer console.Stop(false)

	if script := ctx.GlobalString(utils.ExecFlag.Name); script != "" {
		console.Evaluate(script)
		return nil
	}

	// Otherwise print the welcome screen and enter interactive mode
	console.Welcome()
	console.Interactive()

	return nil
}

// dialRPC returns a RPC client which connects to the given endpoint.
// The check for empty endpoint implements the defaulting logic
// for "sesa attach" and "sesa monitor" with no argument.
func dialRPC(endpoint string) (*rpc.Client, error) {
	if endpoint == "" {
		endpoint = node.DefaultIPCEndpoint(clientIdentifier)
	} else if strings.HasPrefix(endpoint, "rpc:") || strings.HasPrefix(endpoint, "ipc:") {
		// Backwards compatibility with rpc which required these prefixes.
		endpoint = endpoint[4:]
	}
	return rpc.Dial(endpoint)
}

// ephemeralConsole starts a new sesa node, attaches an ephemeral JavaScript
// console to it, executes each of the files specified as arguments and tears
// everything down.
func ephemeralConsole(ctx *cli.Context) error {
	var b strings.Builder
	for _, file := range ctx.Args() {
		b.Write([]byte(fmt.Sprintf("loadScript('%s');", file)))
	}
	utils.Fatalf(`The "js" command is deprecated. Please use the following instead:
sesa --exec "%s" console`, b.String())
	return nil
}
