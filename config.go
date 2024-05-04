package main

type Settings struct {
	Override bool `toml:"override"`
}

type Source struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type Destination struct {
	Name          string            `toml:"name"`
	Path          string            `toml:"path"`
	IgnoredFields []string          `toml:"ignore"`
	FieldsMap     map[string]string `toml:"map"`
}

type Mapping struct {
	Settings    Settings    `toml:"settings"`
	Source      Source      `toml:"source"`
	Destination Destination `toml:"destination"`
}

type Config struct {
	Mappings []Mapping  `toml:"mappings"`
	Settings GenSetting `toml:"settings"`
}

type GenSetting struct {
	Style          string `toml:"style"`
	Module         string `toml:"module"`
	PathFromModule string `toml:"path_from_module"`
}
