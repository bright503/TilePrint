//go:build !windows

package main

import "log"

func runtimeCommand()        {}
func desktopReady() bool     { return true }
func desktopError(err error) { log.Print(err) }
