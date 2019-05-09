package encoding

import "fmt"

type FormatOptions struct {
	Strict bool `mapstructure:"strict"`
}

type FormatAdapter interface {
	Process(msg []byte) (Datapoint, error)
	Output(dp Datapoint) []byte
	KindS() string
	Kind() FormatName
}

type FormatName string

func (f FormatName) ToHandler(fo FormatOptions) (FormatAdapter, error) {
	switch f {
	case PlainFormat:
		return NewPlain(fo.Strict), nil
	default:
		return nil, fmt.Errorf("please use a valid \"format\" for `%s`", f)
	}
}
