// main.go
// This is a HelloWorld-like example

package main

import (
	"os"

	"github.com/mkideal/cli"
)

type arguments struct {
	Name string `cli:"n,name" usage:"tell me your name"`
	Help bool   `cli:"h,help" usage:"show help"`
}

// AutoHelp implements cli.AutoHelper interface
// NOTE: cli.Helper is a predefined type which implements cli.AutoHelper
func (argv *arguments) AutoHelp() bool {
	return argv.Help
}

func run(ctx *cli.Context) error {
	argv := ctx.Argv().(*arguments)
	ctx.String("Hello, %s!\n", argv.Name)
	return nil
}

func main() {
	os.Exit(cli.Run(new(arguments), run))
}
