package cmd

import (
	"github.com/khulnasoft/blazedock/pkg/blazedock"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// describeGitInfoCmd represents the describeTree command
var describeGitInfoCmd = &cobra.Command{
	Use:   "git-info",
	Short: "Prints the Git info consumed by blazedock",
	Run: func(cmd *cobra.Command, args []string) {
		comp, pkg, script, _ := getTarget(args, false)

		var (
			nfo *blazedock.GitInfo
			loc string
		)
		if comp != nil {
			nfo = comp.Git()
			loc = comp.Origin
		} else if pkg != nil {
			nfo = pkg.C.Git()
			loc = pkg.C.Origin
		} else if script != nil {
			nfo = script.C.Git()
			loc = script.C.Origin
		} else {
			log.Fatal("no target given - try passing a package or component")
		}

		if nfo == nil {
			log.WithField("loc", loc).Fatal("not a Git working copy")
		}
		w := getWriterFromFlags(cmd)
		if w.FormatString == "" {
			w.FormatString = `dirty:	{{.Dirty }}
origin:	{{ .Origin }}
commit:	{{ .Commit }}
`
		}
		err := w.Write(nfo)
		if err != nil {
			log.WithError(err).Fatal("cannot write git info")
		}
	},
}

func init() {
	describeCmd.AddCommand(describeGitInfoCmd)
	addFormatFlags(describeGitInfoCmd)
}
