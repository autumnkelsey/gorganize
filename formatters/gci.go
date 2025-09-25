package formatters

import (
	"github.com/daixiang0/gci/pkg/config"
	"github.com/daixiang0/gci/pkg/gci"
	"github.com/daixiang0/gci/pkg/section"
)

var gciConfig = config.Config{
	BoolConfig: config.BoolConfig{
		CustomOrder:   true,
		SkipGenerated: true,
	},
	Sections: section.SectionList{
		section.Standard{},
		section.Custom{Prefix: "github.com/aifimmunology"},
		section.Default{}},
}

type gciFormatter struct{}

func (gciFormatter) Format(filename string, src []byte) ([]byte, error) {
	_, formatted, err := gci.LoadFormat(src, filename, gciConfig)
	return formatted, err
}
