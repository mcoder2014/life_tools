package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

func DoubleCheck(notice string) error {
	var fd int
	fd = syscall.Stdin
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		panic(err)
	}
	defer term.Restore(fd, oldState)

	t := term.NewTerminal(os.Stdin, "> ")

	var sb strings.Builder
	sb.WriteString(notice)
	sb.WriteString(" (y/n): \n")

	_, err = t.Write([]byte(sb.String()))
	if err != nil {
		return err
	}

	userInput, err := t.ReadLine()
	if err != nil {
		return err
	}
	if userInput != "y" {
		return fmt.Errorf("user input is not y")
	}
	return nil
}
