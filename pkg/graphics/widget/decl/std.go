package decl

import (
	"avyos.dev/pkg/graphics/parse"
	"embed"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed widgets/*.qml
var widgetFS embed.FS

var stdSources = map[string]string{}

func loadStdLibrary(registry *componentRegistry) error {
	for path, src := range stdSources {
		doc, err := parse.Parse(src)
		if err != nil {
			return &parse.BuildError{Message: "failed to parse standard library " + path + ": " + err.Error()}
		}
		for _, comp := range doc.Components {
			registry.register(comp)
		}
	}

	entries, err := fs.ReadDir(widgetFS, "widgets")
	if err != nil {
		return &parse.BuildError{Message: "failed to read embedded widgets: " + err.Error()}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".qml" {
			continue
		}
		path := "widgets/" + entry.Name()
		data, err := widgetFS.ReadFile(path)
		if err != nil {
			return &parse.BuildError{Message: "failed to read embedded widget " + path + ": " + err.Error()}
		}
		src := string(data)
		stdSources[path] = src
		stdSources["widgets/"+strings.TrimSuffix(entry.Name(), ".qml")] = src

		doc, err := parse.Parse(src)
		if err != nil {
			return &parse.BuildError{Message: "failed to parse embedded widget " + path + ": " + err.Error()}
		}
		for _, comp := range doc.Components {
			registry.register(comp)
		}
	}

	return nil
}
