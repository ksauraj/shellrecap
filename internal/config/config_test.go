package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("SHELLRECAP_CONFIG_DIR", t.TempDir())
	c := Load()
	if c.Theme != "auto" || !c.Share.Has(RecapGIF) || !c.Share.Has(TourGIF) || c.Share.Has(Slides) {
		t.Errorf("defaults = %+v", c)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELLRECAP_CONFIG_DIR", dir)
	c := Defaults()
	c.Theme = "black"
	c.Share.Outputs = []string{Poster, Slides}
	c.Share.Shape = Story
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	got := Load()
	if got.Theme != "black" || got.Share.Shape != Story || !got.Share.Has(Slides) || got.Share.Has(RecapGIF) {
		t.Errorf("loaded %+v, want what was saved", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json.tmp")); err == nil {
		t.Error("left the temporary file behind")
	}
}

func TestLoadRepairsBadValues(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELLRECAP_CONFIG_DIR", dir)
	os.WriteFile(filepath.Join(dir, "config.json"),
		[]byte(`{"theme":"neon","share":{"outputs":["video"],"gif_size":"huge","shape":"circle","image_theme":"plaid"}}`), 0o644)
	got, want := Load(), Defaults()
	if got.Theme != want.Theme || got.Share.GIFSize != want.Share.GIFSize || got.Share.Shape != want.Share.Shape ||
		got.Share.ImageTheme != want.Share.ImageTheme || len(got.Share.Outputs) != len(want.Share.Outputs) {
		t.Errorf("bad values loaded as %+v, want the defaults", got)
	}

	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`not json`), 0o644)
	if got := Load(); got.Theme != "auto" {
		t.Errorf("a corrupt file loaded as %+v, want the defaults", got)
	}
}
