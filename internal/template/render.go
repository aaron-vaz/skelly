package template

import (
	"os"
	"strings"
	goTemplate "text/template"
)

func (config *ProjectTemplate) ProcessTemplatedFileName(tFileName string) (string, error) {
	tmpl, err := goTemplate.New("templated-file-path").Parse(tFileName)
	if err != nil {
		return "", err
	}

	rFilePath := new(strings.Builder)
	err = tmpl.Execute(rFilePath, config)
	if err != nil {
		return "", err
	}

	if rFilePath.String() == tFileName {
		return tFileName, nil
	}

	err = os.Rename(tFileName, rFilePath.String())
	if err != nil {
		return "", err
	}

	return rFilePath.String(), nil
}

func (config ProjectTemplate) ProcessTemplatedFile(tFilePath string) error {
	tmpl, err := goTemplate.ParseFiles(tFilePath)
	if err != nil {
		return err
	}

	tFile, err := os.Create(tFilePath)
	if err != nil {
		return err
	}

	defer tFile.Close()
	return tmpl.Execute(tFile, config)
}
