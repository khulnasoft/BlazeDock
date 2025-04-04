package vet

import (
	"fmt"
	"strings"

	"github.com/khulnasoft/blazedock/pkg/blazedock"
)

func init() {
	register(PackageCheck("has-gomod", "ensures all Go packages have a go.mod file in their source list", blazedock.GoPackage, checkGolangHasGomod))
	register(PackageCheck("has-buildflags", "checks for use of deprecated buildFlags config", blazedock.GoPackage, checkGolangHasBuildFlags))
}

func checkGolangHasGomod(pkg *blazedock.Package) ([]Finding, error) {
	var (
		foundGoMod bool
		foundGoSum bool
	)
	for _, src := range pkg.Sources {
		if strings.HasSuffix(src, "/go.mod") {
			foundGoMod = true
		}
		if strings.HasSuffix(src, "/go.sum") {
			foundGoSum = true
		}
		if foundGoSum && foundGoMod {
			return nil, nil
		}
	}

	var f []Finding
	if !foundGoMod {
		f = append(f, Finding{
			Component:   pkg.C,
			Description: "package sources contain no go.mod file",
			Error:       true,
			Package:     pkg,
		})
	}
	if !foundGoSum {
		f = append(f, Finding{
			Component:   pkg.C,
			Description: "package sources contain no go.sum file",
			Error:       true,
			Package:     pkg,
		})
	}
	return f, nil
}

func checkGolangHasBuildFlags(pkg *blazedock.Package) ([]Finding, error) {
	goCfg, ok := pkg.Config.(blazedock.GoPkgConfig)
	if !ok {
		return nil, fmt.Errorf("Go package does not have go package config")
	}

	if len(goCfg.BuildFlags) > 0 {
		return []Finding{{
			Component:   pkg.C,
			Description: "buildFlags are deprecated, use buildCommand instead",
			Error:       false,
			Package:     pkg,
		}}, nil
	}

	return nil, nil
}
