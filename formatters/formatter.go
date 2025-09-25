package formatters

// order matters here
var defaultFormatters = []formatter{
	&gciFormatter{},
	&golinesFormatter{},
	&aifiFormatter{},
	&gofmtFormatter{},
}

type Formatter struct {
	formatters []formatter
}

func (f *Formatter) Format(filename string, src []byte) (res []byte, err error) {
	res = append(make([]byte, 0, len(src)), src...)
	for _, formatter := range f.formatters {
		if res, err = formatter.Format(filename, res); err != nil {
			return nil, err
		}
	}
	return
}

type formatter interface {
	Format(filename string, src []byte) ([]byte, error)
}

func NewFormatter(formatters ...formatter) *Formatter {
	if len(formatters) == 0 {
		formatters = defaultFormatters
	}
	return &Formatter{formatters}
}
