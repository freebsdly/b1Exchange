package conf

import (
	"b1Exchange/pkg/model"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

//
func Parse(file string) (*model.Configuration, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var cfg = new(model.Configuration)
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}

	err = cfg.Check()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
