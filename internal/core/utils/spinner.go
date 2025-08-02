package utils

import (
	"fmt"
	"time"

	"github.com/theckman/yacspin"
)

type Spinner struct {
	spinner *yacspin.Spinner
}

func NewSpinner(message string) (*Spinner, error) {
	cfg := yacspin.Config{
		Frequency:         100 * time.Millisecond,
		CharSet:           yacspin.CharSets[59], // Nice spinning dots
		Suffix:            " " + message,
		SuffixAutoColon:   true,
		ColorAll:          true,
		Colors:            []string{"fgYellow"},
		StopCharacter:     "✓",
		StopColors:        []string{"fgGreen"},
		StopFailCharacter: "✗",
		StopFailColors:    []string{"fgRed"},
	}

	spinner, err := yacspin.New(cfg)
	if err != nil {
		return nil, err
	}

	return &Spinner{spinner: spinner}, nil
}

func (s *Spinner) Start() error {
	return s.spinner.Start()
}

func (s *Spinner) Stop() error {
	return s.spinner.Stop()
}

func (s *Spinner) StopWithSuccess(message string) error {
	s.spinner.StopMessage(message)
	err := s.spinner.Stop()
	if err == nil {
		// Print success message with checkmark
		fmt.Printf("✓ %s\n", message)
	}
	return err
}

func (s *Spinner) StopWithFailure(message string) error {
	s.spinner.StopFailMessage(message)
	err := s.spinner.Stop()
	if err == nil {
		// Print failure message with X mark
		fmt.Printf("✗ %s\n", message)
	}
	return err
}

func (s *Spinner) UpdateMessage(message string) {
	s.spinner.Message(message)
}

func (s *Spinner) Pause() error {
	return s.spinner.Pause()
}

func (s *Spinner) Unpause() error {
	return s.spinner.Unpause()
}