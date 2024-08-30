package templates

import (
	"os"
	"strings"
	"text/template"
)

type StdRenderer struct{}

func (r *StdRenderer) Supports(config ProjectTemplate) bool {
	return true
}

func (r *StdRenderer) RenderFile(config ProjectTemplate, path string) (string, error) {
	tmpl, err := template.New("templated").Parse(path)
	if err != nil {
		return "", err
	}

	rPath := new(strings.Builder)
	err = tmpl.Execute(rPath, config)
	if err != nil {
		return "", err
	}

	if rPath.String() == path {
		return path, nil
	}

	err = os.Rename(path, rPath.String())
	if err != nil {
		return "", err
	}

	return rPath.String(), nil
}

func (r *StdRenderer) RenderFileContents(config ProjectTemplate, path string) error {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return err
	}

	tFile, err := os.Create(path)
	if err != nil {
		return err
	}

	defer tFile.Close()
	return tmpl.Execute(tFile, config)
}
