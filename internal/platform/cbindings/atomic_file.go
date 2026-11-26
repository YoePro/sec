package cbindings

import (
	"os"
	"sec/internal/configedit"
)

var ErrStaleFile = configedit.ErrStaleFile

// SHA256 identifies the exact reviewed bytes. Rules: MD-021 correction §5(1)-(3).
func SHA256(data []byte) string { return configedit.SHA256(data) }

// ReplaceFileAtomically shares the configuration transaction primitive.
// Rules: MD-021 correction §5(2)-(3); harness design notes §5(3)-(4).
func ReplaceFileAtomically(file string, data []byte, expected *string) error {
	return configedit.ReplaceFileAtomically(file, data, expected)
}

// replaceFileUsing confines every transaction operation to the selected root.
// Rules: harness design notes §5(4),(6); MD-021 correction §5(2)-(3).
func replaceFileUsing(root *os.Root, file string, data []byte, expected *string) error {
	return configedit.ReplaceFileInRoot(root, file, data, expected)
}
