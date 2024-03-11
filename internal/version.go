package internal

import (
	"github.com/icinga/icinga-kubernetes/pkg/version"
)

// Version contains version and Git commit information.
//
// The placeholders are replaced on `git archive` using the `export-subst` attribute.
var Version = version.Version("Icinga Kubernetes", "0.1.0", "$Format:%(describe)$", "$Format:%H$")
