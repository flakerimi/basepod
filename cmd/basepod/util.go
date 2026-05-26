package main

import "fmt"

func printfImpl(format string, args ...any) error {
	_, err := fmt.Printf(format+"\n", args...)
	return err
}
