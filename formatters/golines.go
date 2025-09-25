package formatters

import "github.com/golangci/golines"

var golinesShortener = golines.NewShortener(golines.ShortenerConfig{
	ChainSplitDots:  true,
	IgnoreGenerated: true,
	MaxLen:          120,
	ShortenComments: false,
	TabLen:          4,
})

type golinesFormatter struct{}

func (golinesFormatter) Format(_ string, src []byte) ([]byte, error) {
	return golinesShortener.Shorten(src)
}
