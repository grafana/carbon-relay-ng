package encoding

import "fmt"

type FormatOptions struct {
	Strict bool `mapstructure:"strict,omitempty"`
	Unsafe bool `mapstructure:"unsafe,omitempty"`
}

type FormatAdapter interface {
	Load(msg []byte) (Datapoint, error)
	Dump(dp Datapoint) []byte
	KindS() string
	Kind() FormatName
}

type FormatName string

func (f FormatName) ToHandler(fo FormatOptions) (FormatAdapter, error) {
	switch f {
	case PlainFormat:
		if fo.Unsafe {
			return nil, fmt.Errorf("`unsafe` is disabled for now.")
		}
		return NewPlain(fo.Strict, fo.Unsafe), nil
	case "":
		return nil, fmt.Errorf("`format` key can't be empty. Possible value: [plain]")
	default:
		return nil, fmt.Errorf("please use a valid \"format\" for `%s`", f)
	}
}
