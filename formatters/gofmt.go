package formatters

import "go/format"

type gofmtFormatter struct{}

func (gofmtFormatter) Format(_ string, src []byte) ([]byte, error) {
	return format.Source(src)
}
