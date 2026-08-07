package configapi

import (
	"reflect"
	"testing"
)

func TestStorePublishesImmutableNamespaceSnapshots(t *testing.T) {
	store := NewStore()
	values := map[string]any{
		"channels": map[string]any{
			"refreshInterval": 5,
		},
	}
	if err := store.Publish(values); err != nil {
		t.Fatalf("publish: %v", err)
	}

	values["channels"].(map[string]any)["refreshInterval"] = 99
	var decoded struct {
		RefreshInterval int `json:"refreshInterval"`
	}
	snapshot := store.Snapshot("channels")
	if err := snapshot.Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.RefreshInterval != 5 {
		t.Fatalf("refresh interval = %d, want 5", decoded.RefreshInterval)
	}

	mutable_copy := snapshot.Values()
	mutable_copy["refreshInterval"] = 42
	if got := store.Snapshot("channels").Values()["refreshInterval"]; got != float64(5) {
		t.Fatalf("stored snapshot was mutated: %v", got)
	}
}

func TestStoreSubscribeAndUnsubscribe(t *testing.T) {
	store := NewStore()
	var revisions []uint64
	unsubscribe := store.Subscribe("channels", func(snapshot Snapshot) {
		revisions = append(revisions, snapshot.Revision())
	})

	if err := store.Publish(map[string]any{"channels": map[string]any{"enabled": true}}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if err := store.Publish(map[string]any{"channels": map[string]any{"enabled": false}}); err != nil {
		t.Fatalf("second publish: %v", err)
	}
	unsubscribe()
	unsubscribe()
	if err := store.Publish(map[string]any{"channels": map[string]any{"enabled": true}}); err != nil {
		t.Fatalf("third publish: %v", err)
	}

	want := []uint64{1, 2}
	if !reflect.DeepEqual(revisions, want) {
		t.Fatalf("revisions = %v, want %v", revisions, want)
	}
}

func TestSnapshotDecodeRejectsNilTarget(t *testing.T) {
	if err := (Snapshot{}).Decode(nil); err != ErrNilTarget {
		t.Fatalf("Decode(nil) error = %v, want %v", err, ErrNilTarget)
	}
}
