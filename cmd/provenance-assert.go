package cmd

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/khulnasoft/blazedock/pkg/blazedock"
	"github.com/khulnasoft/blazedock/pkg/provutil"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/bom/pkg/provenance"
)

// provenanceExportCmd represents the provenance assert command
var provenanceAssertCmd = &cobra.Command{
	Use:   "assert <package|file://pathToAFile>",
	Short: "Makes assertions about the provenance of a package",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bundleFN, pkgFN, pkg, err := getProvenanceTarget(cmd, args)
		if err != nil {
			log.WithError(err).Fatal("cannot locate bundle")
		}

		var assertions provutil.Assertions
		if signed, err := cmd.Flags().GetBool("signed"); err != nil {
			log.Fatal(err)
		} else if signed {
			log.Warn("checking signatures is most likely broken and will probably return false results")

			var keyPath string
			if pkg == nil {
				keyPath = os.Getenv("BLAZEDOCK_PROVENANCE_KEYPATH")
			} else {
				keyPath = pkg.C.W.Provenance.KeyPath
			}
			if keyPath == "" {
				log.Fatal("no key path specified - use the BLAZEDOCK_PROVENANCE_KEYPATH to specify one")
			}

			var key in_toto.Key
			err := key.LoadKeyDefaults(keyPath)
			if err != nil {
				log.WithError(err).Fatal("cannot load key from " + pkg.C.W.Provenance.KeyPath)
			}
			assertions = append(assertions, provutil.AssertSignedWith(key))
		}
		if do, err := cmd.Flags().GetBool("built-with-blazedock"); err != nil {
			log.Fatal(err)
		} else if do {
			assertions = append(assertions, provutil.AssertBuiltWithBlazedock)
		}
		if ver, err := cmd.Flags().GetString("built-with-blazedock-version"); err != nil {
			log.Fatal(err)
		} else if ver != "" {
			assertions = append(assertions, provutil.AssertBuiltWithBlazedockVersion(ver))
		}
		if do, err := cmd.Flags().GetBool("git-only"); err != nil {
			log.Fatal(err)
		} else if do {
			assertions = append(assertions, provutil.AssertGitMaterialOnly)
		}

		var failures []provutil.Violation
		stmt := provenance.NewSLSAStatement()
		assert := func(env *provenance.Envelope) error {
			if env.PayloadType != in_toto.PayloadType {
				log.Warnf("only supporting %s payloads, not %s - skipping", in_toto.PayloadType, env.PayloadType)
				return nil
			}

			failures = append(assertions.AssertBundle(env), failures...)

			raw, err := base64.StdEncoding.DecodeString(env.Payload)
			if err != nil {
				return err
			}
			err = json.Unmarshal(raw, &stmt)
			if err != nil {
				return err
			}

			failures = append(assertions.AssertStatement(stmt), failures...)

			return nil
		}

		if pkg == nil {
			var f *os.File
			f, err = os.Open(bundleFN)
			if err != nil {
				log.WithError(err).Fatalf("cannot open attestation bundle %s", bundleFN)
			}
			defer f.Close()

			err = provutil.DecodeBundle(f, assert)
		} else {
			err = blazedock.AccessAttestationBundleInCachedArchive(pkgFN, func(bundle io.Reader) error {
				return provutil.DecodeBundle(bundle, assert)
			})
		}
		if err != nil {
			log.WithError(err).Fatal("cannot assert attestation bundle")
		}

		if len(failures) != 0 {
			for _, f := range failures {
				log.Error(f.String())
			}
			log.Fatal("failed")
		}
	},
}

func getProvenanceTarget(cmd *cobra.Command, args []string) (bundleFN, pkgFN string, pkg *blazedock.Package, err error) {
	if strings.HasPrefix(args[0], "file://") {
		bundleFN = strings.TrimPrefix(args[0], "file://")
	} else {
		_, pkg, _, _ = getTarget(args, false)
		if pkg == nil {
			log.Fatal("provenance export requires a package")
		}

		_, cache := getBuildOpts(cmd)

		var ok bool
		pkgFN, ok = cache.Location(pkg)
		if !ok {
			log.Fatalf("%s is not built", pkg.FullName())
		}
	}
	return
}

func init() {
	provenanceAssertCmd.Flags().Bool("signed", false, "ensure that all entries in the attestation bundle are signed and valid under the given key")
	provenanceAssertCmd.Flags().Bool("built-with-blazedock", false, "ensure that all entries in the attestation bundle are built by blazedock")
	provenanceAssertCmd.Flags().String("built-with-blazedock-version", "", "ensure that all entries in the attestation bundle are built by a specific blazedock version")
	provenanceAssertCmd.Flags().Bool("git-only", false, "ensure that all entries in the attestation bundle are built directly from Git (i.e. only have git material entries)")

	addBuildFlags(provenanceAssertCmd)
	provenanceCmd.AddCommand(provenanceAssertCmd)
}
