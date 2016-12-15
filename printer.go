package main

import (
	"fmt"
	"os"
	"runtime"
)

type Color string

const (
	colorReset  Color = "\u001B[0m"
	ColorBlack  Color = "\u001B[30m"
	ColorRed    Color = "\u001B[31m"
	ColorGreen  Color = "\u001B[32m"
	ColorYellow Color = "\u001B[33m"
	ColorBlue   Color = "\u001B[34m"
	ColorPurple Color = "\u001B[35m"
	ColorCyan   Color = "\u001B[36m"
	ColorWhite  Color = "\u001B[37m"
)

func Println(color Color, v ...interface{}) {
	if runtime.GOOS == "windows" {
		fmt.Println(v...)
		return
	}
	fmt.Print(color)
	fmt.Print(v...)
	fmt.Println(colorReset)
}

func Printf(color Color, format string, v ...interface{}) {
	if runtime.GOOS == "windows" {
		fmt.Println(v...)
		return
	}
	fmt.Print(color)
	fmt.Printf(format, v...)
	fmt.Print(colorReset)
}

func ErrPrintln(color Color, v ...interface{}) {
	if runtime.GOOS == "windows" {
		fmt.Println(v...)
		return
	}
	fmt.Fprint(os.Stderr, color)
	fmt.Fprint(os.Stderr, v...)
	fmt.Fprintln(os.Stderr, colorReset)
}

func ErrPrintf(color Color, format string, v ...interface{}) {
	if runtime.GOOS == "windows" {
		fmt.Println(v...)
		return
	}
	fmt.Fprint(os.Stderr, color)
	fmt.Fprintf(os.Stderr, format, v...)
	fmt.Fprint(os.Stderr, colorReset)
}
