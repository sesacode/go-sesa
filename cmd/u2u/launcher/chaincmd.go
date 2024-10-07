package launcher

import (
	"gopkg.in/urfave/cli.v1"

	"github.com/sesanetwork/go-sesa/cmd/utils"
)

var (
	EvmExportMode = cli.StringFlag{
		Name:  "export.evm.mode",
		Usage: `EVM export mode ("full" or "ext-mpt" or "mpt")`,
		Value: "mpt",
	}
	EvmExportExclude = cli.StringFlag{
		Name:  "export.evm.exclude",
		Usage: `DB of EVM keys to exclude from genesis`,
	}
	GenesisExportSections = cli.StringFlag{
		Name:  "export.sections",
		Usage: `Genesis sections to export separated by comma (e.g. "brs-1" or "ers" or "evm-2")`,
		Value: "brs,ers,evm",
	}
	importCommand = cli.Command{
		Name:      "import",
		Usage:     "Import a blockchain file",
		ArgsUsage: "<filename> (<filename 2> ... <filename N>) [check=false]",
		Category:  "MISCELLANEOUS COMMANDS",
		Description: `
    sesa import events

The import command imports events from an RLP-encoded files.
Events are fully verified by default, unless overridden by check=false flag.`,

		Subcommands: []cli.Command{
			{
				Action:    utils.MigrateFlags(importEvents),
				Name:      "events",
				Usage:     "Import blockchain events",
				ArgsUsage: "<filename> (<filename 2> ... <filename N>)",
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: `
The import command imports events from RLP-encoded files.
Events are fully verified by default, unless overridden by --check=false flag.`,
			},
			{
				Action:    utils.MigrateFlags(importEvm),
				Name:      "evm",
				Usage:     "Import EVM storage",
				ArgsUsage: "<filename> (<filename 2> ... <filename N>)",
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: `
    sesa import evm

The import command imports EVM storage (trie nodes, code, preimages) from files.`,
			},
			{
				Name:      "txtracer",
				Usage:     "Import transaction traces",
				ArgsUsage: "<filename>",
				Action:    utils.MigrateFlags(importTxTracer),
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: `
			sesa import txtracer

			The import command imports transaction traces and replaces the old ones 
			with traces from a file.
			`,
			},
		},
	}
	exportCommand = cli.Command{
		Name:     "export",
		Usage:    "Export blockchain",
		Category: "MISCELLANEOUS COMMANDS",

		Subcommands: []cli.Command{
			{
				Name:      "events",
				Usage:     "Export blockchain events",
				ArgsUsage: "<filename> [<epochFrom> <epochTo>]",
				Action:    utils.MigrateFlags(exportEvents),
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: `
    sesa export events

Requires a first argument of the file to write to.
Optional second and third arguments control the first and
last epoch to write. If the file ends with .gz, the output will
be gzipped
`,
			},
			{
				Name:      "genesis",
				Usage:     "Export current state into a genesis file",
				ArgsUsage: "<filename or dry-run> [<epochFrom> <epochTo>] [--export.evm.mode=MODE --export.evm.exclude=DB_PATH --export.sections=A,B,C]",
				Action:    utils.MigrateFlags(exportGenesis),
				Flags: []cli.Flag{
					DataDirFlag,
					EvmExportMode,
					EvmExportExclude,
					GenesisExportSections,
				},
				Description: `
    sesa export genesis

Export current state into a genesis file.
Requires a first argument of the file to write to.
Optional second and third arguments control the first and
last epoch to write.
Pass dry-run instead of filename for calculation of hashes without exporting data.
EVM export mode is configured with --export.evm.mode.
`,
			},
			{
				Name:      "evm-keys",
				Usage:     "Export EVM node keys",
				ArgsUsage: "<directory>",
				Action:    utils.MigrateFlags(exportEvmKeys),
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: `
    sesa export evm-keys

Requires a first argument of the DB directory to write to.
`,
			},
			{
				Name:      "txtraces",
				Usage:     "Export stored transaction traces",
				ArgsUsage: "<filename> [<blockFrom> <blockTo>]",
				Action:    utils.MigrateFlags(exportTxTracer),
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: `
    client export txtraces

Requires a first argument of the file to write to.
Optional second and third arguments control the first and
last block to write transaction traces. If the file ends with .gz, the output will
be gzipped
`,
			},
		},
	}
	deleteCommand = cli.Command{
		Name:     "delete",
		Usage:    "Delete blockchain data",
		Category: "MISCELLANEOUS COMMANDS",

		Subcommands: []cli.Command{
			{
				Name:      "txtracer",
				Usage:     "Delete transaction traces",
				ArgsUsage: "[<blockFrom> <blockTo>]",
				Action:    utils.MigrateFlags(deleteTxTracer),
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: `
    client delete txtracer

Optional first and second arguments control the first and
last block to delete transaction traces from. If the file ends with .gz, the output will
be gzipped
`,
			},
		},
	}
	checkCommand = cli.Command{
		Name:     "check",
		Usage:    "Check blockchain",
		Category: "MISCELLANEOUS COMMANDS",

		Subcommands: []cli.Command{
			{
				Name:   "evm",
				Usage:  "Check EVM storage",
				Action: utils.MigrateFlags(checkEvm),
				Flags: []cli.Flag{
					DataDirFlag,
				},
				Description: `
    sesa check evm

Checks EVM storage roots and code hashes
`,
			},
		},
	}
)
