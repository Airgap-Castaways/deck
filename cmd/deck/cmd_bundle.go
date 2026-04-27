package main

import (
	"github.com/Airgap-Castaways/deck/internal/bundlecli"
	"github.com/Airgap-Castaways/deck/internal/initcli"
)

func executeInit(env *cliEnv, output string) error {
	return initcli.Run(initcli.Options{
		Output:       output,
		DeckWorkDir:  deckWorkDirName,
		StdoutPrintf: env.stdoutPrintf,
	})
}

func executeBundleVerify(env *cliEnv, filePath string, positionalArgs []string, output string) error {
	resolvedOutput, err := resolveOutputFormat(output)
	if err != nil {
		return err
	}
	return bundlecli.Verify(bundlecli.VerifyOptions{
		FilePath:       filePath,
		PositionalArgs: positionalArgs,
		Output:         resolvedOutput,
		Verbosef:       env.verbosef,
		JSONEncoder: func(v any) error {
			enc := env.stdoutJSONEncoder()
			enc.SetIndent("", "  ")
			return enc.Encode(v)
		},
		StdoutPrintf: env.stdoutPrintf,
	})
}

func executeBundleBuild(env *cliEnv, root string, out string) error {
	return bundlecli.Build(bundlecli.BuildOptions{
		Root:         root,
		Out:          out,
		Verbosef:     env.verbosef,
		StdoutPrintf: env.stdoutPrintf,
	})
}
