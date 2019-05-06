package formats

import "fmt"

type FormatOptions struct {
	Strict bool `mapstructure:"strict"`
}

type FormatHandler interface {
	Process(msg []byte) (Datapoint, error)
	Output(dp Datapoint) []byte
	KindS() string
	Kind() FormatName
}

type FormatName string

func (f FormatName) ToHandler(fo FormatOptions) (FormatHandler, error) {
	switch f {
	case PlainFormat:
		return NewPlain(fo.Strict), nil
	default:
		return nil, fmt.Errorf("please use a valid \"format\" for `%s`", f)
	}
}
