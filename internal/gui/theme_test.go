package gui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// themeFor must ignore the variant Fyne passes in, like the old DarkTheme/LightTheme.
func TestThemeForPinsVariant(t *testing.T) {
	test.NewTempApp(t)
	bg := func(v fyne.ThemeVariant) any { return theme.DefaultTheme().Color(theme.ColorNameBackground, v) }
	for mode, want := range map[string]fyne.ThemeVariant{
		"dark": theme.VariantDark, "light": theme.VariantLight, "": theme.VariantLight,
	} {
		for _, passed := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight} {
			if got := themeFor(mode).Color(theme.ColorNameBackground, passed); got != bg(want) {
				t.Errorf("themeFor(%q) with variant %d = %v, want %v", mode, passed, got, bg(want))
			}
		}
	}
}
