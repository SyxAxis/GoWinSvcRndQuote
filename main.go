/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"os"
	"svc_rnd_qt/pkg"
)

func main() {

	if len(os.Args) < 2 {
		pkg.Usage("no service command")
	}

	// This is ultra important that it interacts with the O/S at the most basic level
	// do not attempt to use anything like Cobra as that will do things with terminals that Windows
	// service subsystem doesn't like. Windows runs "hidden screens" to handle various events
	// services run on those "hidden screens" and must be basic STDIN/STDOUT
	// https://learn.microsoft.com/en-gb/windows/win32/winstation/about-window-stations-and-desktops?redirectedfrom=MSDN

	pkg.ServiceControl(os.Args[1])

	// pkg.InitCustomCode()
}
