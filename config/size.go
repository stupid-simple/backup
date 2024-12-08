package config

import "github.com/docker/go-units"

type SizeArgument struct {
	Size int64 `arg:"" help:"size in bytes"`
}

func (s *SizeArgument) UnmarshalText(text []byte) (err error) {
	s.Size, err = units.FromHumanSize(string(text))
	return
}
