package main

import (
	_ "embed"
	"os"
)

//go:embed wizard_installer.tmpl
var wizardTemplate string

// pickInstallerTemplate memilih template Setup.exe. Wizard aktif kecuali NETRA_WIZARD=0.
func pickInstallerTemplate() string {
	if os.Getenv("NETRA_WIZARD") != "0" {
		return wizardTemplate
	}
	return installerTemplate
}
