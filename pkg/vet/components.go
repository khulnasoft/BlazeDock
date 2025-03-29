package vet

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/khulnasoft/blazedock/pkg/blazedock"
)

func init() {
	register(ComponentCheck("fmt", "ensures the BUILD.yaml of a component is blazedock fmt'ed", checkComponentsFmt))
}

func checkComponentsFmt(comp *blazedock.Component) ([]Finding, error) {
	fc, err := os.ReadFile(filepath.Join(comp.Origin, "BUILD.yaml"))
	if err != nil {
		return nil, err
	}
	if len(fc) == 0 {
		// empty BUILD.yaml files are ok
		return nil, nil
	}

	buf := bytes.NewBuffer(nil)
	err = blazedock.FormatBUILDyaml(buf, bytes.NewReader(fc), false)
	if err != nil {
		return nil, err
	}

	if bytes.EqualFold(buf.Bytes(), fc) {
		return nil, nil
	}

	return []Finding{
		{
			Component:   comp,
			Description: "component's BUILD.yaml is not formated using `blazedock fmt`",
			Error:       false,
		},
	}, nil
}
