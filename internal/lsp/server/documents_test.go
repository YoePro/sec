package server

import "testing"

func TestDocumentsRejectsStaleChanges(t *testing.T) {
	documents := NewDocuments()
	documents.Open("file:///main.sec", 1, "first")
	if _, changed := documents.Change("file:///main.sec", 1, "stale"); changed {
		t.Fatal("same-version change was accepted")
	}
	snapshot, changed := documents.Change("file:///main.sec", 2, "second")
	if !changed || snapshot.Version != 2 || snapshot.Text != "second" {
		t.Fatalf("wrong updated snapshot: %#v, changed=%v", snapshot, changed)
	}
}

func TestDocumentsCloseRemovesSnapshot(t *testing.T) {
	documents := NewDocuments()
	documents.Open("file:///main.sec", 1, "source")
	documents.Close("file:///main.sec")
	if _, ok := documents.Snapshot("file:///main.sec"); ok {
		t.Fatal("closed document still has a snapshot")
	}
}
