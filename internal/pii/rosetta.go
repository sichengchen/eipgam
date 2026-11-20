package pii

import (
	"github.com/sichengchen/eipgam/internal/rosetta"
	"github.com/sichengchen/eipgam/internal/winarm"
)

// translated says eipgam runs under Rosetta: an Intel build on Apple
// silicon. Tests replace it.
var translated = rosetta.Translated

// emulated says eipgam runs under Windows on Arm's x64 emulation: an x64
// build on an Arm64 computer. Tests replace it.
var emulated = winarm.Emulated
