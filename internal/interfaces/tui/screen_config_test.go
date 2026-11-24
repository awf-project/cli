package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadedConfig(info ConfigInfo) ConfigScreen {
	s := newConfigScreen()
	s.SetSize(100, 20)
	s, _ = s.Update(ConfigLoadedMsg{Info: info})
	return s
}

func TestConfigScreen_ShowsTheFileItsStatusAndInputs(t *testing.T) {
	s := loadedConfig(ConfigInfo{
		Path: ".awf/config.yaml", Exists: true,
		Inputs: map[string]any{"env": "prod", "region": "eu", "API_KEY": "s3cr3t", "PASSWORD": "hunter2"},
	})

	out := ansi.Strip(s.view(100, 20))

	for _, want := range []string{"Configuration", ".awf/config.yaml", "found", "Inputs", "env = prod", "region = eu"} {
		assert.Contains(t, out, want)
	}
	assert.Contains(t, out, "API_KEY = ***")
	assert.Contains(t, out, "PASSWORD = ***")
	assert.NotContains(t, out, "s3cr3t", "secret values are never drawn")
	assert.NotContains(t, out, "hunter2")
	assert.Contains(t, out, "take precedence", "the hint explains how the values are used")
}

func TestConfigScreen_InputsAreSorted(t *testing.T) {
	s := loadedConfig(ConfigInfo{Exists: true, Inputs: map[string]any{"zeta": 1, "alpha": 2, "mid": 3}})

	out := ansi.Strip(s.view(100, 20))

	assert.Less(t, indexOf(out, "alpha"), indexOf(out, "mid"))
	assert.Less(t, indexOf(out, "mid"), indexOf(out, "zeta"))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestConfigScreen_States(t *testing.T) {
	t.Run("not loaded yet", func(t *testing.T) {
		s := newConfigScreen()
		s.SetSize(100, 20)
		assert.Contains(t, ansi.Strip(s.view(100, 20)), labelConfigLoading)
	})
	t.Run("file missing is a valid empty setup", func(t *testing.T) {
		s := loadedConfig(ConfigInfo{Path: ".awf/config.yaml", Exists: false})
		out := ansi.Strip(s.view(100, 20))
		assert.Contains(t, out, "not found")
		assert.Contains(t, out, labelConfigScreenNoInputs)
		assert.NotContains(t, out, "✗")
	})
	t.Run("file without inputs", func(t *testing.T) {
		s := loadedConfig(ConfigInfo{Path: "c.yaml", Exists: true})
		assert.Contains(t, ansi.Strip(s.view(100, 20)), labelConfigScreenNoInputs)
	})
	t.Run("read error is shown in full", func(t *testing.T) {
		s := newConfigScreen()
		s.SetSize(60, 20)
		s, _ = s.Update(ConfigLoadedMsg{Err: "config error: yaml: line 3: did not find expected key while parsing a block mapping"})
		assert.Contains(t, flatText(s.view(60, 20)), "did not find expected key while parsing a block mapping")
	})
	t.Run("a good reload clears an earlier error", func(t *testing.T) {
		s := newConfigScreen()
		s.SetSize(100, 20)
		s, _ = s.Update(ConfigLoadedMsg{Err: "boom"})
		s, _ = s.Update(ConfigLoadedMsg{Info: ConfigInfo{Path: "c.yaml", Exists: true}})
		out := ansi.Strip(s.view(100, 20))
		assert.NotContains(t, out, "boom")
		assert.Contains(t, out, "c.yaml")
	})
}

func TestConfigScreen_Scrolling(t *testing.T) {
	inputs := map[string]any{}
	for i := 0; i < 40; i++ {
		inputs[fmt.Sprintf("input_%02d", i)] = i
	}
	s := loadedConfig(ConfigInfo{Path: "c.yaml", Exists: true, Inputs: inputs})
	total := len(s.lines())
	rows := s.rows()
	require.Greater(t, total, rows)

	assert.Contains(t, ansi.Strip(s.view(100, 20)), "(1–", "an overflowing config shows its scroll position")

	s, _ = s.Update(press("down"))
	assert.Equal(t, 1, s.offset)
	s, _ = s.Update(press("pgdown"))
	assert.Equal(t, 1+rows-1, s.offset)
	s, _ = s.Update(press("end"))
	assert.Equal(t, total-rows, s.offset, "end shows the last line, never past it")
	s, _ = s.Update(press("down"))
	assert.Equal(t, total-rows, s.offset)
	s, _ = s.Update(press("home"))
	assert.Zero(t, s.offset)
	s, _ = s.Update(press("up"))
	assert.Zero(t, s.offset)

	s, _ = s.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	assert.Equal(t, wheelStep, s.offset)
	s, _ = s.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	assert.Zero(t, s.offset)
}

func TestConfigScreen_ResizeReclampsTheScroll(t *testing.T) {
	inputs := map[string]any{}
	for i := 0; i < 40; i++ {
		inputs[fmt.Sprintf("k%02d", i)] = i
	}
	s := loadedConfig(ConfigInfo{Exists: true, Inputs: inputs})
	s, _ = s.Update(press("end"))
	require.Positive(t, s.offset)

	s.SetSize(100, 80)

	assert.LessOrEqual(t, s.offset, max(0, len(s.lines())-s.rows()))
}

func TestConfigScreen_RefreshAndKeyBar(t *testing.T) {
	s := loadedConfig(ConfigInfo{Exists: true})

	_, cmd := s.Update(press("r"))

	require.NotNil(t, cmd)
	assert.Equal(t, refreshMsg{}, cmd())
	bar := ansi.Strip(s.keys(100))
	for _, want := range []string{"tabs", "scroll", "refresh", "quit"} {
		assert.Contains(t, bar, want)
	}
}

func TestConfigScreen_ViewFillsExactWidths(t *testing.T) {
	s := loadedConfig(ConfigInfo{Path: "c.yaml", Exists: true, Inputs: map[string]any{"long": "x" + string(make([]rune, 0))}})
	for _, w := range []int{80, 100, 160} {
		for _, l := range lines(s.view(w, 20)) {
			assert.Equal(t, w, ansiWidth(l), "width %d", w)
		}
	}
}

type fakeConfig struct {
	info ConfigInfo
	err  error
}

func (f fakeConfig) ReadProjectConfig(context.Context) (ConfigInfo, error) { return f.info, f.err }

func TestBridge_LoadConfig(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	info := ConfigInfo{Path: "c.yaml", Exists: true, Inputs: map[string]any{"env": "prod"}}
	tests := []struct {
		name    string
		reader  ConfigReader
		ctx     context.Context
		wantErr string
		want    ConfigInfo
	}{
		{"no reader", nil, context.Background(), labelConfigScreenNoReader, ConfigInfo{}},
		{"cancelled", fakeConfig{info: info}, cancelled, context.Canceled.Error(), ConfigInfo{}},
		{"read failure keeps what was found", fakeConfig{info: ConfigInfo{Path: "c.yaml", Exists: true}, err: errors.New("bad yaml")}, context.Background(), "bad yaml", ConfigInfo{Path: "c.yaml", Exists: true}},
		{"success", fakeConfig{info: info}, context.Background(), "", info},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBridge(nil, nil)
			if tt.reader != nil {
				b.SetConfigReader(tt.reader)
			}

			msg, ok := b.LoadConfig(tt.ctx)().(ConfigLoadedMsg)

			require.True(t, ok)
			assert.Equal(t, tt.want, msg.Info)
			if tt.wantErr == "" {
				assert.Empty(t, msg.Err)
			} else {
				assert.Contains(t, msg.Err, tt.wantErr)
			}
		})
	}
}

