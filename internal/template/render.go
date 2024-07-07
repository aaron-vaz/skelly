package template

import (
	"os"
	gt "text/template"
)

type TemplatedFile string

func (t TemplatedFile) Render(config ProjectTemplate) error {
	tFilePath := string(t)
	tmpl, err := gt.ParseFiles(tFilePath)
	if err != nil {
		return err
	}

	tFile, err := os.Create(tFilePath)
	if err != nil {
		return err
	}

	return tmpl.Execute(tFile, config)

}
