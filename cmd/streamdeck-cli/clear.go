package main

import (
	"github.com/muesli/coral"
)

var (
	clearCmd = &coral.Command{
		Use:   "clear",
		Short: "clears all images",
		RunE: func(_ *coral.Command, _ []string) error {
			return d.Clear()
		},
	}
)

func init() {
	RootCmd.AddCommand(clearCmd)
}