func TestProjectConfigReader(t *testing.T) {
	dir := t.TempDir()

	t.Run("reads the inputs of an existing file", func(t *testing.T) {
		path := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("inputs:\n  env: prod\n  count: 3\n"), 0o600))
		t.Setenv("AWF_CONFIG_PATH", path)

		info, err := projectConfigReader{}.ReadProjectConfig(context.Background())

		require.NoError(t, err)
		assert.Equal(t, path, info.Path)
		assert.True(t, info.Exists)
		assert.Equal(t, "prod", info.Inputs["env"])
		assert.EqualValues(t, 3, info.Inputs["count"])
	})
	t.Run("a missing file is an empty setup, not an error", func(t *testing.T) {
		path := filepath.Join(dir, "absent.yaml")
		t.Setenv("AWF_CONFIG_PATH", path)

		info, err := projectConfigReader{}.ReadProjectConfig(context.Background())

		require.NoError(t, err)
		assert.False(t, info.Exists)
		assert.Equal(t, path, info.Path)
		assert.Empty(t, info.Inputs)
	})
	t.Run("invalid yaml is an error that still reports the file", func(t *testing.T) {
		path := filepath.Join(dir, "broken.yaml")
		require.NoError(t, os.WriteFile(path, []byte("inputs: [oops\n"), 0o600))
		t.Setenv("AWF_CONFIG_PATH", path)

		info, err := projectConfigReader{}.ReadProjectConfig(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "config error")
		assert.True(t, info.Exists)
		assert.Equal(t, path, info.Path)
	})
}

func TestModel_ConfigTab(t *testing.T) {
	b := NewBridge(nil, nil)
	b.SetConfigReader(fakeConfig{info: ConfigInfo{Path: ".awf/config.yaml", Exists: true, Inputs: map[string]any{"env": "prod"}}})
	m := loadedModel(t, 110, 30)
	m.bridge = b

	t.Run("reload loads the configuration", func(t *testing.T) {
		cmd := m.reload()
		require.NotNil(t, cmd)
		got := m
		for _, msg := range collect(cmd) {
			if cfg, ok := msg.(ConfigLoadedMsg); ok {
				got, _ = send(got, cfg)
			}
		}
		got, _ = send(got, press("3"))

		out := plain(got)
		assert.Contains(t, out, "3 Config")
		assert.Contains(t, out, "env = prod")
		assert.Contains(t, out, ".awf/config.yaml")
	})
	t.Run("q still quits and the tab keys keep working", func(t *testing.T) {
		got, _ := send(m, press("3"))
		got, _ = send(got, press("1"))
		assert.Equal(t, TabWorkflows, got.activeTab)
		got, _ = send(got, press("3"))
		_, cmd := send(got, press("q"))
		assert.True(t, isQuit(cmd))
	})
	t.Run("r refreshes everything", func(t *testing.T) {
		got, _ := send(m, press("3"))
		_, cmd := send(got, press("r"))
		require.NotNil(t, cmd)
		assert.Equal(t, refreshMsg{}, cmd())
	})
}

// collect runs a command, expanding batches, and returns every message produced.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collect(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}
