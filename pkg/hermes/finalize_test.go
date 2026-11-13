package hermes

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type finalization_failure_store struct {
	Store
	finished   bool
	output_err error
}

func (store *finalization_failure_store) FinishTask(int) error { store.finished = true; return nil }
func (store *finalization_failure_store) UpdateResourceOutput(ResourceOutputUpdate) error {
	return store.output_err
}

func TestTaskCannotCompleteBeforeFinalOutputIsCommitted(t *testing.T) {
	for _, test_case := range []struct {
		name, expected_error string
		create_file          bool
	}{
		{name: "rename failure", expected_error: "failed to rename final output"},
		{name: "persistence failure", expected_error: "failed to persist final resource outputs", create_file: true},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			download_dir := t.TempDir()
			file_path := filepath.Join(download_dir, "成片.mp4")
			if test_case.create_file {
				if err := os.WriteFile(file_path, []byte("video"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			store := &finalization_failure_store{output_err: errors.New("database unavailable")}
			engine := New(HermesNewConfig{Store: store, Config: HermesEngineConfig{BasePath: download_dir}})
			if !test_case.create_file {
				file_path = filepath.Join(download_dir, "missing-download")
			}
			job := &TaskJob{ID: 42, DownloadDir: download_dir, Resources: []ResourceJob{{ID: 7, Name: "成片", UniqueID: "source", Kind: "video/mp4", Type: "FILE", FilePath: file_path}}}
			err := engine.finish_task(job)
			if err == nil || !strings.Contains(err.Error(), test_case.expected_error) || store.finished {
				t.Fatalf("err=%v, finished=%v", err, store.finished)
			}
		})
	}
}
